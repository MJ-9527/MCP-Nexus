import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type MarketToolItem, type RankingItem } from '../api'

const CATEGORIES = ['全部', '数据库', '办公', '研发', '营销'] as const

function Stars({ value }: { value: number }) {
  const full = Math.round(value)
  return (
    <span className="stars" title={`${value.toFixed(1)} 分`}>
      {'★'.repeat(full)}
      {'☆'.repeat(5 - full)}
    </span>
  )
}

const HEALTH_LABEL: Record<string, string> = {
  online: '在线',
  offline: '离线',
  degraded: '降级',
  unknown: '未知',
}

export default function Market() {
  const [category, setCategory] = useState('')
  const [keyword, setKeyword] = useState('')
  const [input, setInput] = useState('')
  const [page, setPage] = useState(1)
  const [list, setList] = useState<MarketToolItem[]>([])
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(1)
  const [ranking, setRanking] = useState<RankingItem[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    api.listTools({ category: category || undefined, keyword: keyword || undefined, page })
      .then((r) => {
        setList(r.items)
        setTotal(r.total)
        setTotalPages(r.total_pages)
      })
      .catch((e) => setError(e instanceof Error ? e.message : '加载失败'))
  }, [category, keyword, page])

  useEffect(() => {
    api.getRanking(10).then(setRanking).catch(() => {})
  }, [])

  const search = (kw: string) => {
    setKeyword(kw)
    setPage(1)
  }

  return (
    <div className="market-layout">
      <section>
        <div className="toolbar">
          <div className="chips">
            {CATEGORIES.map((c) => (
              <button
                key={c}
                className={`chip ${category === (c === '全部' ? '' : c) ? 'active' : ''}`}
                onClick={() => {
                  setCategory(c === '全部' ? '' : c)
                  setPage(1)
                }}
              >
                {c}
              </button>
            ))}
          </div>
          <form
            className="search"
            onSubmit={(e) => {
              e.preventDefault()
              search(input)
            }}
          >
            <input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder="搜索工具名称 / 描述"
            />
            <button className="btn primary">搜索</button>
          </form>
        </div>

        {error && <div className="error">{error}</div>}

        <div className="cards">
          {list.map((t) => (
            <Link to={`/tools/${t.id}`} key={t.id} className="card">
              <div className="card-head">
                <strong>{t.name}</strong>
                <span className={`health ${t.health_status}`}>
                  {HEALTH_LABEL[t.health_status] ?? t.health_status}
                </span>
              </div>
              <p className="desc">{t.description || '（无描述）'}</p>
              <div className="card-meta">
                <span className="tag">{t.category || '未分类'}</span>
                <span>v{t.version}</span>
                <Stars value={t.avg_rating} />
                <span>{t.review_count} 条评价</span>
                <span>调用 {t.call_count} 次</span>
              </div>
            </Link>
          ))}
          {!error && list.length === 0 && <p className="empty">没有匹配的工具</p>}
        </div>

        <div className="pager">
          <button className="btn" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
            上一页
          </button>
          <span>
            第 {page} / {totalPages} 页（共 {total} 个）
          </span>
          <button
            className="btn"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => p + 1)}
          >
            下一页
          </button>
        </div>
      </section>

      <aside className="ranking">
        <h3>调用量排行（30 天）</h3>
        <ol>
          {ranking.map((r, i) => (
            <li key={r.tool_name}>
              <span className="rank-no">{i + 1}</span>
              <span className="rank-name">{r.tool_name}</span>
              <span className="rank-count">{r.call_count}</span>
            </li>
          ))}
          {ranking.length === 0 && <li className="empty">暂无数据</li>}
        </ol>
      </aside>
    </div>
  )
}
