package service

import (
	"MCP-Nexus/repository"
	"context"
	"errors"
	"time"
)

var ErrInvalidRetentionPolicy = errors.New("invalid retention policy")

type AuditRetentionService struct {
	repo repository.AuditRetentionRepository
	now  func() time.Time
}

func NewAuditRetentionService(r repository.AuditRetentionRepository) *AuditRetentionService {
	return &AuditRetentionService{repo: r, now: time.Now}
}
func (s *AuditRetentionService) Preview(ctx context.Context, onlineDays, archiveDays int) (*repository.AuditRetentionPreview, error) {
	a, d, e := s.cutoffs(onlineDays, archiveDays)
	if e != nil {
		return nil, e
	}
	return s.repo.Preview(ctx, a, d)
}
func (s *AuditRetentionService) Run(ctx context.Context, onlineDays, archiveDays, batchSize int) (*repository.AuditRetentionResult, error) {
	if batchSize <= 0 || batchSize > 10000 {
		return nil, ErrInvalidRetentionPolicy
	}
	a, d, e := s.cutoffs(onlineDays, archiveDays)
	if e != nil {
		return nil, e
	}
	out := new(repository.AuditRetentionResult)
	for {
		n, e := s.repo.ArchiveBatch(ctx, a, batchSize)
		if e != nil {
			return nil, e
		}
		out.ArchivedRows += n
		if n < int64(batchSize) {
			break
		}
	}
	for {
		n, e := s.repo.DeleteArchiveBatch(ctx, d, batchSize)
		if e != nil {
			return nil, e
		}
		out.DeletedRows += n
		if n < int64(batchSize) {
			break
		}
	}
	return out, nil
}
func (s *AuditRetentionService) cutoffs(onlineDays, archiveDays int) (time.Time, time.Time, error) {
	if s == nil || s.repo == nil || onlineDays < 1 || archiveDays <= onlineDays || archiveDays > 3650 {
		return time.Time{}, time.Time{}, ErrInvalidRetentionPolicy
	}
	now := s.now().UTC()
	return now.AddDate(0, 0, -onlineDays), now.AddDate(0, 0, -archiveDays), nil
}
