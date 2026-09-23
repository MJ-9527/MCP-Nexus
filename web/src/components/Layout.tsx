import { Outlet, NavLink, useNavigate } from 'react-router-dom';

const navItems = [
  { to: '/tools', label: '工具市场', icon: '◆' },
  { to: '/dashboard', label: '数据分析', icon: '📊' },
  { to: '/import', label: '工具导入', icon: '📥' },
  { to: '/alerts', label: '告警中心', icon: '🔔' },
  { to: '/config', label: '接入配置', icon: '⚙️' },
  { to: '/audit', label: '审计日志', icon: '📋' },
];

export default function Layout() {
  const navigate = useNavigate();
  const username = localStorage.getItem('mcp_username') || 'User';
  const role = localStorage.getItem('mcp_role') || 'viewer';

  return (
    <div className='flex h-screen overflow-hidden bg-slate-50'>
      <aside className='w-64 bg-slate-900 text-white flex flex-col shrink-0 shadow-xl'>
        <div className='px-6 py-5 border-b border-slate-700'>
          <div className='flex items-center gap-3'>
            <div className='w-8 h-8 rounded-lg bg-indigo-500 flex items-center justify-center text-lg font-bold'>M</div>
            <div>
              <div className='font-bold text-base tracking-wide'>MCP Nexus</div>
              <div className='text-xs text-slate-400'>工具市场 & 网关</div>
            </div>
          </div>
        </div>
        <nav className='flex-1 py-4 px-3 space-y-1'>
          {navItems.map(({ to, label, icon }) => (
            <NavLink key={to} to={to} end>{({ isActive }) => (
              <div className={'flex items-center gap-3 px-3 py-2.5 rounded-lg cursor-pointer transition-all duration-200 ' + (isActive ? 'bg-indigo-600 text-white shadow-md' : 'text-slate-300 hover:bg-slate-800 hover:text-white')}>
                <span className='text-base w-5 text-center'>{icon}</span>
                <span className='text-sm font-medium'>{label}</span>
              </div>
            )}</NavLink>
          ))}
        </nav>
        <div className='px-4 py-4 border-t border-slate-700'>
          <div className='flex items-center gap-3 mb-3'>
            <div className='w-8 h-8 rounded-full bg-indigo-500/20 border border-indigo-500/40 flex items-center justify-center text-sm'>{username[0].toUpperCase()}</div>
            <div className='flex-1 min-w-0'>
              <div className='text-sm font-medium truncate'>{username}</div>
              <div className='text-xs text-slate-400 capitalize'>{role}</div>
            </div>
          </div>
          <button onClick={() => { localStorage.removeItem('mcp_token'); localStorage.removeItem('mcp_username'); navigate('/login'); }} className='w-full py-2 text-sm text-slate-400 hover:text-red-400 transition-colors text-left'>← 退出登录</button>
        </div>
      </aside>
      <main className='flex-1 overflow-auto'>
        <Outlet />
      </main>
    </div>
  );
}
