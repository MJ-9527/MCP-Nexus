import type { ApiResponse, LoginResponse, ToolListResponse, AnalyticsOverview, Alert, AlertListResponse } from './types';

const BASE = import.meta.env.VITE_API_URL || 'http://localhost:18080';

function token(): string {
  return localStorage.getItem('mcp_token') || '';
}

async function request<T>(method: string, path: string, body?: any): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const t = token();
  if (t) headers['Authorization'] = 'Bearer ' + t;
  const opts: RequestInit = { method, headers, signal: AbortSignal.timeout(10000) };
  if (body !== undefined) opts.body = JSON.stringify(body);
  const resp = await fetch(BASE + path, opts);
  const json: ApiResponse<T> = await resp.json().catch(() => ({ code: 'ERROR', message: resp.statusText, request_id: '' , data: null }));
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
  login: (u: string, p: string): Promise<LoginResponse> =>
    request<LoginResponse>('POST', '/api/auth/login', { username: u, password: p }),
  register: (u: string, p: string, role?: string) =>
    request('POST', '/api/auth/register', { username: u, password: p, role }),
  me: () => request('GET', '/api/auth/me'),
  tools: (params?: Record<string, string>) => {
    const qs = new URLSearchParams(params).toString();
    return request<ToolListResponse>('GET', '/mcp/tools' + (qs ? '?' + qs : '' ));
  },
  callTool: (n: string, a: Record<string, any>) =>
    request('POST', '/mcp/tools/' + encodeURIComponent(n) + '/call', a),
  analytics: (d = 7) => request<AnalyticsOverview>('GET', '/api/analytics/overview?days=' + d),
  alerts: (h?: boolean) => {
    const qs = h !== undefined ? '?handled=' + h : '';
    return request<AlertListResponse>('GET', '/api/alerts' + qs).then(r => r.items);
  },
  acknowledgeAlert: (id: string) =>
    request('PUT', '/api/alerts/' + id + '/ack' , {}),
  auditLogs: () => request('GET', '/api/audit/logs'),
  servers: () => request('GET', '/api/servers'),
};
