import { useState, useRef, useEffect } from 'react'
import { api, uploadFile } from '../api/client'
import { useApp } from '../store/AppContext'
import './BotsPanel.css'

const AI_PROVIDERS = [
  { label: 'OpenAI', value: 'openai', baseUrl: 'https://api.openai.com/v1' },
  { label: '智谱 AI', value: 'zhipu', baseUrl: 'https://open.bigmodel.cn/api/paas/v4' },
  { label: '豆包 (字节)', value: 'doubao', baseUrl: 'https://ark.cn-beijing.volces.com/api/v3' },
  { label: 'DeepSeek', value: 'deepseek', baseUrl: 'https://api.deepseek.com/v1' },
  { label: '月之暗面 (Kimi)', value: 'moonshot', baseUrl: 'https://api.moonshot.cn/v1' },
  { label: '自定义', value: 'custom', baseUrl: '' },
]

const MCP_TRANSPORTS = [
  { label: 'SSE', value: 'sse' },
  { label: 'HTTP', value: 'http' },
]

const MCP_AUTH_TYPES = [
  { label: '无认证', value: 'none' },
  { label: 'Bearer Token', value: 'bearer' },
  { label: 'Basic Auth', value: 'basic' },
]

interface BotInfo {
  bot_id: string
  creator_id: string
  name: string
  avatar_url: string
  system_prompt: string
  model: string
  base_url?: string
  is_system: boolean
  created_at: string
}

interface McpServerInfo {
  id: string
  bot_id: string
  name: string
  description: string
  transport: string
  url: string
  auth_type: string
  enabled: boolean
  created_at: string
}

interface KnowledgeBaseInfo {
  kb_id: string
  owner_id: string
  name: string
  description: string
  doc_count: number
  created_at: string
}

interface DocumentInfo {
  doc_id: string
  kb_id: string
  file_name: string
  file_type: string
  file_size: number
  status: string
  chunk_count: number
  created_at: string
}

