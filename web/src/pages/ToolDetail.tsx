import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  api,
  type ConfigSnippet,
  type MarketToolDetail,
  type Review,
} from '../api'

const HEALTH_LABEL: Record<string, string> = {
  online: '在线',
  offline: '离线',
  degraded: '降级',
  unknown: '未知',
}

export default function ToolDetail({ role }: { role: string }) {
  const { id } = useParams()
  const [tool, setTool] = useState<MarketToolDetail | null>(null)
  const [reviews, setReviews] = useState<Review[]>([])
  const [snippet, setSnippet] = useState<ConfigSnippet | null>(null)
  const [rating, setRating] = useState(5)
  const [comment, setComment] = useState('')
  const [msg, setMsg] = useState('')
  const [error, setError] = useState('')
  const isAdmin = role === 'admin'

  const load = useCallback(() => {
    if (!id) return
    api.getTool(id).then(setTool).catch((e) => setError(e.message))
    api.getReviews(id).then(setReviews).catch(() => {})
  }, [id])

  useEffect(load, [load])

  const loadSnippet = () => {
    if (!id) return
    api
      .getConfigSnippet(id)
      .then(setSnippet)
      .catch((e) => setError(e.message))
  }

  const submitReview = async () => {
    if (!id) return
    setMsg('')
    setError('')
    try {
      await api.upsertReview(id, rating, comment)
      setComment('')
      setMsg('评价已提交')
      load()
    } catch (e) {
      setError(e instanceof Error ? e.message : '提交失败')
    }
  }

  const togglePublish = async (action: 'publish' | 'offline') => {
    if (!id) return
    setMsg('')
    setError('')
    try {
      await (action === 'publish' ? api.publishTool(id) : api.offlineTool(id))
      setMsg(action === 'publish' ? '已发布' : '已下线')
      load()
    } catch (e) {
      setError(e instanceof Error ? e.message : '操作失败')
    }
  }

  if (!tool) return <p className="empty">{error || '加载中…'}</p>

  return (
    <article className="detail">
      <Link to="/" className="back">
        ← 返回市场
      </Link>
      <header className="detail-head">
        <h2>{tool.name}</h2>
        <span className={`health ${tool.health_status}`}>
          {HEALTH_LABEL[tool.health_status] ?? tool.health_status}
        </span>
        {isAdmin && (
          <span className="admin-actions">
            <button className="btn" onClick={() => togglePublish('publish')}>
              发布
            </button>
            <button className="btn danger" onClick={() => togglePublish('offline')}>
              下线
            </button>
          </span>
        )}
      </header>
      <p>{tool.description || '（无描述）'}</p>

      <div className="meta-grid">
        <div>
          <label>版本</label>
          <span>v{tool.version}</span>
        </div>
        <div>
          <label>分类</label>
          <span>{tool.category || '未分类'}</span>
        </div>
        <div>
          <label>所属 Server</label>
          <span>
            {tool.server_name}（{tool.server_endpoint}）
          </span>
        </div>
        <div>
          <label>30 天调用</label>
          <span>{tool.call_count_30d} 次，成功率 {tool.success_rate_30d.toFixed(1)}%</span>
        </div>
        <div>
          <label>评分</label>
          <span>
            {tool.avg_rating.toFixed(1)} / 5（{tool.review_count} 条）
          </span>
        </div>
      </div>

      <section>
        <h3>输入 Schema</h3>
        <pre className="code">
          {tool.input_schema ? JSON.stringify(tool.input_schema, null, 2) : '（无）'}
        </pre>
      </section>

      <section>
        <h3>接入配置片段</h3>
        {snippet ? (
          <>
            <pre className="code">{JSON.stringify(snippet.snippet, null, 2)}</pre>
            <ul className="notes">
              {snippet.notes.map((n) => (
                <li key={n}>{n}</li>
              ))}
            </ul>
          </>
        ) : (
          <button className="btn" onClick={loadSnippet}>
            一键生成 MCP 接入配置
          </button>
        )}
      </section>

      <section>
        <h3>评分与评论</h3>
        <div className="review-form">
          <select value={rating} onChange={(e) => setRating(Number(e.target.value))}>
            {[5, 4, 3, 2, 1].map((v) => (
              <option key={v} value={v}>
                {'★'.repeat(v)}
              </option>
            ))}
          </select>
          <input
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            placeholder="写下你的评价（可选）"
          />
          <button className="btn primary" onClick={submitReview}>
            提交
          </button>
        </div>
        {msg && <p className="ok">{msg}</p>}
        {error && <p className="error">{error}</p>}
        <ul className="reviews">
          {reviews.map((r) => (
            <li key={r.id}>
              <div className="review-head">
                <strong>{r.username}</strong>
                <span className="stars">{'★'.repeat(r.rating)}</span>
                <time>{new Date(r.created_at).toLocaleString()}</time>
              </div>
              {r.comment && <p>{r.comment}</p>}
            </li>
          ))}
          {reviews.length === 0 && <li className="empty">暂无评价</li>}
        </ul>
      </section>
    </article>
  )
}
