package main

// serviceMetrics 是 demo-service 在内存中的基础指标。
type serviceMetrics struct {
	RequestsTotal    int64            `json:"requests_total"`
	RequestsByStatus map[int]int64    `json:"requests_by_status"`
	ToolCallsTotal   int64            `json:"tool_calls_total"`
	ToolCallsByName  map[string]int64 `json:"tool_calls_by_name"`
	ErrorsTotal      int64            `json:"errors_total"`
	ErrorDetails     map[string]int64 `json:"error_details"`
}

func newServiceMetrics() *serviceMetrics {
	return &serviceMetrics{
		RequestsByStatus: make(map[int]int64),
		ToolCallsByName:  make(map[string]int64),
		ErrorDetails:     make(map[string]int64),
	}
}

func (m *serviceMetrics) recordRequest(status int) {
	m.RequestsTotal++
	m.RequestsByStatus[status]++
	if status >= 400 {
		m.ErrorsTotal++
	}
}

func (m *serviceMetrics) recordTool(tool string) {
	m.ToolCallsTotal++
	m.ToolCallsByName[tool]++
}

func (m *serviceMetrics) recordError(label string) {
	m.ErrorsTotal++
	m.ErrorDetails[label]++
}
