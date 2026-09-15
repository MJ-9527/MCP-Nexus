import { useEffect, useState } from 'react'
import { api, type Overview } from '../api'

function StatCard({ label, value, unit, color }: { label: string; value: string | number; unit?: string; color?: string }) {
  return (
    <div className="stat-card" style={color ? { borderColor: color } : {}}>
      <div className="stat-value" style={color ? { color } : {}}>
        {value}
        {unit && <span className="stat-unit">{unit}</span>}
      </div>
      <div className="stat-label">{label}</div>
    </div>
  )
}

function TrendChart({ trend }: { trend: { date: string; calls: number; success_rate: number }[] }) {
  if (!trend.length) return <p className="empty">暂无趋势数据</p>
  const maxCalls = Math.max(...trend.map((t) => t.calls), 1)
  return (
    <div className="trend-chart">
      {trend.map((t) => (
        <div key={t.date} className="trend-bar">
          <div className="trend-count">{t.calls}</div>
          <div className="trend-fill" style={{ height: `${(t.calls / maxCalls) * 100}%` }} />
          <div className="trend-date">{t.date.slice(5)}</div>
        </div>
      ))}
    </div>
  )
}

export default function Analytics() {
  const [data, setData] = useState<Overview | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    api.getOverview().then(setData).catch((e) => setError(e instanceof Error ? e.message : '加载失败'))
  }, [])

  if (error) return <p className="error">{error}</p>
  if (!data) return <p className="empty">加载中…</p>

  return (
    <div className="analytics">
      <h2>调用统计（近 7 天）</h2>

      <div className="stat-grid">
        <StatCard label="总调用次数" value={data.total_calls} />
        <StatCard label="成功次数" value={data.success_count} color="#16a34a" />
        <StatCard label="失败次数" value={data.fail_count} color="#dc2626" />
        <StatCard label="权限拒绝" value={data.reject_count} color="#ea580c" />
        <StatCard label="成功率" value={data.success_rate.toFixed(1)} unit="%" />
        <StatCard label="平均延迟" value={data.avg_latency_ms.toFixed(1)} unit="ms" />
      </div>

      <section>
        <h3>调用趋势</h3>
        <TrendChart trend={data.trend} />
      </section>

      <section>
        <h3>Top 工具排行</h3>
        <table className="data-table">
          <thead>
            <tr>
              <th>#</th>
              <th>工具名称</th>
              <th>调用量</th>
            </tr>
          </thead>
          <tbody>
            {data.top_tools.map((t, i) => (
              <tr key={t.tool_name}>
                <td>{i + 1}</td>
                <td>{t.tool_name}</td>
                <td>{t.call_count}</td>
              </tr>
            ))}
            {data.top_tools.length === 0 && (
              <tr>
                <td colSpan={3} className="empty">
                  暂无数据
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </section>

      <section>
        <h3>异常告警</h3>
        {data.alerts.length > 0 ? (
          <div className="alerts">
            {data.alerts.map((a) => (
              <div key={a.tool_name} className="alert-item">
                <span className="alert-icon">⚠</span>
                <strong>{a.tool_name}</strong>
                <span className="alert-calls">{a.calls} 次/小时</span>
                <span className="alert-reason">{a.reason}</span>
              </div>
            ))}
          </div>
        ) : (
          <p className="ok">无异常告警</p>
        )}
      </section>
    </div>
  )
}
