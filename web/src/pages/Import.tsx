import { useEffect, useState } from 'react';
import { api } from '../api';
import type { ServerInfo } from '../types';

export default function Import() {
  const [activeTab, setActiveTab] = useState<'openapi' | 'skills'>('openapi');
  const [servers, setServers] = useState<ServerInfo[]>([]);
  const [serverId, setServerId] = useState('');
  const [payload, setPayload] = useState('');
  const [published, setPublished] = useState(true);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ success: boolean; message: string } | null>(null);

  useEffect(() => {
    api.servers()
      .then(data => {
        const items = Array.isArray(data) ? data : data.items || [];
        setServers(items);
        if (items.length > 0) setServerId(String(items[0].id));
      })
      .catch(e => setResult({ success: false, message: `服务列表加载失败：${e.message}` }));
  }, []);

  const handleImport = async () => {
    setResult(null);
    const id = Number(serverId);
    if (!id) {
      setResult({ success: false, message: '请先选择目标服务' });
      return;
    }
    try {
      setLoading(true);
      const parsed = JSON.parse(payload);
      const response: any = activeTab === 'openapi'
        ? await api.importOpenAPI(id, parsed, published)
        : await api.importSkills(id, Array.isArray(parsed) ? parsed : parsed.tools, published);
      setResult({ success: true, message: `导入成功，共处理 ${response?.total ?? 0} 个工具。请前往工具市场查看。` });
      setPayload('');
    } catch (e: any) {
      setResult({ success: false, message: `导入失败：${e.message}` });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="p-8 animate-fade-in">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-slate-800">工具导入</h1>
        <p className="text-slate-500 text-sm mt-1">将规范导入已有 MCP Server，并自动生成工具</p>
      </div>

      <div className="flex gap-2 mb-6 border-b border-slate-200">
        <button onClick={() => { setActiveTab('openapi'); setResult(null); }} className={`px-4 py-2 text-sm font-medium ${activeTab === 'openapi' ? 'text-indigo-600 border-b-2 border-indigo-600' : 'text-slate-500'}`}>📄 OpenAPI 导入</button>
        <button onClick={() => { setActiveTab('skills'); setResult(null); }} className={`px-4 py-2 text-sm font-medium ${activeTab === 'skills' ? 'text-indigo-600 border-b-2 border-indigo-600' : 'text-slate-500'}`}>🛠️ Skills 导入</button>
      </div>

      {result && <div className={`mb-6 p-4 rounded-lg border ${result.success ? 'bg-emerald-50 text-emerald-700 border-emerald-200' : 'bg-red-50 text-red-700 border-red-200'}`}>{result.message}</div>}

      <div className="bg-white rounded-xl border border-slate-200 p-6 space-y-5">
        <div>
          <label className="block text-sm font-medium text-slate-700 mb-1">目标 MCP Server</label>
          <select value={serverId} onChange={e => setServerId(e.target.value)} className="w-full px-3 py-2 border border-slate-200 rounded-lg">
            {servers.length === 0 && <option value="">暂无服务，请先注册服务</option>}
            {servers.map(server => <option key={server.id} value={server.id}>{server.name}（{server.endpoint}）</option>)}
          </select>
        </div>

        <div>
          <label className="block text-sm font-medium text-slate-700 mb-1">{activeTab === 'openapi' ? 'OpenAPI JSON 规范' : 'Skills 工具 JSON'}</label>
          <textarea value={payload} onChange={e => setPayload(e.target.value)} placeholder={activeTab === 'openapi' ? '粘贴完整的 OpenAPI JSON，例如 {"openapi":"3.0.0",...}' : '粘贴工具数组，或 {"tools":[...]}'} rows={16} className="w-full px-3 py-2 border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 font-mono text-sm" />
        </div>

        <label className="flex items-center gap-2 text-sm text-slate-700">
          <input type="checkbox" checked={published} onChange={e => setPublished(e.target.checked)} />
          导入后立即发布（发布后仍需具备调用权限；平台管理员可直接调用）
        </label>

        <button onClick={handleImport} disabled={loading || !serverId || !payload.trim()} className="w-full py-2.5 bg-indigo-500 text-white rounded-lg hover:bg-indigo-600 disabled:opacity-50">
          {loading ? '导入中...' : `导入 ${activeTab === 'openapi' ? 'OpenAPI' : 'Skills'}`}
        </button>
      </div>
    </div>
  );
}
