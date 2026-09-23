export interface User {
  id: number | string;
  username: string;
  role: string;
  permissions?: string[];
}

export interface LoginResponse {
  token: string;
  token_type: string;
  expires_at?: string;
  user: User;
}

export interface Tool {
  id: number;
  name: string;
  description: string;
  category: string;
  tags: string[];
  version: string;
  health_status: string;
  call_count: number;
  published: boolean;
  input_schema?: string | Record<string, any>;
}

export interface ToolListResponse {
  tools: Tool[];
  total: number;
}

export interface AnalyticsOverview {
  period: { days: number };
  summary: {
    total_calls: number;
    successful_calls: number;
    failed_calls: number;
    rejected_calls: number;
    success_rate: number;
    avg_latency_ms: number;
  };
  top_tools: Array<{ tool_id: number; tool_name: string; call_count: number; success_rate: number }>;
  daily_trend: Array<{ time?: string; date?: string; calls: number; success: number; failed?: number; rejected: number }>;
  alerts: Alert[];
}

export interface Alert {
  alert_id: number | string;
  type: string;
  severity: string;
  tool_id?: number;
  tool_name: string;
  message: string;
  triggered_at: string;
  status?: string;
  handled: boolean;
}

export interface AlertListResponse {
  items: Alert[];
  total: number;
}

export interface AuditLog {
  id?: number;
  request_id: string;
  created_at?: string;
  tool_name?: string;
  caller_role?: string;
  duration_ms?: number;
  status: string;
  http_status?: number;
	params_summary?: string;
	params_digest?: string;
	params_sensitive_masked?: boolean;
  denied_reason?: string;
  username?: string;
  role?: string;
  code?: string;
  message?: string;
	// UI 展示字段，由 api.auditLogs 根据后端审计数据转换。
	method?: string;
	path?: string;
	status_code?: number;
	latency_ms?: number;
	timestamp?: string;
}

export interface ServerInfo {
  id: number;
  name: string;
  endpoint: string;
  status: string;
  health_status: string;
  last_health_check_at?: string;
}

export interface ApiResponse<T = any> {
  code: string;
  message: string;
  request_id: string;
  data: T;
}
