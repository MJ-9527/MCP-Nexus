package main

// Customer 是脱敏后的客户数据（数据库类示例工具）。
type Customer struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Phone  string `json:"phone"`
	Email  string `json:"email"`
	Region string `json:"region"`
}

const (
	// defaultFileBase 是 read_file 工具默认读取的目录（容器内）。
	defaultFileBase = "/app/data"
	// maxFetchBodyBytes 限制外部 HTTP 响应体大小（1MB）。
	maxFetchBodyBytes = int64(1 << 20)
)
