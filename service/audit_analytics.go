package service

import (
	"context"
	"log"
	"sync"
	"time"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

// AsyncAuditAnalyticsSink 将审计复制到分析库的过程与业务请求解耦。
// 队列满或分析库失败只记录日志，不影响 PostgreSQL 审计和主调用链。
type AsyncAuditAnalyticsSink struct {
	sink       repository.AuditAnalyticsSink
	queue      chan *model.AuditLog
	batchSize  int
	flushEvery time.Duration
	onError    func(error)
	stop       chan struct{}
	done       chan struct{}
	once       sync.Once
}

func NewAsyncAuditAnalyticsSink(sink repository.AuditAnalyticsSink, queueSize, batchSize int, flushEvery time.Duration) *AsyncAuditAnalyticsSink {
	if queueSize <= 0 {
		queueSize = 1000
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	if flushEvery <= 0 {
		flushEvery = time.Second
	}
	a := &AsyncAuditAnalyticsSink{sink: sink, queue: make(chan *model.AuditLog, queueSize), batchSize: batchSize, flushEvery: flushEvery, stop: make(chan struct{}), done: make(chan struct{}), onError: func(err error) { log.Printf("[audit-analytics] batch write failed: %v", err) }}
	go a.run()
	return a
}

func (a *AsyncAuditAnalyticsSink) Enqueue(logEntry *model.AuditLog) bool {
	if a == nil || a.sink == nil || logEntry == nil {
		return false
	}
	select {
	case a.queue <- logEntry:
		return true
	default:
		a.report(context.DeadlineExceeded)
		return false
	}
}

func (a *AsyncAuditAnalyticsSink) Close() {
	if a == nil {
		return
	}
	a.once.Do(func() { close(a.stop); <-a.done })
}

func (a *AsyncAuditAnalyticsSink) run() {
	defer close(a.done)
	ticker := time.NewTicker(a.flushEvery)
	defer ticker.Stop()
	batch := make([]*model.AuditLog, 0, a.batchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := a.sink.WriteBatch(ctx, batch)
		cancel()
		if err != nil {
			a.report(err)
		}
		batch = batch[:0]
	}
	for {
		select {
		case entry := <-a.queue:
			batch = append(batch, entry)
			if len(batch) >= a.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-a.stop:
			for {
				select {
				case entry := <-a.queue:
					batch = append(batch, entry)
					if len(batch) >= a.batchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}

func (a *AsyncAuditAnalyticsSink) report(err error) {
	if a.onError != nil {
		a.onError(err)
	}
}
