package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

// failingBatchRepo 计数 BatchCreate 调用次数与失败次数，用于验证 flush 容错。
type failingBatchRepo struct {
	mu        sync.Mutex
	batches   [][]*model.AuditLog
	failCount int32
	calls     int32
}

var _ repository.AuditLogRepository = (*failingBatchRepo)(nil)

func (r *failingBatchRepo) Create(_ context.Context, log *model.AuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return nil
}

func (r *failingBatchRepo) BatchCreate(_ context.Context, logs []*model.AuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	// 拷贝切片避免被复用影响断言
	dup := make([]*model.AuditLog, len(logs))
	copy(dup, logs)
	r.batches = append(r.batches, dup)
	if atomic.LoadInt32(&r.failCount) > 0 {
		atomic.AddInt32(&r.failCount, -1)
		return errors.New("injected batch failure")
	}
	return nil
}

func (r *failingBatchRepo) List(context.Context, repository.AuditLogFilter) ([]*model.AuditLog, int64, error) {
	return nil, 0, nil
}

// 验证：攒满 batchSize 立即 flush（不等 ticker）。
func TestBatchWriterFlushOnBatchSize(t *testing.T) {
	repo := &failingBatchRepo{}
	// 长间隔保证只能靠 size 触发
	w := NewBatchAuditWriter(repo, 5, time.Hour, 1024)
	w.Start()
	defer w.Stop()

	for i := 0; i < 5; i++ {
		w.Submit(&model.AuditLog{RequestID: "r", Status: "success", DurationMS: int64(i)})
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		repo.mu.Lock()
		got := len(repo.batches)
		repo.mu.Unlock()
		if got >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.batches) != 1 || len(repo.batches[0]) != 5 {
		t.Fatalf("期望一次 5 条批，实际 %+v", repo.batches)
	}
}

// 验证：未达 batchSize 时由 ticker 周期触发 flush。
func TestBatchWriterFlushOnInterval(t *testing.T) {
	repo := &failingBatchRepo{}
	w := NewBatchAuditWriter(repo, 100, 30*time.Millisecond, 1024)
	w.Start()
	defer w.Stop()

	w.Submit(&model.AuditLog{RequestID: "r1", Status: "success"})
	w.Submit(&model.AuditLog{RequestID: "r2", Status: "failed"})

	// 50ms 内应已通过 ticker flush
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		repo.mu.Lock()
		got := len(repo.batches)
		repo.mu.Unlock()
		if got >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.batches) != 1 {
		t.Fatalf("期望一次周期 flush，实际 batches=%d", len(repo.batches))
	}
}

// 验证：Stop 时残留记录必须 flush 落库。
func TestBatchWriterStopFlushesResidual(t *testing.T) {
	repo := &failingBatchRepo{}
	w := NewBatchAuditWriter(repo, 100, time.Hour, 1024)
	w.Start()

	w.Submit(&model.AuditLog{RequestID: "residual", Status: "success"})
	w.Stop() // 应立即触发最后一次 flush

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.batches) != 1 || len(repo.batches[0]) != 1 {
		t.Fatalf("Stop 应 flush 残留记录，实际 %+v", repo.batches)
	}
}

// 验证：队列满时 Submit 不阻塞，丢弃并计数。
func TestBatchWriterDropOnFullQueue(t *testing.T) {
	repo := &failingBatchRepo{}
	// batchSize 极大且 interval 极长，确保只靠队列满触发丢弃
	w := NewBatchAuditWriter(repo, 1<<20, time.Hour, 4)
	w.Start()
	defer w.Stop()

	// 一次性灌 10 条：channel 容量 4，必然丢弃 6 条
	for i := 0; i < 10; i++ {
		w.Submit(&model.AuditLog{RequestID: "r", Status: "success"})
	}
	stats := w.Stats()
	if stats.Dropped == 0 {
		t.Fatalf("队列满应丢弃并计数，实际 dropped=%d", stats.Dropped)
	}
	if stats.Dropped > 10 {
		t.Fatalf("丢弃数不应超过提交数，实际 dropped=%d", stats.Dropped)
	}
}

// 验证：flush 失败仅计数，不重试也不阻塞后续写入。
func TestBatchWriterFlushFailureTolerance(t *testing.T) {
	repo := &failingBatchRepo{}
	atomic.StoreInt32(&repo.failCount, 2) // 前两次 flush 失败
	w := NewBatchAuditWriter(repo, 2, 30*time.Millisecond, 1024)
	w.Start()
	defer w.Stop()

	// 提交 4 条：会触发 2 次 flush，都失败
	w.Submit(&model.AuditLog{RequestID: "r1", Status: "success"})
	w.Submit(&model.AuditLog{RequestID: "r2", Status: "success"})
	w.Submit(&model.AuditLog{RequestID: "r3", Status: "success"})
	w.Submit(&model.AuditLog{RequestID: "r4", Status: "success"})

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		stats := w.Stats()
		if stats.FlushFailed >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	stats := w.Stats()
	if stats.FlushFailed < 2 {
		t.Fatalf("期望 flush 失败计数 >= 2，实际 %d", stats.FlushFailed)
	}
	// 失败后服务仍应正常工作：再发 2 条，应当有成功 flush
	atomic.StoreInt32(&repo.failCount, 0)
	w.Submit(&model.AuditLog{RequestID: "r5", Status: "success"})
	w.Submit(&model.AuditLog{RequestID: "r6", Status: "success"})

	deadline = time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		stats := w.Stats()
		if stats.Flushed >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if stats := w.Stats(); stats.Flushed < 2 {
		t.Fatalf("失败恢复后应正常 flush，实际 flushed=%d", stats.Flushed)
	}
}
