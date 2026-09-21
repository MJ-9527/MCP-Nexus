package service

import (
	"MCP-Nexus/model"
	"context"
	"errors"
	"testing"
	"time"
)

type fakeAnalyticsSink struct {
	batches int
	fail    bool
}

func (f *fakeAnalyticsSink) WriteBatch(_ context.Context, logs []*model.AuditLog) error {
	if f.fail {
		return errors.New("sink unavailable")
	}
	if len(logs) > 0 {
		f.batches++
	}
	return nil
}

func TestAsyncAuditAnalyticsSinkDoesNotBlockWhenQueueIsFull(t *testing.T) {
	fake := &fakeAnalyticsSink{}
	sink := NewAsyncAuditAnalyticsSink(fake, 1, 100, time.Hour)
	defer sink.Close()
	entry := &model.AuditLog{RequestID: "r1"}
	if !sink.Enqueue(entry) {
		t.Fatal("first entry should be queued")
	}
	if sink.Enqueue(entry) {
		t.Fatal("second entry should fail fast when queue is full")
	}
}
