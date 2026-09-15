package permission

import "context"

// MockClient 模拟权限客户端，内存返回权限
type MockClient struct {
	MockData map[string][]string
}

func NewMockClient() *MockClient {
	return &MockClient{
		MockData: make(map[string][]string),
	}
}

func (m *MockClient) GetAllowedToolNamesByRole(_ context.Context, role string) ([]string, error) {
	return m.MockData[role], nil
}

var _ PermissionClient = (*MockClient)(nil)
