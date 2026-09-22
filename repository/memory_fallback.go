package repository

import (
	"MCP-Nexus/model"
	"context"
	"errors"
	"sync"
)

// MemToolRepo is an in-memory ToolRepository for fallback mode (no PostgreSQL).
type MemToolRepo struct {
	mu    sync.RWMutex
	next  int64
	tools map[int64]*model.MCPTool
	byKey map[string]*model.MCPTool
}

func (r *MemToolRepo) Create(_ context.Context, t *model.MCPTool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tools == nil {
		r.tools = make(map[int64]*model.MCPTool)
	}
	if r.byKey == nil {
		r.byKey = make(map[string]*model.MCPTool)
	}
	if t.ID == 0 {
		r.next++
		t.ID = r.next
	}
	key := "s" + itoa(t.ServerID) + ":n" + t.Name
	if _, exists := r.byKey[key]; exists {
		return ErrConflict
	}
	cp := *t
	r.tools[t.ID] = &cp
	r.byKey[key] = &cp
	return nil
}
func (r *MemToolRepo) FindByID(_ context.Context, id int64) (*model.MCPTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if t, ok := r.tools[id]; ok {
		x := *t
		return &x, nil
	}
	return nil, ErrNotFound
}
func (r *MemToolRepo) FindByName(_ context.Context, serverID int64, name string) (*model.MCPTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if t, ok := r.byKey["s"+itoa(serverID)+":n"+name]; ok {
		x := *t
		return &x, nil
	}
	return nil, ErrNotFound
}
func (r *MemToolRepo) List(_ context.Context, _ ToolFilter) ([]*model.MCPTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*model.MCPTool, 0, len(r.tools))
	for _, t := range r.tools {
		x := *t
		out = append(out, &x)
	}
	return out, nil
}
func (r *MemToolRepo) UpdatePublished(_ context.Context, id int64, published bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tools[id]; ok {
		t.Published = published
		return nil
	}
	return ErrNotFound
}

// MemPermRepo is an in-memory ToolPermissionRepository.
type MemPermRepo struct {
	mu    sync.RWMutex
	perms map[int64][]*model.ToolPermission
	next  int64
}

func (r *MemPermRepo) Grant(_ context.Context, p *model.ToolPermission) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.perms == nil {
		r.perms = make(map[int64][]*model.ToolPermission)
	}
	if p.ID == 0 {
		r.next++
		p.ID = r.next
	}
	r.perms[p.ToolID] = append(r.perms[p.ToolID], p)
	return nil
}
func (r *MemPermRepo) Revoke(_ context.Context, _ int64, _, _ *int64, _ string) error {
	return errors.New("not implemented in memory mode")
}
func (r *MemPermRepo) ListByTool(_ context.Context, toolID int64) ([]*model.ToolPermission, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]*model.ToolPermission{}, r.perms[toolID]...), nil
}
func (r *MemPermRepo) HasPermission(_ context.Context, _, toolID int64, _ string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.perms[toolID]) > 0, nil
}

// MemUserRepo is an in-memory UserRepository.
type MemUserRepo struct {
	mu    sync.RWMutex
	users map[string]*model.User
	next  int64
}

func (r *MemUserRepo) Create(_ context.Context, u *model.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.users == nil {
		r.users = make(map[string]*model.User)
	}
	if _, exists := r.users[u.Username]; exists {
		return ErrConflict
	}
	if u.ID == 0 {
		r.next++
		u.ID = r.next
	}
	cp := *u
	r.users[u.Username] = &cp
	return nil
}
func (r *MemUserRepo) FindByUsername(_ context.Context, username string) (*model.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if u, ok := r.users[username]; ok {
		x := *u
		return &x, nil
	}
	return nil, ErrNotFound
}
func (r *MemUserRepo) CreateRole(_ context.Context, _ *model.Role) error { return nil }
func (r *MemUserRepo) AssignRole(_ context.Context, _, _ int64) error   { return nil }

func itoa(n int64) string {
	if n == 0 { return "0" }
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
