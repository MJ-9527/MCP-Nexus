export interface User {
  id: string;
  username: string;
  role: string;
  permissions?: string[];
}

export interface LoginResponse {
  token: string;
  token_type: string;
  expires_in: number;
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
  input_schema?: string;
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
  daily_trend: Array<{ date: string; calls: number; success: number; rejected: number }>;
  alerts: any[];
}

export interface Alert {
  alert_id: string;
  type: string;
  severity: string;
  tool_id: number;
  tool_name: string;
  message: string;
  triggered_at: string;
  handled: boolean;
}


export interface AlertListResponse {\n  items: Alert[];\n  total: number;\n}\n\nexport interface AuditLog {\n  request_id: string;\n  timestamp: string;\n  method: string;\n  path: string;\n  status_code: number;\n  latency_ms: number;\n  username?: string;\n  role?: string;\n  code?: string;\n  message?: string;\n  tool_name?: string;\n}\n\nexport interface ServerInfo {\n  id: number;\n  name: string;\n  endpoint: string;\n  status: string;\n  health_status: string;\n  last_check_at: string;\n}
export interface ApiResponse<T = any> {
  code: string;
  message: string;
  request_id: string;
  data: T;
}
