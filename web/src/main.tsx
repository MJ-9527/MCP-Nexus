import { StrictMode, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Routes, Route, Navigate, NavLink } from 'react-router-dom'
import { getUser, clearSession, type User } from './api'
import Login from './pages/Login'
import Market from './pages/Market'
import ToolDetail from './pages/ToolDetail'
import Analytics from './pages/Analytics'
import AuditLogs from './pages/AuditLogs'
import './styles.css'

function App() {
  const [user, setUser] = useState<User | null>(getUser)

  if (!user) {
    return <Login onLogin={setUser} />
  }

  const logout = () => {
    clearSession()
    setUser(null)
  }

  return (
    <BrowserRouter>
      <header className="topbar">
        <span className="brand">MCP-Nexus 工具市场</span>
        <nav className="nav-links">
          <NavLink to="/" end>市场</NavLink>
          <NavLink to="/analytics">统计</NavLink>
          <NavLink to="/audit">审计</NavLink>
        </nav>
        <span className="spacer" />
        <span className="user-badge">
          {user.username ?? `用户 ${user.id}`}（{user.role}）
        </span>
        <button className="btn ghost" onClick={logout}>
          退出
        </button>
      </header>
      <main className="container">
        <Routes>
          <Route path="/" element={<Market />} />
          <Route path="/tools/:id" element={<ToolDetail role={user.role} />} />
          <Route path="/analytics" element={<Analytics />} />
          <Route path="/audit" element={<AuditLogs />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
    </BrowserRouter>
  )
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
