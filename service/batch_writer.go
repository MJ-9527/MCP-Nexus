package service

import (
	"context"
	"log"
	"sync"
	"time"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

// B14 异步批量审计写入器。
//
// 设计目标：把每次调用产生的审计记录由「逐条同步 INSERT」改为「入队即返回，
// 后台 goroutine 攒满 N 条或周期 T 触发一次批量 INSERT」。这样主调用链不再
// 被 IO 阻塞，且单次 SQL 提交多条记录显著降低 IO 抖动。
//
// 容错与降级：
//   - Submit 永不阻塞调用链（select default 路径丢弃最旧记录并计数）
//   - flush 失败仅记日志，不重试，避免堆积无限增长
//   - Stop 触发最后一次 flush，保证优雅退出时不丢已入队记录
//
// 该 writer 只负责「攒批 + 批量提交」，参数脱敏/摘要由 AuditLogService.Submit
// 在入队前完成；writer 拿到的都是已脱敏的 *model.AuditLog。

const (
	// DefaultBatchSize 触发批量写入的记录数阈值（B14 约束：100 条触发 flush）。
	DefaultBatchSize = 100
	// DefaultBatchInterval 周期性 flush 间隔，无论攒够多少条（B14 约束：500ms）。
	DefaultBatchInterval = 500 * time.Millisecond
	// DefaultQueueCapacity 入队 channel 容量。大于触发阈值，避免短时尖峰误丢。
	DefaultQueueCapacity = 1024
	// flushTimeout 单次 flush 的整体超时，防止下游阻塞拖垮后台 goroutine。
	flushTimeout = 5 * time.Second
)

// BatchAuditWriter 异步批量审计写入器（B14）。
type BatchAuditWriter struct {
	repo      repository.AuditLogRepository
	queue     chan *model.AuditLog
	batchSize int
	interval  time.Duration

	// 状态统计（仅供观测，不参与业务逻辑）
	mu          sync.Mutex
	started     bool
	stopped     bool
	dropped     int64 // 因队列满而丢弃的记录数
	flushFailed int64 // flush 失败次数
	flushed     int64 // 成功写入记录数
	stopCh      chan struct{}
	doneCh      chan struct{}
}

// NewBatchAuditWriter 构造批量写入器。零值参数走默认配置。
// 调用方必须在首次 Submit 前 Start；进程退出前 Stop 以保证残留记录落地。
func NewBatchAuditWriter(repo repository.AuditLogRepository, batchSize int, interval time.Duration, queueCap int) *BatchAuditWriter {
	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}
	if interval <= 0 {
		interval = DefaultBatchInterval
	}
	if queueCap <= 0 {
		queueCap = DefaultQueueCapacity
	}
	return &BatchAuditWriter{
		repo:      repo,
		queue:     make(chan *model.AuditLog, queueCap),
		batchSize: batchSize,
		interval:  interval,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

// Start 启动后台 flush goroutine。重复调用为 no-op。
func (w *BatchAuditWriter) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started || w.stopped {
		return
	}
	w.started = true
	go w.run()
}

// Stop 触发最后一次 flush 并关闭入队；阻塞直到残留记录落地或 flush 超时。
// 重复调用安全（幂等）。
func (w *BatchAuditWriter) Stop() {
	w.mu.Lock()
	if !w.started || w.stopped {
		w.mu.Unlock()
		return
	}
	w.stopped = true
	w.mu.Unlock()
	close(w.stopCh)
	<-w.doneCh
}

// Submit 非阻塞入队。队列满时丢弃当前记录并增加 dropped 计数。
// 调用方传 nil 记录被静默忽略。
func (w *BatchAuditWriter) Submit(log *model.AuditLog) {
	if log == nil {
		return
	}
	select {
	case w.queue <- log:
	default:
		// 队列满：直接丢弃新记录。B14 完成标准要求「审计延迟不阻塞主调用」，
		// 因此丢弃优先于阻塞；通过 Stats 暴露丢弃数便于运维告警。
		w.mu.Lock()
		w.dropped++
		w.mu.Unlock()
	}
}

// Stats 返回观测快照（drop/flush 失败/成功写入计数）。
type WriterStats struct {
	Dropped     int64 `json:"dropped"`
	FlushFailed int64 `json:"flush_failed"`
	Flushed     int64 `json:"flushed"`
}

func (w *BatchAuditWriter) Stats() WriterStats {
	w.mu.Lock()
	defer w.mu.Unlock()
	return WriterStats{Dropped: w.dropped, FlushFailed: w.flushFailed, Flushed: w.flushed}
}

// run 后台主循环：周期性或攒满阈值触发 flush；收到 stop 信号后做最后一次 flush。
func (w *BatchAuditWriter) run() {
	defer close(w.doneCh)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	buffer := make([]*model.AuditLog, 0, w.batchSize)
	flush := func() {
		if len(buffer) == 0 {
			return
		}
		w.flush(buffer)
		buffer = make([]*model.AuditLog, 0, w.batchSize)
	}
	for {
		select {
		case <-w.stopCh:
			// 排空入队 channel 中的剩余记录，然后做最后一次 flush
			for {
				select {
				case log := <-w.queue:
					if log != nil {
						buffer = append(buffer, log)
					}
				default:
					flush()
					return
				}
			}
		case <-ticker.C:
			flush()
		case log := <-w.queue:
			if log != nil {
				buffer = append(buffer, log)
			}
			// 攒满阈值立即 flush，不等下一个 tick
			if len(buffer) >= w.batchSize {
				flush()
			}
		}
	}
}

// flush 提交一次批量写入。失败仅记日志与计数，不重试。
func (w *BatchAuditWriter) flush(logs []*model.AuditLog) {
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()
	if err := w.repo.BatchCreate(ctx, logs); err != nil {
		w.mu.Lock()
		w.flushFailed++
		w.mu.Unlock()
		log.Printf("[audit-batch] flush %d 条失败 err=%v", len(logs), err)
		return
	}
	w.mu.Lock()
	w.flushed += int64(len(logs))
	w.mu.Unlock()
}
