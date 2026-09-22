import { useState, useEffect, useCallback } from 'react';
import { api } from '../api';

function StatCard({ label, value, unit, color, icon }: { label: string; value: string | number; unit?: string; color: string; icon: string }) {
  return (
    <div className="bg-white rounded-xl border border-slate-200 p-5 hover:shadow-md transition-shadow">
      <div className="flex items-center justify-between mb-3">
        <span className="text-slate-500 text-sm">{label}</span>
        <span className="text-2xl">{icon}</span>
      </div>
      <div className="flex items-baseline gap-1">
        <span className="text-3xl font-bold text-slate-800">{value}</span>
        {unit && <span className="text-sm text-slate-400">{unit}</span>}
      </div>
      <div className="mt-2 h-1 rounded-full bg-slate-100 overflow-hidden">
        <div className="h-full rounded-full transition-all duration-500" style={{ width: '100%', background: color }} />
      </div>
    </div>
  );
}

function TrendBar({ day, calls, max }: { day: string; calls: number; max: number }) {
  const pct = max > 0 ? (calls / max) * 100 : 0;
  return (
    <div className="flex items-center gap-3">
      <span className="text-xs text-slate-400 w-12 shrink-0">{day.slice(5)}</span>
      <div className="flex-1 h-6 bg-slate-100 rounded-full overflow-hidden">
        <div
          className="h-full rounded-full transition-all duration-700"
          style={{ width: pct + '%', background: 'linear-gradient(90deg, #6366f1, #8b5cf6)' }}
        />
      </div>
      <span className="text-xs font-medium text-slate-600 w-8 text-right">{calls}</span>
    </div>
  );
}

export default function Dashboard() {
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [days, setDays] = useState(7);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.analytics(days);
      setData(res);
    } catch (e) { console.error(e); }
    finally { setLoading(false); }
  }, [days]);

  useEffect(() => { load(); }, [load]);

  if (loading && !data) {
    return (
      <div className="p-8 animate-fade-in">
        <div className="flex items-center gap-3 text-slate-400">
          <span className="w-5 h-5 border-2 border-slate-300 border-t-indigo-500 rounded-full animate-spin inline-block" />
          加载统计数据...
        </div>
      </div>
    );
  }
  if (!data) {
    return (
      <div className="p-8 animate-fade-in text-center text-slate-400">
        <div className="text-3xl mb-2">📊</div>
        <p>暂无统计数据</p>
      </div>
    );
  }

  const s = data.summary;
  const maxCalls = Math.max(...(data.daily_trend || []).map((d: any) => d.calls), 1);

  return (
    <div className="p-8 animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-slate-800">数据分析</h1>
          <p className="text-slate-500 text-sm mt-1">网关调用统计与趋势分析</p>
        </div>
        <select
          value={days}
          onChange={e => setDays(Number(e.target.value))}
          className="px-4 py-2 rounded-xl border border-slate-200 bg-white text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 transition-all"
        >
          <option value={7}>最近 7 天</option>
          <option value={14}>最近 14 天</option>
          <option value={30}>最近 30 天</option>
        </select>
      </div>

      {/* Stat Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        <StatCard label="总调用量" value={s.total_calls} color="#6366f1" icon="📈" />
        <StatCard label="成功率" value={s.success_rate?.toFixed(1)} unit="%" color="#10b981" icon="✅" />
        <StatCard label="失败次数" value={s.failed_calls} color="#ef4444" icon="❌" />
        <StatCard label="平均延迟" value={s.avg_latency_ms} unit="ms" color="#f59e0b" icon="⚡" />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Top Tools */}
        <div className="bg-white rounded-xl border border-slate-200 p-6">
          <h3 className="font-semibold text-slate-700 mb-4 flex items-center gap-2">
            <span>🏆</span> Top 工具排行
          </h3>
          {(data.top_tools || []).length === 0 ? (
            <div className="text-center py-8 text-slate-400 text-sm">暂无工具数据</div>
          ) : (
            <div className="space-y-3">
              {(data.top_tools || []).slice(0, 5).map((t: any, i: number) => (
                <div key={t.tool_id} className="flex items-center gap-4">
                  <span className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold shrink-0 ${
                    i === 0 ? 'bg-amber-100 text-amber-600' : i === 1 ? 'bg-slate-100 text-slate-500' : i === 2 ? 'bg-orange-50 text-orange-500' : 'bg-slate-50 text-slate-400'
                  }`}>{i + 1}</span>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-medium text-slate-700 truncate">{t.tool_name}</div>
                    <div className="text-xs text-slate-400">{t.call_count} 次调用</div>
                  </div>
                  <div className="text-sm font-semibold text-indigo-600">
                    {t.success_rate?.toFixed(1)}%
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Daily Trend */}
        <div className="bg-white rounded-xl border border-slate-200 p-6">
          <h3 className="font-semibold text-slate-700 mb-4 flex items-center gap-2">
            <span>📊</span> 每日调用趋势
          </h3>
          {(data.daily_trend || []).length === 0 ? (
            <div className="text-center py-8 text-slate-400 text-sm">暂无趋势数据</div>
          ) : (
            <div className="space-y-2">
              {(data.daily_trend || []).map((d: any) => (
                <TrendBar key={d.date} day={d.date} calls={d.calls} max={maxCalls} />
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Success / Failed breakdown */}
      <div className="mt-6 bg-white rounded-xl border border-slate-200 p-6">
        <h3 className="font-semibold text-slate-700 mb-4 flex items-center gap-2">
          <span>📋</span> 调用详情
        </h3>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
          {[
            { label: '成功调用' , value: s.successful_calls, color: '#10b981' },
            { label: '失败调用' , value: s.failed_calls, color: '#ef4444' },
            { label: '被拒绝'   , value: s.rejected_calls, color: '#f59e0b' },
            { label: '总请求'   , value: s.total_calls, color: '#6366f1' },
          ].map(({ label, value, color }) => (
            <div key={label} className="text-center p-4 rounded-xl bg-slate-50">
              <div className="text-2xl font-bold text-slate-800">{value}</div>
              <div className="text-xs text-slate-500 mt-1">{label}</div>
              <div className="mt-2 h-1 rounded-full bg-slate-200 overflow-hidden">
                <div className="h-full rounded-full" style={{ width: '100%', background: color, opacity: 0.6 }} />
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}