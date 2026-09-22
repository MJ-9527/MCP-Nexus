import { useState, useEffect, useCallback } from 'react';
import { api } from '../api';
import type { AuditLog } from '../types';

const METHOD_COLORS: Record<string, string> = {
  GET: 'bg-blue-100 text-blue-700',
  POST: 'bg-emerald-100 text-emerald-700',
  PUT: 'bg-amber-100 text-amber-700',
  DELETE: 'bg-red-100 text-red-700',
  DEFAULT: 'bg-slate-100 text-slate-600',
};

function StatusBadge({ code }: { code: number }) {
  const isOk = code >= 200 && code < 300;
  const isErr = code >= 400;
  return (
    <span className={`inline-flex px-2 py-0.5 rounded text-xs font-mono font-semibold ${isOk ? 'bg-emerald-100 text-emerald-700' : isErr ? 'bg-red-100 text-red-700' : 'bg-amber-100 text-amber-700'}`}>
      {code}
    </span>
  );
}

export default function AuditLogs() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<string>('all');
  const [search, setSearch] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.auditLogs();
      const items = Array.isArray(res) ? res : (res?.items || []);
      setLogs(items);
    } catch (e) { console.error(e); }
    finally { setLoading(false); }
  }, []);

  useEffect(() => { load(); }, [load]);

  const filtered = logs.filter(l => {
    const matchFilter = filter === 'error' ? l.status_code >= 400
      : filter === 'success' ? l.status_code < 400 : true;
    const matchSearch = search
      ? (l.path?.toLowerCase().includes(search.toLowerCase())
        || l.method?.toLowerCase().includes(search.toLowerCase())
        || (l.username || '').toLowerCase().includes(search.toLowerCase()))
      : true;
    return matchFilter && matchSearch;
  });

  const successCount = logs.filter(l => l.status_code < 400).length;
  const errorCount = logs.filter(l => l.status_code >= 400).length;

  const filterButtons = [
    { key: 'all', label: `全部 (${logs.length})` },
    { key: 'success', label: `成功 (${successCount})`, dot: successCount > 0 ? 'bg-emerald-500' : '' },
    { key: 'error', label: `错误 (${errorCount})`, dot: errorCount > 0 ? 'bg-red-500' : '' },
  ];

  return (
    <div className='p-8 animate-fade-in'>
      <div className='flex items-center justify-between mb-8'>
        <div>
          <h1 className='text-2xl font-bold text-slate-800'>审计日志</h1>
          <p className='text-slate-500 text-sm mt-1'>记录所有 API 请求的访问轨迹与安全事件</p>
        </div>
        <div className='flex gap-2 items-center'>
          {filterButtons.map(({ key, label, dot }) => (
            <button key={key} onClick={() => setFilter(key)} className={`px-4 py-2 rounded-xl text-sm font-medium transition-all flex items-center gap-2 ${filter === key ? 'bg-indigo-600 text-white shadow-md' : 'bg-white text-slate-600 border border-slate-200 hover:border-indigo-300'}`}>
              {dot && <span className={`w-2 h-2 rounded-full ${dot}`} />}
              {label}
            </button>
          ))}
        </div>
      </div>

      <div className='mb-4'>
        <div className='relative max-w-xs'>
          <span className='absolute left-3 top-1/2 -translate-y-1/2 text-slate-400'>🔍</span>
          <input
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder='搜索路径、方法或用户...'
            className='w-full pl-10 pr-4 py-2 rounded-xl border border-slate-200 bg-white text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500'
          />
        </div>
      </div>

      {loading ? (
        <div className='space-y-3'>
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className='bg-white rounded-xl border border-slate-200 p-4 loading-shimmer' style={{ height: 56 }} />
          ))}
        </div>
      ) : filtered.length === 0 ? (
        <div className='text-center py-20 bg-white rounded-xl border border-slate-200'>
          <div className='text-4xl mb-3'>📋</div>
          <p className='text-base font-medium text-slate-600'>暂无审计日志</p>
          <p className='text-sm text-slate-400 mt-1'>系统运行正常，暂无记录</p>
        </div>
      ) : (
        <div className='bg-white rounded-xl border border-slate-200 overflow-hidden'>
          <div className='overflow-x-auto'>
            <table className='w-full text-sm'>
              <thead>
                <tr className='bg-slate-50 border-b border-slate-200'>
                  <th className='px-4 py-3 text-left text-xs font-medium text-slate-500'>时间</th>
                  <th className='px-4 py-3 text-left text-xs font-medium text-slate-500'>方法</th>
                  <th className='px-4 py-3 text-left text-xs font-medium text-slate-500'>路径</th>
                  <th className='px-4 py-3 text-left text-xs font-medium text-slate-500'>状态</th>
                  <th className='px-4 py-3 text-left text-xs font-medium text-slate-500'>耗时</th>
                  <th className='px-4 py-3 text-left text-xs font-medium text-slate-500'>用户</th>
                  <th className='px-4 py-3 text-left text-xs font-medium text-slate-500'>请求ID</th>
                </tr>
              </thead>
              <tbody className='divide-y divide-slate-100'>
                {filtered.map(log => (
                  <tr key={log.request_id} className='hover:bg-slate-50 transition-colors'>
                    <td className='px-4 py-3 text-slate-500 whitespace-nowrap'>
                      {new Date(log.timestamp).toLocaleString('zh-CN')}
                    </td>
                    <td className='px-4 py-3'>
                      <span className={`inline-flex px-2 py-0.5 rounded text-xs font-mono font-semibold ${METHOD_COLORS[log.method] || METHOD_COLORS.DEFAULT}`}>
                        {log.method}
                      </span>
                    </td>
                    <td className='px-4 py-3 font-mono text-xs text-slate-600 max-w-[240px] truncate'>{log.path}</td>
                    <td className='px-4 py-3'><StatusBadge code={log.status_code} /></td>
                    <td className='px-4 py-3 text-slate-500 whitespace-nowrap'>{log.latency_ms} ms</td>
                    <td className='px-4 py-3 text-slate-600'>{log.username || '-'}</td>
                    <td className='px-4 py-3 text-xs text-slate-400 font-mono'>{log.request_id.slice(0, 8)}...</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className='px-4 py-3 border-t border-slate-200 text-xs text-slate-400 flex justify-between'>
            <span>共 {filtered.length} 条记录</span>
            <span>总计 {logs.length} 条</span>
          </div>
        </div>
      )}
    </div>
  );
}
