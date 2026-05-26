import { useState, useRef } from 'react'
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

export default function BotsPanel({ onSwitchToChat }: { onSwitchToChat: () => void }) {
  const { conversations, setActiveConvId, loadConversations } = useApp()
  const [tab, setTab] = useState<'create' | 'list'>('list')
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

  const groupConversations = conversations.filter(c => c.type === 2)

  return (
    <div className="bots-panel">
      {toast && <div className="bots-toast">{toast}</div>}
      <div className="bots-header">
        <h3>AI 助手</h3>
      </div>
      <div className="bots-tabs">
        <button className={`ctab ${tab === 'list' ? 'active' : ''}`} onClick={() => { setTab('list'); loadBots() }}>我的 Bot</button>
        <button className={`ctab ${tab === 'create' ? 'active' : ''}`} onClick={() => setTab('create')}>创建 Bot</button>
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
    </div>
  )
}
