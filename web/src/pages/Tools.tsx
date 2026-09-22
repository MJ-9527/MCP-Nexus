import { useState, useEffect, useCallback } from 'react';
import { api } from '../api';
import type { Tool } from '../types';

const CATEGORY_ICONS: Record<string, string> = {
  database: '🗄️', office: '📄', file: '📁', sales: '💰', default: '🔧'
};

function getCategoryIcon(cat: string) {
  const c = cat?.toLowerCase() || 'default';
  return CATEGORY_ICONS[c] || CATEGORY_ICONS['default'];
}

function HealthBadge({ status }: { status: string }) {
  const isOnline = status === 'online';
  return (
    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${
      isOnline
        ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
        : 'bg-red-50 text-red-700 border border-red-200'
    }`}>
      <span className={`w-1.5 h-1.5 rounded-full ${isOnline ? 'bg-emerald-500' : 'bg-red-500'}`} />
      {isOnline ? '在线' : '离线'}
    </span>
  );
}

function ToolCard({ tool }: { tool: Tool }) {
  const [expanded, setExpanded] = useState(false);
  return (
    <div className="bg-white rounded-xl border border-slate-200 hover:shadow-lg hover:border-indigo-200 transition-all duration-200 overflow-hidden group">
      <div className="p-5">
        <div className="flex items-start justify-between mb-3">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-indigo-50 flex items-center justify-center text-xl group-hover:scale-110 transition-transform">
              {getCategoryIcon(tool.category)}
            </div>
            <div>
              <h3 className="font-semibold text-slate-800 text-base">{tool.name}</h3>
              <span className="text-xs text-slate-400">v{tool.version}</span>
            </div>
          </div>
          <HealthBadge status={tool.health_status} />
        </div>
        <p className="text-slate-500 text-sm leading-relaxed line-clamp-2">{tool.description || '暂无描述'}</p>
        <div className="flex flex-wrap gap-1.5 mt-3">
          {tool.tags?.map(t => (
            <span key={t} className="px-2 py-0.5 rounded-md bg-slate-100 text-slate-500 text-xs">{t}</span>
          ))}
          <span className="px-2 py-0.5 rounded-md bg-indigo-50 text-indigo-600 text-xs font-medium">{tool.category}</span>
        </div>
        <div className="flex items-center gap-4 mt-4 pt-4 border-t border-slate-100 text-xs text-slate-400">
          <span>📊 {tool.call_count} 次调用</span>
          <button
            onClick={() => setExpanded(!expanded)}
            className="text-indigo-500 hover:text-indigo-700 font-medium ml-auto transition-colors"
          >
            {expanded ? '收起' : '查看详情'}
          </button>
        </div>
      </div>
      {expanded && (
        <div className="px-5 pb-5 border-t border-slate-100 pt-4 animate-fade-in">
          <div className="mb-3">
            <div className="text-xs font-medium text-slate-500 mb-1.5">输入参数 Schema</div>
            <pre className="bg-slate-50 rounded-lg p-3 text-xs text-slate-600 overflow-x-auto font-mono">
              {JSON.stringify(JSON.parse(tool.input_schema || '{}'), null, 2)}
            </pre>
          </div>
          <div className="text-xs text-slate-400">
            分类: {tool.category} · 状态: {tool.published ? '已发布' : '未发布'}
          </div>
        </div>
      )}
    </div>
  );
}

export default function Tools() {
  const [tools, setTools] = useState<Tool[]>([]);
  const [loading, setLoading] = useState(true);
  const [keyword, setKeyword] = useState('');
  const [category, setCategory] = useState('');
  const [sortBy, setSortBy] = useState('popularity');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const params: Record<string, string> = { sort: sortBy };
      if (keyword) params.keyword = keyword;
      if (category) params.category = category;
      const res = await api.tools(params);
      setTools(res.tools);
    } catch (e) { console.error(e); }
    finally { setLoading(false); }
  }, [keyword, category, sortBy]);

  useEffect(() => { load(); }, [load]);
  const categories = [...new Set(tools.map(t => t.category).filter(Boolean))];

  return (
    <div className="p-8 animate-fade-in">
      {/* Header */}
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-slate-800">工具市场</h1>
        <p className="text-slate-500 text-sm mt-1">浏览和发现可用的 MCP 工具</p>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-3 mb-6">
        <div className="relative flex-1 min-w-[240px]">
          <span className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400">🔍</span>
          <input
            value={keyword}
            onChange={e => setKeyword(e.target.value)}
            placeholder="搜索工具名称或描述..."
            className="w-full pl-10 pr-4 py-2.5 rounded-xl border border-slate-200 bg-white text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent transition-all"
          />
        </div>
        <select
          value={category}
          onChange={e => setCategory(e.target.value)}
          className="px-4 py-2.5 rounded-xl border border-slate-200 bg-white text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 transition-all"
        >
          <option value="">所有分类</option>
          {categories.map(c => <option key={c} value={c}>{c}</option>)}
        </select>
        <select
          value={sortBy}
          onChange={e => setSortBy(e.target.value)}
          className="px-4 py-2.5 rounded-xl border border-slate-200 bg-white text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 transition-all"
        >
          <option value="popularity">按热度排序</option>
          <option value="name">按名称排序</option>
          <option value="rating">按评分排序</option>
        </select>
      </div>

      {/* Results count */}
      <div className="text-sm text-slate-500 mb-4">
        共 {tools.length} 个工具
        {keyword && <span>，筛选 {keyword}</span>}
        {category && <span>，分类: {category}</span>}
      </div>

      {/* Grid */}
      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="bg-white rounded-xl border border-slate-200 p-5 loading-shimmer" style={{ minHeight: 180 }} />
          ))}
        </div>
      ) : tools.length === 0 ? (
        <div className="text-center py-20 text-slate-400">
          <div className="text-4xl mb-3">🔍</div>
          <p className="text-base font-medium">没有找到工具</p>
          <p className="text-sm mt-1">尝试调整搜索条件</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
          {tools.map(tool => <ToolCard key={tool.id} tool={tool} />)}
        </div>
      )}
    </div>
  );
}