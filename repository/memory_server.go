package repository

import (
	"MCP-Nexus/model"
	"context"
	"strings"
	"sync"
)

type MemoryServerRepository struct {
	mu      sync.RWMutex
	nextID  int64
	servers map[int64]*model.MCPServer
}

func NewMemoryServerRepository() *MemoryServerRepository {
	return &MemoryServerRepository{nextID: 1, servers: make(map[int64]*model.MCPServer)}
}
func (r *MemoryServerRepository) Create(_ context.Context, s *model.MCPServer) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.servers {
		if strings.EqualFold(x.Name, s.Name) || x.Endpoint == s.Endpoint {
			return ErrConflict
		}
	}
	if s.ID == 0 {
		s.ID = r.nextID
		r.nextID++
	}
	x := *s
	r.servers[s.ID] = &x
	return nil
}
func (r *MemoryServerRepository) FindByName(_ context.Context, n string) (*model.MCPServer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.servers {
		if strings.EqualFold(s.Name, n) {
			x := *s
			return &x, nil
		}
	}
	return nil, ErrNotFound
}
func (r *MemoryServerRepository) FindByEndpoint(_ context.Context, e string) (*model.MCPServer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.servers {
		if s.Endpoint == e {
			x := *s
			return &x, nil
		}
	}
	return nil, ErrNotFound
}

func (r *MemoryServerRepository) List(_ context.Context) ([]*model.MCPServer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*model.MCPServer, 0, len(r.servers))
	for _, s := range r.servers {
		serverCopy := *s
		result = append(result, &serverCopy)
	}
	return result, nil
}

func (r *MemoryServerRepository) FindByID(_ context.Context, id int64) (*model.MCPServer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.servers {
		if s.ID == id {
			serverCopy := *s
			return &serverCopy, nil
		}
	}
	return nil, ErrNotFound
}
