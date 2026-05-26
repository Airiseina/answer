import { useState } from 'react'
import { api } from '../api/client'
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
  bot_id: number
  creator_id: number
  name: string
  avatar_url: string
  system_prompt: string
  model: string
  base_url?: string
  is_system: boolean
  created_at: number
}

export default function BotsPanel({ onSwitchToChat }: { onSwitchToChat: () => void }) {
  const { openSystemAI, conversations, setActiveConvId } = useApp()
  const [tab, setTab] = useState<'create' | 'list'>('list')
  const [bots, setBots] = useState<BotInfo[]>([])
  const [toast, setToast] = useState('')

  const [name, setName] = useState('')
  const [systemPrompt, setSystemPrompt] = useState('')
  const [provider, setProvider] = useState('openai')
  const [baseUrl, setBaseUrl] = useState(AI_PROVIDERS[0].baseUrl)
  const [apiKey, setApiKey] = useState('')
  const [model, setModel] = useState('')

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

  const handleDelete = async (botId: number) => {
    const res = await api('POST', '/api/bot/delete', { bot_id: botId })
    showToast(res.code === 0 ? '已删除' : res.msg || '删除失败')
    if (res.code === 0) loadBots()
  }

  const handleStartBotChat = async (botId: number) => {
    const addRes = await api('POST', '/api/bot/add_to_conversation', {
      bot_id: botId,
      conversation_id: 0,
      conversation_type: 1,
    })
    if (addRes.code === 0) {
      const convId = String(addRes.data?.conversation_id || '')
      if (convId && convId !== '0') {
        const existing = conversations.find(c => c.id === convId)
        if (existing) {
          setActiveConvId(convId)
        }
      }
      onSwitchToChat()
    } else {
      showToast(addRes.msg || '创建会话失败')
    }
  }

  const handleSystemAI = async () => {
    await openSystemAI()
    onSwitchToChat()
  }

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
        <button className="btn-primary full" style={{ marginBottom: 12 }} onClick={handleSystemAI}>
          🤖 与系统 AI 助手对话
        </button>

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
                      <div className="friend-name">{b.name}{b.is_system ? ' (系统)' : ''}</div>
                      <div className="friend-sub">{b.model}</div>
                    </div>
                  </div>
                  <button className="btn-icon" title="开始对话" onClick={() => handleStartBotChat(b.bot_id)} style={{ flexShrink: 0 }}>💬</button>
                  {!b.is_system && (
                    <button className="btn-icon" title="删除" onClick={() => handleDelete(b.bot_id)} style={{ flexShrink: 0, color: 'var(--danger)' }}>✕</button>
                  )}
                </div>
              ))
            )}
          </div>
        )}
      </div>
    </div>
  )
}
