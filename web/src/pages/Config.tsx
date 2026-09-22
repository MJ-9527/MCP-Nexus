import { useState } from 'react';

const MCP_CONFIG = `{
  "mcpServers": {
    "mcp-nexus": {
      "command": "npx",
      "args": ["@modelcontextprotocol/server-example"],
      "env": {
        "MCP_GATEWAY": "http://localhost:18080",
        "MCP_GATEWAY_TOKEN": "<YOUR_JWT_TOKEN>"
      }
    }
  }
}`;

const OPENAPI_IMPORT = `# 导入 OpenAPI 规范
# 1. 在工具市场页面点击"导入"按钮
# 2. 上传 .yaml 或 .json 格式的 OpenAPI 文件
# 3. 系统将自动生成对应的 MCP 工具
#
# 支持的字段:
# - paths: API 路径
# - parameters: 请求参数
# - responses: 响应格式
# - security: 鉴权方式 (apiKey / bearer)
`;

export default function Config() {
  const [copied, setCopied] = useState('');
  const [tab, setTab] = useState<'mcp' | 'openapi' >('mcp');

  function copy(text: string, label: string) {
    navigator.clipboard.writeText(text);
    setCopied(label);
    setTimeout(() => setCopied(''), 2000);
  }

  return (
    <div className="p-8 animate-fade-in max-w-4xl">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-slate-800">接入配置</h1>
        <p className="text-slate-500 text-sm mt-1">配置 MCP 客户端和导入外部 API</p>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 bg-slate-100 rounded-xl p-1 mb-6 w-fit">
        {[
          { key: 'mcp' as const, label: 'MCP 配置' },
          { key: 'openapi' as const, label: 'OpenAPI 导入' },
        ].map(({ key, label }) => (
          <button
            key={key}
            onClick={() => setTab(key)}
            className={`px-5 py-2 rounded-lg text-sm font-medium transition-all ${
              tab === key
                ? 'bg-white text-indigo-600 shadow-sm'
                : 'text-slate-500 hover:text-slate-700'
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {tab === 'mcp' && (
        <div className="space-y-6">
          {/* MCP Config Card */}
          <div className="bg-white rounded-xl border border-slate-200 overflow-hidden">
            <div className="px-6 py-4 border-b border-slate-100 flex items-center justify-between">
              <div>
                <h3 className="font-semibold text-slate-700">MCP Server 配置</h3>
                <p className="text-sm text-slate-400 mt-0.5">复制以下配置到 Claude Desktop 或 Cursor 的 settings.json</p>
              </div>
              <button
                onClick={() => copy(MCP_CONFIG, 'config')}
                className={`px-4 py-2 rounded-lg text-sm font-medium transition-all ${
                  copied === 'config'
                    ? 'bg-emerald-100 text-emerald-700'
                    : 'bg-indigo-600 text-white hover:bg-indigo-700'
                }`}
              >
                {copied === 'config' ? '✓ 已复制' : '复制配置'}
              </button>
            </div>
            <div className="p-6 bg-slate-900 overflow-x-auto">
              <pre className="text-sm text-emerald-400 font-mono whitespace-pre">{MCP_CONFIG.replace('<YOUR_JWT_TOKEN>', '(登录后复制)' )}</pre>
            </div>
          </div>

          {/* Gateway Info */}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            {[
              { label: '网关地址' , value: 'http://localhost:18080' , icon: '🌐' },
              { label: '认证方式' , value: 'Bearer Token' , icon: '🔐' },
              { label: '协议支持' , value: 'HTTP / JSON-RPC' , icon: '📡' },
            ].map(({ label, value, icon }) => (
              <div key={label} className="bg-white rounded-xl border border-slate-200 p-5">
                <div className="text-2xl mb-2">{icon}</div>
                <div className="text-sm text-slate-500">{label}</div>
                <div className="font-semibold text-slate-800 mt-1">{value}</div>
              </div>
            ))}
          </div>

          {/* How to use */}
          <div className="bg-white rounded-xl border border-slate-200 p-6">
            <h3 className="font-semibold text-slate-700 mb-4">使用步骤</h3>
            <ol className="space-y-3">
              {[
                '在上方获取 JWT Token（登录后复制）' ,
                '将 MCP 配置粘贴到 Claude Desktop → Settings → Connectors' ,
                '重启 Claude Desktop，即可在对话框中使用工具' ,
                '在 Cursor 中: Settings → Features → MCP → 添加 server' ,
              ].map((step, i) => (
                <li key={i} className="flex items-start gap-3">
                  <span className="w-6 h-6 rounded-full bg-indigo-100 text-indigo-600 text-xs font-bold flex items-center justify-center shrink-0">{i + 1}</span>
                  <span className="text-sm text-slate-600">{step}</span>
                </li>
              ))}
            </ol>
          </div>
        </div>
      )}

      {tab === 'openapi' && (
        <div className="bg-white rounded-xl border border-slate-200 overflow-hidden">
          <div className="px-6 py-4 border-b border-slate-100 flex items-center justify-between">
            <div>
              <h3 className="font-semibold text-slate-700">OpenAPI 导入指南</h3>
              <p className="text-sm text-slate-400 mt-0.5">将 OpenAPI 规范转换为 MCP 工具</p>
            </div>
          </div>
          <div className="p-6 bg-slate-900 overflow-x-auto">
            <pre className="text-sm text-blue-300 font-mono whitespace-pre">{OPENAPI_IMPORT}</pre>
          </div>
          <div className="p-6">
            <div className="flex items-center gap-3 p-4 bg-amber-50 border border-amber-200 rounded-xl">
              <span className="text-xl">⚠️</span>
              <p className="text-sm text-amber-700">OpenAPI 导入功能正在开发中，敬请期待。</p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}