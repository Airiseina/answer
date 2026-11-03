import { useEffect, useRef, useState } from 'react'
import { api, uploadFileTo } from '../api/client.ts'
import './SkillPanel.css'

interface BotOption {
  bot_id: string
  name: string
  is_system?: boolean
}

interface SkillInfo {
  skill_id: string
  owner_id: string
  name: string
  description: string
  version: string
  status: number
  kb_id: string
  size_bytes: number
  file_count: number
  created_at: string
  updated_at: string
}

interface SkillFileInfo {
  file_id: string
  skill_id: string
  rel_path: string
  object_url: string
  size_bytes: number
}

interface BotSkill {
  skill_id: string
  name: string
  description: string
  version: string
  enabled: boolean
}

// 与后端 model.UserSkillStatus 保持一致：0 待解析 / 1 可用 / 2 解析失败 / 3 已停用
const SKILL_STATUS_LABELS: Record<number, string> = {
  0: '待解析',
  1: '可用',
  2: '解析失败',
  3: '已停用',
}

function skillStatusBadge(status: number): string {
  if (status === 1) return 'enabled'
  if (status === 2 || status === 3) return 'disabled'
  return 'sse'
}

function formatSize(bytes: number): string {
  if (bytes <= 0) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function formatTime(ms: string): string {
  const value = Number(ms)
  if (!value) return ''
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

export default function SkillPanel({ bots }: { bots: BotOption[] }) {
  const [skills, setSkills] = useState<SkillInfo[]>([])
  const [selectedSkill, setSelectedSkill] = useState<SkillInfo | null>(null)
  const [files, setFiles] = useState<SkillFileInfo[]>([])
  const [warnings, setWarnings] = useState<string[]>([])
  const [botId, setBotId] = useState('')
  const [botSkills, setBotSkills] = useState<BotSkill[]>([])
  const [uploading, setUploading] = useState(false)
  const [toast, setToast] = useState('')
  const fileInputRef = useRef<HTMLInputElement>(null)

  const showToast = (msg: string) => {
    setToast(msg)
    setTimeout(() => setToast(''), 3000)
  }

  const loadSkills = async () => {
    const res = await api('GET', '/api/skill/list')
    if (res.code === 0) setSkills(res.data?.skills || [])
  }

  const loadBotSkills = async (target: string) => {
    if (!target) {
      setBotSkills([])
      return
    }
    const res = await api('GET', `/api/skill/bot_skills?bot_id=${target}`)
    if (res.code === 0) setBotSkills(res.data?.skills || [])
  }

  useEffect(() => {
    loadSkills()
  }, [])

  useEffect(() => {
    // 默认选中第一个可绑定 Bot，绑定面板才有意义。
    if (!botId) {
      const candidate = bots.find(b => !b.is_system)
      if (candidate) setBotId(candidate.bot_id)
    }
  }, [bots])

  useEffect(() => {
    loadBotSkills(botId)
  }, [botId])

  const handleUpload = async (file: File) => {
    setUploading(true)
    setWarnings([])
    const res = await uploadFileTo<{ skill_id: string; file_count: number; size_bytes: number; warnings?: string[] }>(
      '/api/skill/upload',
      file,
    )
    setUploading(false)
    if (res.code !== 0) {
      showToast(res.msg || '上传失败')
      return
    }
    const skipped: string[] = res.data?.warnings || []
    setWarnings(skipped)
    showToast(skipped.length > 0 ? `已导入 ${res.data?.file_count} 个文件，${skipped.length} 个被跳过` : 'Skill 已导入')
    loadSkills()
  }

  const handleSelect = async (skill: SkillInfo) => {
    setSelectedSkill(skill)
    const res = await api('GET', `/api/skill/detail?skill_id=${skill.skill_id}`)
    if (res.code === 0) {
      setFiles(res.data?.files || [])
      if (res.data?.skill) setSelectedSkill(res.data.skill)
    }
  }

  const handleDelete = async (skillId: string) => {
    const res = await api('POST', '/api/skill/delete', { skill_id: skillId })
    const ok = res.code === 0 && res.data?.success
    showToast(ok ? '已删除' : res.msg || '删除失败')
    if (ok) {
      if (selectedSkill?.skill_id === skillId) {
        setSelectedSkill(null)
        setFiles([])
      }
      loadSkills()
      loadBotSkills(botId)
    }
  }

  const handleBind = async (skillId: string, enabled: boolean) => {
    if (!botId) {
      showToast('请先选择要绑定的 Bot')
      return
    }
    const res = await api('POST', '/api/skill/bind', { bot_id: botId, skill_id: skillId, enabled })
    const ok = res.code === 0 && res.data?.success
    showToast(ok ? (enabled ? '已绑定到 Bot' : '已解绑') : res.msg || '操作失败')
    if (ok) loadBotSkills(botId)
  }

  const boundMap = new Map(botSkills.map(s => [s.skill_id, s]))

  return (
    <div className="skill-panel">
      {toast && <div className="bots-toast">{toast}</div>}

      <div className="skill-head">
        <span style={{ fontWeight: 600, fontSize: 14 }}>我的 Skill</span>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <input
            ref={fileInputRef}
            type="file"
            accept=".zip,.md,.markdown,.txt"
            style={{ display: 'none' }}
            onChange={e => {
              const file = e.target.files?.[0]
              if (file) handleUpload(file)
              if (fileInputRef.current) fileInputRef.current.value = ''
            }}
          />
          <button className="btn-primary" style={{ fontSize: 12, padding: '4px 12px' }} disabled={uploading} onClick={() => fileInputRef.current?.click()}>
            {uploading ? '上传中…' : '+ 上传 Skill'}
          </button>
        </div>
      </div>
      <div className="skill-hint">
        仅支持资料型 Skill：zip 包（含 SKILL.md 与 references/*.md）或单个 markdown 文件，单包不超过 10MB，内容只作为知识库检索资料，不执行任何脚本。
      </div>

      {warnings.length > 0 && (
        <div className="skill-warnings">
          {warnings.map((w, i) => (
            <div key={i}>· {w}</div>
          ))}
        </div>
      )}

      {skills.length === 0 ? (
        <div className="contacts-empty" style={{ padding: '20px 0' }}>暂无 Skill，点击"上传 Skill"开始</div>
      ) : (
        <div className="mcp-server-list">
          {skills.map(skill => (
            <div
              key={skill.skill_id}
              className="mcp-server-item"
              style={{ cursor: 'pointer' }}
              onClick={() => handleSelect(skill)}
            >
              <div className="mcp-server-main">
                <div className="mcp-server-info">
                  <div className="mcp-server-name">
                    🧩 {skill.name}
                    <span className={`mcp-badge ${skillStatusBadge(skill.status)}`}>{SKILL_STATUS_LABELS[skill.status] || '未知'}</span>
                    {boundMap.has(skill.skill_id) && <span className="mcp-badge enabled">已绑定</span>}
                  </div>
                  {skill.description && <div className="mcp-server-desc">{skill.description}</div>}
                  <div className="mcp-server-url">
                    {skill.version ? `v${skill.version} · ` : ''}
                    {skill.file_count} 个文件 · {formatSize(skill.size_bytes)}
                  </div>
                </div>
              </div>
              <div className="mcp-server-actions" onClick={e => e.stopPropagation()}>
                <button
                  className="btn-icon"
                  title={boundMap.get(skill.skill_id)?.enabled ? '从当前 Bot 解绑' : '绑定到当前 Bot'}
                  onClick={() => handleBind(skill.skill_id, !boundMap.get(skill.skill_id)?.enabled)}
                  style={{ flexShrink: 0 }}
                >
                  {boundMap.get(skill.skill_id)?.enabled ? '🔌' : '🔗'}
                </button>
                <button className="btn-icon" title="删除" onClick={() => handleDelete(skill.skill_id)} style={{ flexShrink: 0, color: 'var(--danger)' }}>✕</button>
              </div>
            </div>
          ))}
        </div>
      )}

      <div className="skill-bind">
        <label style={{ fontSize: 12, color: 'var(--text-secondary)' }}>绑定目标 Bot</label>
        <select value={botId} onChange={e => setBotId(e.target.value)} className="skill-bind-select">
          <option value="">选择 Bot</option>
          {bots.filter(b => !b.is_system).map(b => (
            <option key={b.bot_id} value={b.bot_id}>{b.name}</option>
          ))}
        </select>
        <div className="skill-hint" style={{ flex: 1, margin: 0 }}>
          {botSkills.length > 0 ? `该 Bot 已启用 ${botSkills.filter(s => s.enabled).length} 个 Skill，运行时会并入知识库检索。` : '该 Bot 尚未绑定 Skill。'}
        </div>
      </div>

      {selectedSkill && (
        <div className="skill-files">
          <div style={{ fontWeight: 600, fontSize: 14, margin: '4px 0 8px' }}>
            {selectedSkill.name} · 文件清单
          </div>
          {files.length === 0 ? (
            <div className="contacts-empty" style={{ padding: '12px 0' }}>暂无文件记录</div>
          ) : (
            <div className="mcp-server-list">
              {files.map(f => (
                <div key={f.file_id} className="mcp-server-item">
                  <div className="mcp-server-main">
                    <div className="mcp-server-info">
                      <div className="mcp-server-name">📄 {f.rel_path}</div>
                      <div className="mcp-server-url">{formatSize(f.size_bytes)}</div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
          <div className="skill-hint" style={{ marginTop: 8 }}>
            导入时间 {formatTime(selectedSkill.updated_at)} · 承载知识库 {selectedSkill.kb_id || '-'}
          </div>
        </div>
      )}
    </div>
  )
}
