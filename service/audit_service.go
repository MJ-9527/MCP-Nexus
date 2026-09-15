package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"time"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

// AuditService 异步审计日志服务：通过 channel 缓冲 + 后台 goroutine 批量写入 ClickHouse。
type AuditService struct {
	repo  repository.AuditRepository
	queue chan *model.AuditLog
	now   func() time.Time
}

func NewAuditService(repo repository.AuditRepository) *AuditService {
	return &AuditService{
		repo:  repo,
		queue: make(chan *model.AuditLog, 1000),
		now:   time.Now,
	}
}

// RecordAsync 非阻塞记录一条审计日志。队列满时丢弃并告警（避免阻塞调用链）。
func (s *AuditService) RecordAsync(entry *model.AuditLog) {
	if entry == nil {
		return
	}
	if entry.CalledAt.IsZero() {
		entry.CalledAt = s.now()
	}
	select {
	case s.queue <- entry:
	default:
		log.Printf("[audit] 队列已满，丢弃日志: tool=%s caller=%d", entry.ToolName, entry.CallerID)
	}
}

// Start 启动后台消费 goroutine，每 500ms 或满 100 条 flush 一次。
// 服务退出时取消 ctx 即可停止。
func (s *AuditService) Start(ctx context.Context) {
	const (
		flushInterval = 500 * time.Millisecond
		batchSize     = 100
	)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batch := make([]*model.AuditLog, 0, batchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.repo.BatchInsert(ctx, batch); err != nil {
			log.Printf("[audit] 批量写入失败: %v", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			log.Println("审计日志服务已停止")
			return
		case entry := <-s.queue:
			batch = append(batch, entry)
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// SummarizeArgs 将调用参数 JSON 化后取 SHA256 前 16 位 + 原长度，用于脱敏审计（需求 §4.8）。
func SummarizeArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return ""
	}
	raw, _ := json.Marshal(args)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:16] + "|len=" + itoa(len(raw))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
