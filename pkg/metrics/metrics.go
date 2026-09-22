package metrics

// ServiceMetrics 是 demo-service 在内存中的基础指标。
type ServiceMetrics struct {
	RequestsTotal    int64            `json:"requests_total"`
	RequestsByStatus map[int]int64    `json:"requests_by_status"`
	ToolCallsTotal   int64            `json:"tool_calls_total"`
	ToolCallsByName  map[string]int64 `json:"tool_calls_by_name"`
	ErrorsTotal      int64            `json:"errors_total"`
	ErrorDetails     map[string]int64 `json:"error_details"`
}

// New 初始化 ServiceMetrics。
func New() *ServiceMetrics {
	return &ServiceMetrics{
		RequestsByStatus: make(map[int]int64),
		ToolCallsByName:  make(map[string]int64),
		ErrorDetails:     make(map[string]int64),
	}
}

// RecordRequest 记录一次 HTTP 请求指标。
func (m *ServiceMetrics) RecordRequest(status int) {
	m.RequestsTotal++
	m.RequestsByStatus[status]++
	if status >= 400 {
		m.ErrorsTotal++
	}
}

// RecordTool 记录一次工具调用。
func (m *ServiceMetrics) RecordTool(tool string) {
	m.ToolCallsTotal++
	m.ToolCallsByName[tool]++
}

// RecordError 记录一次错误分类。
func (m *ServiceMetrics) RecordError(label string) {
	m.ErrorsTotal++
	m.ErrorDetails[label]++
}
