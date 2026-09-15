// API 客户端：统一携带 JWT、解包 {code,message,data} 响应
const TOKEN_KEY = 'mcp_token'
const USER_KEY = 'mcp_user'

export interface User {
  id: number
  role: 'admin' | 'developer' | 'agent'
  username?: string
}

export interface MarketToolItem {
  id: number
  name: string
  description: string
  category: string
  tags: string[]
  version: string
  health_status: string
  server_name: string
  call_count: number
  avg_rating: number
  review_count: number
  created_at: string
}

export interface MarketListResponse {
  items: MarketToolItem[]
  page: number
  page_size: number
  total: number
  total_pages: number
}

export interface MarketToolDetail extends MarketToolItem {
  input_schema: unknown
  server_endpoint: string
  server_health: string
  call_count_30d: number
  success_rate_30d: number
}

export interface Review {
  id: number
  tool_id: number
  user_id: number
  username: string
  rating: number
  comment: string
  created_at: string
}

export interface RankingItem {
  tool_name: string
  call_count: number
}

export interface ConfigSnippet {
  tool_name: string
  snippet: Record<string, unknown>
  notes: string[]
}

export class ApiError extends Error {
  code: string
  constructor(code: string, message: string) {
    super(message)
    this.code = code
  }
}

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function getUser(): User | null {
  const raw = localStorage.getItem(USER_KEY)
  return raw ? (JSON.parse(raw) as User) : null
}

export function saveSession(token: string, user: User) {
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}

export function clearSession() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers)
  headers.set('Content-Type', 'application/json')
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const resp = await fetch(path, { ...init, headers })
  const body = (await resp.json()) as { code: string; message: string; data: T }
  if (body.code !== 'OK') {
    if (resp.status === 401) clearSession()
    throw new ApiError(body.code, body.message)
  }
  return body.data
}

export const api = {
  async login(username: string, password: string): Promise<{ token: string; expires_at: string; user: User }> {
    return request('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    })
  },
  listTools(params: { category?: string; keyword?: string; page?: number; pageSize?: number }): Promise<MarketListResponse> {
    const q = new URLSearchParams()
    if (params.category) q.set('category', params.category)
    if (params.keyword) q.set('keyword', params.keyword)
    q.set('page', String(params.page ?? 1))
    q.set('page_size', String(params.pageSize ?? 9))
    return request(`/api/market/tools?${q}`)
  },
  getTool(id: number | string): Promise<MarketToolDetail> {
    return request(`/api/market/tools/${id}`)
  },
  getRanking(limit = 10): Promise<RankingItem[]> {
    return request(`/api/market/ranking?limit=${limit}`)
  },
  getReviews(toolId: number | string): Promise<Review[]> {
    return request(`/api/market/tools/${toolId}/reviews`)
  },
  upsertReview(toolId: number | string, rating: number, comment: string): Promise<Review> {
    return request(`/api/market/tools/${toolId}/reviews`, {
      method: 'POST',
      body: JSON.stringify({ rating, comment }),
    })
  },
  getConfigSnippet(toolId: number | string): Promise<ConfigSnippet> {
    return request(`/api/market/tools/${toolId}/config-snippet?gateway_url=${location.origin.replace(/:\d+$/, ':8080')}`)
  },
  publishTool(toolId: number | string): Promise<unknown> {
    return request(`/api/tools/${toolId}/publish`, { method: 'POST', body: '{}' })
  },
  offlineTool(toolId: number | string): Promise<unknown> {
    return request(`/api/tools/${toolId}/offline`, { method: 'POST', body: '{}' })
  },
  getOverview(): Promise<Overview> {
    return request('/api/analytics/overview')
  },
  getAuditLogs(params: { tool_name?: string; role?: string; success?: string; since_hours?: number; page?: number; page_size?: number }): Promise<AuditLogListResponse> {
    const q = new URLSearchParams()
    if (params.tool_name) q.set('tool_name', params.tool_name)
    if (params.role) q.set('role', params.role)
    if (params.success) q.set('success', params.success)
    if (params.since_hours) q.set('since_hours', String(params.since_hours))
    q.set('page', String(params.page ?? 1))
    q.set('page_size', String(params.page_size ?? 20))
    return request(`/api/audit/logs?${q}`)
  },
}

export interface Overview {
  total_calls: number
  success_count: number
  fail_count: number
  reject_count: number
  success_rate: number
  avg_latency_ms: number
  trend: { date: string; calls: number; success_rate: number }[]
  top_tools: RankingItem[]
  alerts: { tool_name: string; calls: number; reason: string }[]
}

export interface AuditLogEntry {
  request_id: string
  caller_id: number
  role: string
  tool_name: string
  server_id: number
  args_summary: string
  called_at: string
  latency_ms: number
  success: boolean
  http_status: number
  reject_reason: string
  cost_estimate: number
}

export interface AuditLogListResponse {
  items: AuditLogEntry[]
  page: number
  page_size: number
  total: number
  total_pages: number
}
