import type { ApiResponse, LoginResponse, Tool, ToolListResponse, AnalyticsOverview, Alert, AlertListResponse, AuditLog, ServerInfo } from './types';

export const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:18080';

function token(): string {
  return localStorage.getItem('mcp_token') || '';
}

async function request<T>(method: string, path: string, body?: any): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const t = token();
  if (t) headers['Authorization'] = 'Bearer ' + t;
  const opts: RequestInit = { method, headers, signal: AbortSignal.timeout(10000) };
  if (body !== undefined) opts.body = JSON.stringify(body);
  const resp = await fetch(API_BASE + path, opts);
  const json: ApiResponse<T> = await resp.json().catch(() => ({ code: 'ERROR', message: resp.statusText, request_id: '', data: null }));
  if (!resp.ok) {
    if (resp.status === 401) {
      localStorage.removeItem('mcp_token');
      window.location.href = '/login';
      throw new Error('Unauthorized');
    }
    throw new Error(json.message || resp.statusText);
  }
  return json.data as T;
}

export const api = {
  // 认证
  login: (u: string, p: string): Promise<LoginResponse> =>
    request<LoginResponse>('POST', '/api/auth/login', { username: u, password: p }),
  me: (): Promise<{ id: number; username: string; role: string }> =>
    request('GET', '/api/auth/me'),

  // 工具市场：GET /api/tools 返回 {items,total,page,page_size}，这里适配为页面需要的 {tools,total}
  tools: async (params?: Record<string, string>): Promise<ToolListResponse> => {
    const qs = new URLSearchParams(params).toString();
    const data = await request<{ items: Tool[]; total: number }>('GET', '/api/tools' + (qs ? '?' + qs : ''));
    return { tools: data.items || [], total: data.total ?? 0 };
  },

  // MCP 网关：发现当前账号可调用的工具（权限过滤后的子集）
  mcpTools: () => request<{ tools: Tool[] }>('GET', '/mcp/tools'),
  callTool: (name: string, args: Record<string, any>) =>
    request('POST', '/mcp/tools/' + encodeURIComponent(name) + '/call', { arguments: args }),

  // 统计与告警（ClickHouse 分析存储启用时可用）
  analytics: (d = 7): Promise<AnalyticsOverview> =>
    request<AnalyticsOverview>('GET', '/api/analytics/overview?days=' + d),
  alerts: async (handled?: boolean): Promise<Alert[]> => {
    const status = handled === undefined ? '' : handled ? 'acknowledged' : 'open';
    const qs = status ? '?status=' + status : '';
    const res = await request<AlertListResponse>('GET', '/api/analytics/alerts' + qs);
    // 后端告警用 status=open/acknowledged，页面使用 handled 布尔，统一在 API 层适配
    return (res.items || []).map(a => ({ ...a, handled: a.handled ?? a.status === 'acknowledged' }));
  },
  acknowledgeAlert: (id: string | number) =>
    request('POST', '/api/analytics/alerts/' + id + '/acknowledge', {}),

  // 审计与服务
  // 后端记录的是工具调用审计（created_at/tool_name/status/http_status/duration_ms/caller_role），
  // 页面按 HTTP 请求审计的列展示，这里做一层字段适配。
  auditLogs: async (params?: Record<string, string>): Promise<{ items: AuditLog[]; total: number }> => {
    const qs = new URLSearchParams(params).toString();
    const res = await request<{ items: any[]; total: number }>('GET', '/api/audit/logs' + (qs ? '?' + qs : ''));
    const items = (res.items || []).map(l => {
      const code = l.http_status
        || (l.status === 'success' ? 200 : l.status === 'denied' ? 403 : 500);
      return {
        request_id: l.request_id,
        timestamp: l.created_at,
        method: 'CALL',
        path: l.tool_name ? '/mcp/tools/' + l.tool_name + '/call' : '-',
        status_code: code,
        latency_ms: l.duration_ms,
        username: l.caller_role,
        role: l.caller_role,
        message: l.denied_reason || l.reject_reason,
        tool_name: l.tool_name,
        status: l.status,
      } as AuditLog;
    });
    return { items, total: res.total ?? items.length };
  },
  servers: (): Promise<ServerInfo[] | { items: ServerInfo[] }> =>
    request('GET', '/api/servers'),
  importOpenAPI: (serverId: number, spec: unknown, published = true) =>
    request('POST', `/api/servers/${serverId}/import-openapi`, { spec, published }),
  importSkills: (serverId: number, tools: unknown[], published = true) =>
    request('POST', `/api/servers/${serverId}/import-skills`, { tools, published }),
};