export default function BotsPanel({ onSwitchToChat }: { onSwitchToChat: () => void }) {
  const { conversations, setActiveConvId, loadConversations } = useApp()
  const [tab, setTab] = useState<'create' | 'list' | 'mcp' | 'knowledge'>('list')
  const [bots, setBots] = useState<BotInfo[]>([])
  const [toast, setToast] = useState('')
  const [addBotModal, setAddBotModal] = useState<{ botId: string; botName: string } | null>(null)
  const [selectedConvId, setSelectedConvId] = useState('')
  const [editBot, setEditBot] = useState<BotInfo | null>(null)

  const [name, setName] = useState('')
  const [systemPrompt, setSystemPrompt] = useState('')
  const [provider, setProvider] = useState('openai')
  const [baseUrl, setBaseUrl] = useState(AI_PROVIDERS[0].baseUrl)
  const [apiKey, setApiKey] = useState('')
  const [model, setModel] = useState('')
  const [avatarUrl, setAvatarUrl] = useState('')
  const avatarInputRef = useRef<HTMLInputElement>(null)

  const [mcpBotId, setMcpBotId] = useState('')
  const [mcpServers, setMcpServers] = useState<McpServerInfo[]>([])
  const [mcpModal, setMcpModal] = useState<'create' | 'edit' | null>(null)
  const [mcpForm, setMcpForm] = useState({ id: '', name: '', url: '', description: '', transport: 'sse', auth_type: 'none', auth_token: '' })

  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBaseInfo[]>([])
  const [kbModal, setKbModal] = useState<'create' | 'edit' | null>(null)
  const [kbForm, setKbForm] = useState({ kb_id: '', name: '', description: '' })
  const [selectedKbId, setSelectedKbId] = useState('')
  const [documents, setDocuments] = useState<DocumentInfo[]>([])
  const [docFile, setDocFile] = useState<File | null>(null)
  const docInputRef = useRef<HTMLInputElement>(null)
  const [bindKbBotId, setBindKbBotId] = useState('')
  const [botKnowledgeBases, setBotKnowledgeBases] = useState<KnowledgeBaseInfo[]>([])
  const [bindModal, setBindModal] = useState(false)

  const showToast = (msg: string) => {
    setToast(msg)
    setTimeout(() => setToast(''), 3000)
  }

  const handleProviderChange = (val: string) => {
    setProvider(val)
    const found = AI_PROVIDERS.find(p => p.value === val)
    if (found && found.baseUrl) {
      setBaseUrl(found.baseUrl)
    }
  }

  const handleCreate = async () => {
    if (!name || !systemPrompt || !model) {
      showToast('请填写必填项')
      return
    }
    const res = await api('POST', '/api/bot/create', {
      name,
      system_prompt: systemPrompt,
      api_key: apiKey,
      model,
      base_url: baseUrl,
    })
    if (res.code === 0) {
      showToast('Bot 创建成功')
      setName('')
      setSystemPrompt('')
      setApiKey('')
      setModel('')
      setProvider('openai')
      setBaseUrl(AI_PROVIDERS[0].baseUrl)
      loadBots()
    } else {
      showToast(res.msg || '创建失败')
    }
  }

  const loadBots = async () => {
    const res = await api('GET', '/api/bot/list')
    if (res.code === 0) {
      setBots(res.data?.bots || [])
    } else {
      showToast(res.msg || '获取失败')
    }
  }

  const handleDelete = async (botId: string) => {
    const res = await api('POST', '/api/bot/delete', { bot_id: botId })
    showToast(res.code === 0 ? '已删除' : res.msg || '删除失败')
    if (res.code === 0) loadBots()
  }

  const handleStartBotChat = async (botId: string) => {
    const addRes = await api('POST', '/api/bot/add_to_conversation', {
      bot_id: botId,
      conversation_id: "0",
      conversation_type: 1,
    })
    if (addRes.code === 0) {
      const convId = String(addRes.data?.conversation_id || '')
      if (convId && convId !== '0') {
        await loadConversations()
        setActiveConvId(convId)
        onSwitchToChat()
      } else {
        await loadConversations()
        showToast('会话已存在，请在会话列表中查找')
      }
    } else {
      showToast(addRes.msg || '创建会话失败')
    }
  }

  const handleAddBotToGroup = async () => {
    if (!addBotModal || !selectedConvId) return
    const res = await api('POST', '/api/bot/add_to_conversation', {
      bot_id: addBotModal.botId,
      conversation_id: selectedConvId,
      conversation_type: 2,
    })
    if (res.code === 0) {
      showToast('已将 Bot 拉入群聊')
      setAddBotModal(null)
      setSelectedConvId('')
      await loadConversations()
    } else {
      showToast(res.msg || '拉入群聊失败')
    }
  }

  const openEditModal = (bot: BotInfo) => {
    setEditBot(bot)
    setName(bot.name)
    setAvatarUrl(bot.avatar_url || '')
    setSystemPrompt(bot.system_prompt)
    setModel(bot.model)
    setApiKey('')
    setBaseUrl(bot.base_url || '')
    const matched = AI_PROVIDERS.find(p => p.baseUrl && bot.base_url?.startsWith(p.baseUrl.replace('/v1', '')))
    setProvider(matched ? matched.value : 'custom')
  }

  const handleUpdate = async () => {
    if (!editBot) return
    const payload: Record<string, string> = {
      bot_id: editBot.bot_id,
    }
    if (name && name !== editBot.name) payload.name = name
    if (systemPrompt && systemPrompt !== editBot.system_prompt) payload.system_prompt = systemPrompt
    if (model && model !== editBot.model) payload.model = model
    if (apiKey) payload.api_key = apiKey
    if (baseUrl && baseUrl !== (editBot.base_url || '')) payload.base_url = baseUrl
    if (avatarUrl !== (editBot.avatar_url || '')) payload.avatar_url = avatarUrl

    const res = await api('POST', '/api/bot/update', payload)
    if (res.code === 0) {
      showToast('Bot 已更新')
      setEditBot(null)
      loadBots()
      loadConversations()
    } else {
      showToast(res.msg || '更新失败')
    }
  }

  const handleAvatarUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    try {
      const res = await uploadFile(file)
      if (res.code !== 0 || !res.data?.url) {
        showToast(res.msg || '上传失败')
        return
      }
      setAvatarUrl(res.data.url)
    } catch {
      showToast('上传头像失败')
    }
    if (avatarInputRef.current) avatarInputRef.current.value = ''
  }

  const loadMcpServers = async (botId: string) => {
    if (!botId) return
    const res = await api('GET', `/api/bot/mcp/list?bot_id=${botId}`)
    if (res.code === 0) {
      setMcpServers(res.data?.servers || [])
    } else {
      showToast(res.msg || '获取 MCP Server 列表失败')
    }
  }

  useEffect(() => {
    if (tab === 'list') loadBots()
    if (tab === 'knowledge') loadKnowledgeBases()
  }, [tab])

  useEffect(() => {
    if (tab === 'mcp' && mcpBotId) {
      loadMcpServers(mcpBotId)
    } else if (tab === 'mcp' && !mcpBotId && bots.length === 0) {
      loadBots()
    }
  }, [tab, mcpBotId])

  const handleMcpCreate = async () => {
    if (!mcpBotId || !mcpForm.name || !mcpForm.url) {
      showToast('请填写必填项')
      return
    }
    const res = await api('POST', '/api/bot/mcp/create', {
      bot_id: mcpBotId,
      name: mcpForm.name,
      url: mcpForm.url,
      description: mcpForm.description,
      transport: mcpForm.transport,
      auth_type: mcpForm.auth_type,
      auth_token: mcpForm.auth_token,
    })
    if (res.code === 0) {
      showToast('MCP Server 添加成功')
      setMcpModal(null)
      setMcpForm({ id: '', name: '', url: '', description: '', transport: 'sse', auth_type: 'none', auth_token: '' })
      loadMcpServers(mcpBotId)
    } else {
      showToast(res.msg || '添加失败')
    }
  }

  const handleMcpUpdate = async () => {
    const payload: Record<string, any> = { id: mcpForm.id }
    if (mcpForm.name) payload.name = mcpForm.name
    if (mcpForm.url) payload.url = mcpForm.url
    if (mcpForm.description) payload.description = mcpForm.description
    if (mcpForm.transport) payload.transport = mcpForm.transport
    if (mcpForm.auth_type) payload.auth_type = mcpForm.auth_type
    if (mcpForm.auth_token) payload.auth_token = mcpForm.auth_token
    const res = await api('POST', '/api/bot/mcp/update', payload)
    if (res.code === 0) {
      showToast('MCP Server 已更新')
      setMcpModal(null)
      setMcpForm({ id: '', name: '', url: '', description: '', transport: 'sse', auth_type: 'none', auth_token: '' })
      loadMcpServers(mcpBotId)
    } else {
      showToast(res.msg || '更新失败')
    }
  }

  const handleMcpDelete = async (id: string) => {
    const res = await api('POST', '/api/bot/mcp/delete', { id })
    showToast(res.code === 0 ? 'MCP Server 已删除' : res.msg || '删除失败')
    if (res.code === 0) loadMcpServers(mcpBotId)
  }

  const handleMcpToggle = async (server: McpServerInfo) => {
    const res = await api('POST', '/api/bot/mcp/update', {
      id: server.id,
      enabled: !server.enabled,
    })
    if (res.code === 0) {
      loadMcpServers(mcpBotId)
    } else {
      showToast(res.msg || '切换状态失败')
    }
  }

  const openMcpEditModal = (server: McpServerInfo) => {
    setMcpForm({
      id: server.id,
      name: server.name,
      url: server.url,
      description: server.description,
      transport: server.transport,
      auth_type: server.auth_type,
      auth_token: '',
    })
    setMcpModal('edit')
  }

  const loadKnowledgeBases = async () => {
    const res = await api('GET', '/api/knowledge/list')
    if (res.code === 0) {
      setKnowledgeBases(res.data?.knowledge_bases || [])
    } else {
      showToast(res.msg || '获取知识库列表失败')
    }
  }

  const handleKbCreate = async () => {
    if (!kbForm.name) {
      showToast('请填写知识库名称')
      return
    }
    const res = await api('POST', '/api/knowledge/create', {
      name: kbForm.name,
      description: kbForm.description,
    })
    if (res.code === 0) {
      showToast('知识库创建成功')
      setKbModal(null)
      setKbForm({ kb_id: '', name: '', description: '' })
      loadKnowledgeBases()
    } else {
      showToast(res.msg || '创建失败')
    }
  }

  const handleKbUpdate = async () => {
    const res = await api('POST', '/api/knowledge/update', {
      kb_id: kbForm.kb_id,
      name: kbForm.name,
      description: kbForm.description,
    })
    if (res.code === 0) {
      showToast('知识库已更新')
      setKbModal(null)
      setKbForm({ kb_id: '', name: '', description: '' })
      loadKnowledgeBases()
    } else {
      showToast(res.msg || '更新失败')
    }
  }

  const handleKbDelete = async (kbId: string) => {
    const res = await api('POST', '/api/knowledge/delete', { kb_id: kbId })
    showToast(res.code === 0 ? '知识库已删除' : res.msg || '删除失败')
    if (res.code === 0) {
      if (selectedKbId === kbId) {
        setSelectedKbId('')
        setDocuments([])
      }
      loadKnowledgeBases()
    }
  }

  const loadDocuments = async (kbId: string) => {
    const res = await api('GET', `/api/knowledge/document/list?kb_id=${kbId}`)
    if (res.code === 0) {
      setDocuments(res.data?.documents || [])
    } else {
      showToast(res.msg || '获取文档列表失败')
    }
  }

  const handleDocUpload = async () => {
    if (!selectedKbId || !docFile) {
      showToast('请选择要上传的文件')
      return
    }
    const uploadRes = await uploadFile(docFile)
    if (uploadRes.code !== 0 || !uploadRes.data?.url) {
      showToast(uploadRes.msg || '文件上传失败')
      return
    }
    const ext = docFile.name.split('.').pop()?.toLowerCase() || ''
    const fileType = ext === 'pdf' ? 'pdf' : ext === 'md' ? 'md' : ext === 'docx' || ext === 'doc' ? 'docx' : ext === 'pptx' || ext === 'ppt' ? 'pptx' : 'txt'
    const res = await api('POST', '/api/knowledge/document/add', {
      kb_id: selectedKbId,
      file_name: docFile.name,
      file_url: uploadRes.data.url,
      file_type: fileType,
      file_size: docFile.size,
    })
    if (res.code === 0) {
      showToast('文档已提交，正在解析中...')
      setDocFile(null)
      if (docInputRef.current) docInputRef.current.value = ''
      loadDocuments(selectedKbId)
    } else {
      showToast(res.msg || '添加文档失败')
    }
  }

  const handleDocDelete = async (docId: string) => {
    const res = await api('POST', '/api/knowledge/document/delete', { doc_id: docId })
    showToast(res.code === 0 ? '文档已删除' : res.msg || '删除失败')
    if (res.code === 0) loadDocuments(selectedKbId)
  }

  const handleDocRetry = async (docId: string) => {
    const res = await api('POST', '/api/knowledge/document/retry', { doc_id: docId })
    showToast(res.code === 0 ? '已重新提交解析' : res.msg || '重试失败')
    if (res.code === 0) loadDocuments(selectedKbId)
  }

  const loadBotKnowledgeBases = async (botId: string) => {
    const res = await api('GET', `/api/knowledge/bot_bases?bot_id=${botId}`)
    if (res.code === 0) {
      setBotKnowledgeBases(res.data?.knowledge_bases || [])
    }
  }

  const handleBindKb = async (kbId: string) => {
    if (!bindKbBotId) return
    const res = await api('POST', '/api/knowledge/bind', { bot_id: bindKbBotId, kb_id: kbId })
    if (res.code === 0) {
      showToast('已绑定知识库')
      loadBotKnowledgeBases(bindKbBotId)
    } else {
      showToast(res.msg || '绑定失败')
    }
  }

  const handleUnbindKb = async (kbId: string) => {
    if (!bindKbBotId) return
    const res = await api('POST', '/api/knowledge/unbind', { bot_id: bindKbBotId, kb_id: kbId })
    if (res.code === 0) {
      showToast('已解绑知识库')
      loadBotKnowledgeBases(bindKbBotId)
    } else {
      showToast(res.msg || '解绑失败')
    }
  }

  const groupConversations = conversations.filter(c => c.type === 2)

  return (
    <div className="bots-panel">
      {toast && <div className="bots-toast">{toast}</div>}
      <div className="bots-header">
        <h3>AI 助手</h3>
      </div>
      <div className="bots-tabs">
        <button className={`ctab ${tab === 'list' ? 'active' : ''}`} onClick={() => setTab('list')}>我的 Bot</button>
        <button className={`ctab ${tab === 'create' ? 'active' : ''}`} onClick={() => setTab('create')}>创建 Bot</button>
        <button className={`ctab ${tab === 'mcp' ? 'active' : ''}`} onClick={() => setTab('mcp')}>MCP 工具</button>
        <button className={`ctab ${tab === 'knowledge' ? 'active' : ''}`} onClick={() => setTab('knowledge')}>知识库</button>
      </div>
      <div className="bots-content">
        {tab === 'create' && (
          <div className="add-form">
            <div className="form-field">
              <label>Bot 名称 *</label>
              <input value={name} onChange={e => setName(e.target.value)} placeholder="给你的 AI 起个名字" />
            </div>
            <div className="form-field">
              <label>系统提示词 *</label>
              <textarea value={systemPrompt} onChange={e => setSystemPrompt(e.target.value)} placeholder="定义 AI 的角色和行为" rows={4} style={{ width: '100%', resize: 'vertical', padding: 8, borderRadius: 6, border: '1px solid var(--border)' }} />
            </div>
            <div className="form-field">
              <label>AI 服务商</label>
              <select value={provider} onChange={e => handleProviderChange(e.target.value)} style={{ width: '100%', padding: 8, borderRadius: 6, border: '1px solid var(--border)' }}>
                {AI_PROVIDERS.map(p => (
                  <option key={p.value} value={p.value}>{p.label}</option>
                ))}
              </select>
            </div>
            <div className="form-field">
              <label>API Base URL {provider === 'custom' && '*'}</label>
              <input value={baseUrl} onChange={e => setBaseUrl(e.target.value)} placeholder="https://api.openai.com/v1" />
              {provider !== 'custom' && <div style={{ fontSize: 12, color: 'var(--text-secondary)', marginTop: 2 }}>选择服务商后自动填充</div>}
            </div>
            <div className="form-field">
              <label>API Key</label>
              <input type="password" value={apiKey} onChange={e => setApiKey(e.target.value)} placeholder="sk-..." />
            </div>
            <div className="form-field">
              <label>模型名称 *</label>
              <input value={model} onChange={e => setModel(e.target.value)} placeholder="如 gpt-4o-mini, glm-4-flash" />
            </div>
            <button className="btn-primary full" onClick={handleCreate}>创建 Bot</button>
          </div>
        )}

        {tab === 'list' && (
          <div className="bot-list">
            {bots.length === 0 ? (
              <div className="contacts-empty">暂无 Bot，点击"创建 Bot"开始</div>
            ) : (
              bots.map(b => (
                <div key={b.bot_id} className="friend-item">
                  <div className="friend-main" style={{ flex: 1, display: 'flex', alignItems: 'center', gap: 10 }}>
                    <div className="friend-avatar">🤖</div>
                    <div className="friend-info">
                      <div className="friend-name">
                        {b.name}
                        {b.is_system && <span style={{ fontSize: 11, background: 'var(--primary)', color: '#fff', borderRadius: 4, padding: '1px 6px', marginLeft: 6, verticalAlign: 'middle' }}>系统</span>}
                      </div>
                      <div className="friend-sub">{b.model}</div>
                    </div>
                  </div>
                  <button className="btn-icon" title="开始私聊" onClick={() => handleStartBotChat(b.bot_id)} style={{ flexShrink: 0 }}>💬</button>
                  {groupConversations.length > 0 && (
                    <button className="btn-icon" title="拉入群聊" onClick={() => { setAddBotModal({ botId: b.bot_id, botName: b.name }); setSelectedConvId('') }} style={{ flexShrink: 0 }}>👥</button>
                  )}
                  <button className="btn-icon" title="MCP 工具管理" onClick={() => { setMcpBotId(b.bot_id); setTab('mcp') }} style={{ flexShrink: 0 }}>🔧</button>
                  {!b.is_system && (
                    <>
                      <button className="btn-icon" title="编辑" onClick={() => openEditModal(b)} style={{ flexShrink: 0 }}>✎</button>
                      <button className="btn-icon" title="删除" onClick={() => handleDelete(b.bot_id)} style={{ flexShrink: 0, color: 'var(--danger)' }}>✕</button>
                    </>
                  )}
                </div>
              ))
            )}
          </div>
        )}

        {tab === 'mcp' && (
          <div className="mcp-tab">
            <div className="form-field">
              <label>选择 Bot</label>
              <select value={mcpBotId} onChange={e => setMcpBotId(e.target.value)} style={{ width: '100%', padding: 8, borderRadius: 6, border: '1px solid var(--border)' }}>
                <option value="">-- 请选择 Bot --</option>
                {bots.map(b => (
                  <option key={b.bot_id} value={b.bot_id}>{b.name}</option>
                ))}
              </select>
            </div>

            {mcpBotId && (
              <>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', margin: '12px 0 8px' }}>
                  <span style={{ fontWeight: 600, fontSize: 14 }}>MCP Server 列表</span>
                  <button className="btn-primary" style={{ fontSize: 12, padding: '4px 12px' }} onClick={() => {
                    setMcpForm({ id: '', name: '', url: '', description: '', transport: 'sse', auth_type: 'none', auth_token: '' })
                    setMcpModal('create')
                  }}>+ 添加</button>
                </div>

                {mcpServers.length === 0 ? (
                  <div className="contacts-empty" style={{ padding: '20px 0' }}>暂无 MCP Server，点击"添加"开始配置</div>
                ) : (
                  <div className="mcp-server-list">
                    {mcpServers.map(s => (
                      <div key={s.id} className="mcp-server-item">
                        <div className="mcp-server-main">
                          <div className="mcp-server-info">
                            <div className="mcp-server-name">
                              {s.name}
                              <span className={`mcp-badge ${s.transport}`}>{s.transport.toUpperCase()}</span>
                              <span className={`mcp-badge ${s.enabled ? 'enabled' : 'disabled'}`}>{s.enabled ? '已启用' : '已禁用'}</span>
                            </div>
                            <div className="mcp-server-url">{s.url}</div>
                            {s.description && <div className="mcp-server-desc">{s.description}</div>}
                          </div>
                        </div>
                        <div className="mcp-server-actions">
                          <button className="btn-icon" title={s.enabled ? '禁用' : '启用'} onClick={() => handleMcpToggle(s)} style={{ flexShrink: 0 }}>
                            {s.enabled ? '⏸' : '▶'}
                          </button>
                          <button className="btn-icon" title="编辑" onClick={() => openMcpEditModal(s)} style={{ flexShrink: 0 }}>✎</button>
                          <button className="btn-icon" title="删除" onClick={() => handleMcpDelete(s.id)} style={{ flexShrink: 0, color: 'var(--danger)' }}>✕</button>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </>
            )}
          </div>
        )}

        {tab === 'knowledge' && (
          <div className="knowledge-tab">
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', margin: '0 0 12px' }}>
              <span style={{ fontWeight: 600, fontSize: 14 }}>我的知识库</span>
              <button className="btn-primary" style={{ fontSize: 12, padding: '4px 12px' }} onClick={() => {
                setKbForm({ kb_id: '', name: '', description: '' })
                setKbModal('create')
              }}>+ 创建知识库</button>
            </div>

            {knowledgeBases.length === 0 ? (
              <div className="contacts-empty" style={{ padding: '20px 0' }}>暂无知识库，点击"创建知识库"开始</div>
            ) : (
              <div className="mcp-server-list">
                {knowledgeBases.map(kb => (
                  <div key={kb.kb_id} className="mcp-server-item" style={{ cursor: 'pointer' }} onClick={() => { setSelectedKbId(kb.kb_id); loadDocuments(kb.kb_id) }}>
                    <div className="mcp-server-main">
                      <div className="mcp-server-info">
                        <div className="mcp-server-name">
                          📚 {kb.name}
                          <span className="mcp-badge enabled">{kb.doc_count} 文档</span>
                        </div>
                        {kb.description && <div className="mcp-server-desc">{kb.description}</div>}
                      </div>
                    </div>
                    <div className="mcp-server-actions" onClick={e => e.stopPropagation()}>
                      <button className="btn-icon" title="绑定到Bot" onClick={() => { setBindKbBotId(''); setBotKnowledgeBases([]); setBindModal(true); setSelectedKbId(kb.kb_id) }} style={{ flexShrink: 0 }}>🤖</button>
                      <button className="btn-icon" title="编辑" onClick={() => { setKbForm({ kb_id: kb.kb_id, name: kb.name, description: kb.description }); setKbModal('edit') }} style={{ flexShrink: 0 }}>✎</button>
                      <button className="btn-icon" title="删除" onClick={() => handleKbDelete(kb.kb_id)} style={{ flexShrink: 0, color: 'var(--danger)' }}>✕</button>
                    </div>
                  </div>
                ))}
              </div>
            )}

            {selectedKbId && (
              <div style={{ marginTop: 16 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', margin: '0 0 8px' }}>
                  <span style={{ fontWeight: 600, fontSize: 14 }}>文档管理</span>
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <input ref={docInputRef} type="file" accept=".pdf,.md,.docx,.doc,.pptx,.ppt,.txt" style={{ display: 'none' }} onChange={e => { setDocFile(e.target.files?.[0] || null) }} />
                    <button className="btn-cancel" style={{ fontSize: 12, padding: '4px 12px' }} onClick={() => docInputRef.current?.click()}>选择文件</button>
                    {docFile && (
                      <>
                        <span style={{ fontSize: 12, color: 'var(--text-secondary)', maxWidth: 120, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{docFile.name}</span>
                        <button className="btn-primary" style={{ fontSize: 12, padding: '4px 12px' }} onClick={handleDocUpload}>上传</button>
                      </>
                    )}
                  </div>
                </div>
                <div style={{ fontSize: 11, color: 'var(--text-secondary)', marginBottom: 8 }}>支持 PDF、Markdown、Word、PowerPoint 格式</div>
                {documents.length === 0 ? (
                  <div className="contacts-empty" style={{ padding: '16px 0' }}>暂无文档，上传文件开始构建知识库</div>
                ) : (
                  <div className="mcp-server-list">
                    {documents.map(doc => (
                      <div key={doc.doc_id} className="mcp-server-item">
                        <div className="mcp-server-main">
                          <div className="mcp-server-info">
                            <div className="mcp-server-name">
                              {doc.file_type === 'pdf' ? '📄' : doc.file_type === 'md' ? '📝' : doc.file_type === 'docx' ? '📃' : doc.file_type === 'pptx' ? '📊' : '📄'} {doc.file_name}
                              <span className={`mcp-badge ${doc.status === 'completed' ? 'enabled' : doc.status === 'failed' ? 'disabled' : 'sse'}`}>
                                {doc.status === 'completed' ? '已完成' : doc.status === 'failed' ? '失败' : doc.status === 'processing' ? '解析中' : '待解析'}
                              </span>
                              {doc.chunk_count > 0 && <span className="mcp-badge sse">{doc.chunk_count} 分块</span>}
                            </div>
                            <div className="mcp-server-url">{(doc.file_size / 1024).toFixed(1)} KB</div>
                          </div>
                        </div>
                        <div className="mcp-server-actions">
                          {doc.status === 'failed' && (
                            <button className="btn-icon" title="重试" onClick={() => handleDocRetry(doc.doc_id)} style={{ flexShrink: 0 }}>🔄</button>
                          )}
                          <button className="btn-icon" title="删除" onClick={() => handleDocDelete(doc.doc_id)} style={{ flexShrink: 0, color: 'var(--danger)' }}>✕</button>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        )}
      </div>

      {addBotModal && (
        <div className="modal-overlay" onClick={() => setAddBotModal(null)}>
          <div className="modal-card" onClick={e => e.stopPropagation()}>
            <h3>将 {addBotModal.botName} 拉入群聊</h3>
            <div className="form-field">
              <label>选择群聊</label>
              {groupConversations.length === 0 ? (
                <div className="picker-empty">暂无群聊会话</div>
              ) : (
                <select value={selectedConvId} onChange={e => setSelectedConvId(e.target.value)} style={{ width: '100%', padding: 8, borderRadius: 6, border: '1px solid var(--border)' }}>
                  <option value="">-- 请选择群聊 --</option>
                  {groupConversations.map(c => (
                    <option key={c.id} value={c.id}>{c.name}</option>
                  ))}
                </select>
              )}
            </div>
            <div className="modal-actions">
              <button className="btn-cancel" onClick={() => setAddBotModal(null)}>取消</button>
              <button className="btn-primary" onClick={handleAddBotToGroup} disabled={!selectedConvId}>确认拉入</button>
            </div>
          </div>
        </div>
      )}

      {editBot && (
        <div className="modal-overlay" onClick={() => setEditBot(null)}>
          <div className="modal-card" onClick={e => e.stopPropagation()}>
            <h3>编辑 Bot: {editBot.name}</h3>
            <div className="form-field">
              <label>Bot 名称</label>
              <input value={name} onChange={e => setName(e.target.value)} />
            </div>
            <div className="form-field">
              <label>头像</label>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                {avatarUrl ? (
                  <img src={avatarUrl} alt="头像" style={{ width: 48, height: 48, borderRadius: '50%', objectFit: 'cover' }} />
                ) : (
                  <div style={{ width: 48, height: 48, borderRadius: '50%', background: 'var(--bg-secondary)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 24 }}>🤖</div>
                )}
                <button className="btn-cancel" onClick={() => avatarInputRef.current?.click()}>选择图片</button>
              </div>
              <input ref={avatarInputRef} type="file" accept="image/*" style={{ display: 'none' }} onChange={handleAvatarUpload} />
            </div>
            <div className="form-field">
              <label>系统提示词</label>
              <textarea value={systemPrompt} onChange={e => setSystemPrompt(e.target.value)} rows={4} style={{ width: '100%', resize: 'vertical', padding: 8, borderRadius: 6, border: '1px solid var(--border)' }} />
            </div>
            <div className="form-field">
              <label>AI 服务商</label>
              <select value={provider} onChange={e => handleProviderChange(e.target.value)} style={{ width: '100%', padding: 8, borderRadius: 6, border: '1px solid var(--border)' }}>
                {AI_PROVIDERS.map(p => (
                  <option key={p.value} value={p.value}>{p.label}</option>
                ))}
              </select>
            </div>
            <div className="form-field">
              <label>API Base URL</label>
              <input value={baseUrl} onChange={e => setBaseUrl(e.target.value)} />
            </div>
            <div className="form-field">
              <label>API Key（留空则不修改）</label>
              <input type="password" value={apiKey} onChange={e => setApiKey(e.target.value)} placeholder="输入新 Key 以替换" />
            </div>
            <div className="form-field">
              <label>模型名称</label>
              <input value={model} onChange={e => setModel(e.target.value)} />
            </div>
            <div className="modal-actions">
              <button className="btn-cancel" onClick={() => setEditBot(null)}>取消</button>
              <button className="btn-primary" onClick={handleUpdate}>保存</button>
            </div>
          </div>
        </div>
      )}

      {mcpModal && (
        <div className="modal-overlay" onClick={() => setMcpModal(null)}>
          <div className="modal-card" onClick={e => e.stopPropagation()}>
            <h3>{mcpModal === 'create' ? '添加 MCP Server' : '编辑 MCP Server'}</h3>
            <div className="form-field">
              <label>名称 *</label>
              <input value={mcpForm.name} onChange={e => setMcpForm({ ...mcpForm, name: e.target.value })} placeholder="如: weather, search, mem0" />
            </div>
            <div className="form-field">
              <label>SSE URL *</label>
              <input value={mcpForm.url} onChange={e => setMcpForm({ ...mcpForm, url: e.target.value })} placeholder="http://mcp-server:8000/sse" />
            </div>
            <div className="form-field">
              <label>描述</label>
              <input value={mcpForm.description} onChange={e => setMcpForm({ ...mcpForm, description: e.target.value })} placeholder="MCP Server 功能描述" />
            </div>
            <div className="form-field">
              <label>传输协议</label>
              <select value={mcpForm.transport} onChange={e => setMcpForm({ ...mcpForm, transport: e.target.value })} style={{ width: '100%', padding: 8, borderRadius: 6, border: '1px solid var(--border)' }}>
                {MCP_TRANSPORTS.map(t => (
                  <option key={t.value} value={t.value}>{t.label}</option>
                ))}
              </select>
            </div>
            <div className="form-field">
              <label>认证类型</label>
              <select value={mcpForm.auth_type} onChange={e => setMcpForm({ ...mcpForm, auth_type: e.target.value })} style={{ width: '100%', padding: 8, borderRadius: 6, border: '1px solid var(--border)' }}>
                {MCP_AUTH_TYPES.map(a => (
                  <option key={a.value} value={a.value}>{a.label}</option>
                ))}
              </select>
            </div>
            {mcpForm.auth_type !== 'none' && (
              <div className="form-field">
                <label>认证令牌</label>
                <input type="password" value={mcpForm.auth_token} onChange={e => setMcpForm({ ...mcpForm, auth_token: e.target.value })} placeholder="Bearer Token 或 Base64(user:pass)" />
              </div>
            )}
            <div className="modal-actions">
              <button className="btn-cancel" onClick={() => setMcpModal(null)}>取消</button>
              <button className="btn-primary" onClick={mcpModal === 'create' ? handleMcpCreate : handleMcpUpdate}>
                {mcpModal === 'create' ? '添加' : '保存'}
              </button>
            </div>
          </div>
        </div>
      )}

      {kbModal && (
        <div className="modal-overlay" onClick={() => setKbModal(null)}>
          <div className="modal-card" onClick={e => e.stopPropagation()}>
            <h3>{kbModal === 'create' ? '创建知识库' : '编辑知识库'}</h3>
            <div className="form-field">
              <label>知识库名称 *</label>
              <input value={kbForm.name} onChange={e => setKbForm({ ...kbForm, name: e.target.value })} placeholder="如: 产品文档、公司制度" />
            </div>
            <div className="form-field">
              <label>描述</label>
              <textarea value={kbForm.description} onChange={e => setKbForm({ ...kbForm, description: e.target.value })} placeholder="知识库用途描述" rows={3} style={{ width: '100%', resize: 'vertical', padding: 8, borderRadius: 6, border: '1px solid var(--border)' }} />
            </div>
            <div className="modal-actions">
              <button className="btn-cancel" onClick={() => setKbModal(null)}>取消</button>
              <button className="btn-primary" onClick={kbModal === 'create' ? handleKbCreate : handleKbUpdate}>
                {kbModal === 'create' ? '创建' : '保存'}
              </button>
            </div>
          </div>
        </div>
      )}

      {bindModal && (
        <div className="modal-overlay" onClick={() => setBindModal(false)}>
          <div className="modal-card" onClick={e => e.stopPropagation()}>
            <h3>绑定知识库到 Bot</h3>
            <div className="form-field">
              <label>选择 Bot</label>
              <select value={bindKbBotId} onChange={e => { setBindKbBotId(e.target.value); if (e.target.value) loadBotKnowledgeBases(e.target.value) }} style={{ width: '100%', padding: 8, borderRadius: 6, border: '1px solid var(--border)' }}>
                <option value="">-- 请选择 Bot --</option>
                {bots.map(b => (
                  <option key={b.bot_id} value={b.bot_id}>{b.name}</option>
                ))}
              </select>
            </div>
            {bindKbBotId && (
              <>
                <div style={{ margin: '8px 0', fontSize: 13, fontWeight: 600 }}>已绑定的知识库</div>
                {botKnowledgeBases.length === 0 ? (
                  <div style={{ fontSize: 12, color: 'var(--text-secondary)', padding: '8px 0' }}>暂无绑定</div>
                ) : (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 4, marginBottom: 12 }}>
                    {botKnowledgeBases.map(kb => (
                      <div key={kb.kb_id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '6px 8px', background: 'var(--bg-secondary)', borderRadius: 6 }}>
                        <span style={{ fontSize: 13 }}>📚 {kb.name}</span>
                        <button className="btn-icon" style={{ fontSize: 12, color: 'var(--danger)' }} onClick={() => handleUnbindKb(kb.kb_id)}>解绑</button>
                      </div>
                    ))}
                  </div>
                )}
                <div style={{ margin: '8px 0', fontSize: 13, fontWeight: 600 }}>点击绑定当前知识库</div>
                <button className="btn-primary full" onClick={() => handleBindKb(selectedKbId)}>绑定此知识库</button>
              </>
            )}
            <div className="modal-actions" style={{ marginTop: 12 }}>
              <button className="btn-cancel" onClick={() => setBindModal(false)}>关闭</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
