import { useState, type FormEvent } from 'react'
import { api, saveSession, type User } from '../api'

export default function Login({ onLogin }: { onLogin: (u: User) => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const { token, user } = await api.login(username, password)
      const u = { ...user, username }
      saveSession(token, u)
      onLogin(u)
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-wrap">
      <form className="login-card" onSubmit={submit}>
        <h1>MCP-Nexus 工具市场</h1>
        <p className="hint">使用网关账号登录（admin / admin123）</p>
        <input
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          placeholder="用户名"
          autoFocus
        />
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="密码"
        />
        {error && <div className="error">{error}</div>}
        <button className="btn primary" disabled={loading || !username || !password}>
          {loading ? '登录中…' : '登录'}
        </button>
      </form>
    </div>
  )
}
