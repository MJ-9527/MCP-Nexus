import { useEffect, useState } from 'react'
import { api, type AuditLogEntry } from '../api'

export default function AuditLogs() {
  const [logs, setLogs] = useState<AuditLogEntry[]>([])
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(1)
  const [page, setPage] = useState(1)
  const [toolName, setToolName] = useState('')
  const [role, setRole] = useState('')
  const [success, setSuccess] = useState('')
  const [error, setError] = useState('')

  const load = () => {
    api
      .getAuditLogs({ tool_name: toolName || undefined, role: role || undefined, success: success || undefined, page })
      .then((r) => {
        setLogs(r.items)
        setTotal(r.total)
        setTotalPages(r.total_pages)
      })
      .catch((e) => setError(e instanceof Error ? e.message : '加载失败'))
  }

  useEffect(load, [page, toolName, role, success])

  return (
    <div className="audit-page">
      <h2>审计日志</h2>

      <div className="filter-bar">
        <input
          value={toolName}
          onChange={(e) => {
            setToolName(e.target.value)
            setPage(1)
          }}
          placeholder="工具名称"
        />
        <select
          value={role}
          onChange={(e) => {
            setRole(e.target.value)
            setPage(1)
          }}
        >
          <option value="">全部角色</option>
          <option value="admin">admin</option>
          <option value="developer">developer</option>
          <option value="agent">agent</option>
        </select>
        <select
          value={success}
          onChange={(e) => {
            setSuccess(e.target.value)
            setPage(1)
          }}
        >
          <option value="">全部状态</option>
          <option value="true">成功</option>
          <option value="false">失败</option>
        </select>
      </div>

      {error && <p className="error">{error}</p>}

      <table className="data-table">
        <thead>
          <tr>
            <th>时间</th>
            <th>请求 ID</th>
            <th>调用者</th>
            <th>角色</th>
            <th>工具</th>
            <th>状态</th>
            <th>HTTP</th>
            <th>延迟</th>
            <th>拒绝原因</th>
          </tr>
        </thead>
        <tbody>
          {logs.map((l) => (
            <tr key={l.request_id + l.called_at}>
              <td>{new Date(l.called_at).toLocaleString()}</td>
              <td className="mono">{l.request_id.slice(0, 8)}</td>
              <td>{l.caller_id}</td>
              <td>{l.role}</td>
              <td>{l.tool_name}</td>
              <td className={l.success ? 'ok' : 'error'}>
                {l.success ? '成功' : '失败'}
              </td>
              <td>{l.http_status}</td>
              <td>{l.latency_ms}ms</td>
              <td>{l.reject_reason || '-'}</td>
            </tr>
          ))}
          {logs.length === 0 && (
            <tr>
              <td colSpan={9} className="empty">
                暂无审计日志
              </td>
            </tr>
          )}
        </tbody>
      </table>

      <div className="pager">
        <button className="btn" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
          上一页
        </button>
        <span>
          第 {page} / {totalPages} 页（共 {total} 条）
        </span>
        <button className="btn" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>
          下一页
        </button>
      </div>
    </div>
  )
}
