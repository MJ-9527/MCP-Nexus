import { useState, useEffect, useCallback } from 'react';
import { api } from '../api';
import type { Alert } from '../types';

const SEVERITY_CONFIG = {
  high: { bg: 'bg-red-50' , border: 'border-red-200' , text: 'text-red-700' , badge: 'bg-red-100 text-red-700' , dot: 'bg-red-500' },
  medium: { bg: 'bg-amber-50' , border: 'border-amber-200' , text: 'text-amber-700' , badge: 'bg-amber-100 text-amber-700' , dot: 'bg-amber-500' },
  low: { bg: 'bg-blue-50' , border: 'border-blue-200' , text: 'text-blue-700' , badge: 'bg-blue-100 text-blue-700' , dot: 'bg-blue-500' },
};

function SeverityBadge({ severity }: { severity: string }) {
  const cfg = SEVERITY_CONFIG[severity as keyof typeof SEVERITY_CONFIG] || SEVERITY_CONFIG.low;
  return (
    <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold ${cfg.badge}`}>
      <span className={`w-1.5 h-1.5 rounded-full ${cfg.dot}`} />
      {severity === 'high' ? '高风险' : severity === 'medium' ? '中风险' : '低风险'}
    </span>
  );
}

export default function Alerts() {
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<'all' | 'unhandled' | 'handled' >('all');
  const [ackId, setAckId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const handled = filter === 'unhandled' ? false : filter === 'handled' ? true : undefined;
      const res = await api.alerts(handled);
      setAlerts(res || []);
    } catch (e) { console.error(e); }
    finally { setLoading(false); }
  }, [filter]);

  useEffect(() => { load(); }, [load]);

  async function handleAck(id: string) {
    setAckId(id);
    try {
      await api.acknowledgeAlert(id);
      load();
    } finally {
      setAckId(null);
    }
  }

  const unhandled = alerts.filter(a => !a.handled).length;
  const handled = alerts.filter(a => a.handled).length;

  return (
    <div className="p-8 animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-slate-800">告警中心</h1>
          <p className="text-slate-500 text-sm mt-1">监控工具调用异常与安全风险</p>
        </div>
        <div className="flex gap-2">
          {[
            { key: 'all' as const, label: `全部 (${alerts.length})` },
            { key: 'unhandled' as const, label: `未处理 (${unhandled})`, dot: unhandled > 0 ? 'bg-red-500' : "", },
            { key: 'handled' as const, label: `已处理 (${handled})` },
          ].map(({ key, label, dot }) => (
            <button
              key={key}
              onClick={() => setFilter(key)}
              className={`px-4 py-2 rounded-xl text-sm font-medium transition-all flex items-center gap-2 ${
                filter === key
                  ? 'bg-indigo-600 text-white shadow-md'
                  : 'bg-white text-slate-600 border border-slate-200 hover:border-indigo-300'
              }`}
            >
              {dot && <span className={`w-2 h-2 rounded-full ${dot} ${filter !== 'unhandled' ? 'opacity-0' : ''}`} />}
              {label}
            </button>
          ))}
        </div>
      </div>

      {/* Content */}
      {loading ? (
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="bg-white rounded-xl border border-slate-200 p-5 loading-shimmer" style={{ height: 96 }} />
          ))}
        </div>
      ) : alerts.length === 0 ? (
        <div className="text-center py-20 bg-white rounded-xl border border-slate-200">
          <div className="text-4xl mb-3">{filter === 'handled' ? '🎉' : '🔔'}</div>
          <p className="text-base font-medium text-slate-600">
            {filter === 'handled' ? '没有已处理的告警' : filter === 'unhandled' ? '暂无未处理告警' : '暂无告警'}
          </p>
          <p className="text-sm text-slate-400 mt-1">系统运行正常</p>
        </div>
      ) : (
        <div className="space-y-3">
          {alerts.map(alert => {
            const cfg = SEVERITY_CONFIG[alert.severity as keyof typeof SEVERITY_CONFIG] || SEVERITY_CONFIG.low;
            return (
              <div
                key={alert.alert_id}
                className={`bg-white rounded-xl border p-5 transition-all hover:shadow-md ${
                  cfg.border} ${alert.handled ? 'opacity-60' : ''} animate-slide-in` }
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="flex items-start gap-4 min-w-0">
                    <div className={`w-10 h-10 rounded-xl flex items-center justify-center text-lg shrink-0 ${cfg.bg}`}>
                      {alert.severity === 'high' ? '🚨' : alert.severity === 'medium' ? '⚠️' : 'ℹ️'}
                    </div>
                    <div className="min-w-0">
                      <div className="flex items-center gap-2 flex-wrap mb-1">
                        <SeverityBadge severity={alert.severity} />
                        <span className="text-xs text-slate-400">{alert.type}</span>
                        {alert.handled && <span className="px-2 py-0.5 rounded-full bg-emerald-100 text-emerald-700 text-xs font-medium">已处理</span>}
                      </div>
                      <h4 className="font-semibold text-slate-700 truncate">{alert.tool_name}</h4>
                      <p className="text-sm text-slate-500 mt-0.5">{alert.message}</p>
                      <p className="text-xs text-slate-400 mt-1">{new Date(alert.triggered_at).toLocaleString('zh-CN')}</p>
                    </div>
                  </div>
                  {!alert.handled && (
                    <button
                      onClick={() => handleAck(alert.alert_id)}
                      disabled={ackId === alert.alert_id}
                      className="px-4 py-2 rounded-lg bg-indigo-600 text-white text-sm font-medium hover:bg-indigo-700 transition-colors shrink-0 disabled:opacity-60"
                    >
                      {ackId === alert.alert_id ? '处理中...' : '确认处理'}
                    </button>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}