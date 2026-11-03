import { useCallback, useEffect, useState } from 'react'
import { adminApi, type AdminProfile } from './adminApi.ts'

interface Props {
  profile: AdminProfile | null
}

interface MarketBotInfo {
  market_bot_id: string
  publisher_id: string
  publisher_name: string
  name: string
  description: string
  avatar_url: string
  system_prompt: string
  model: string
  base_url?: string
  category: string
  tags: string
  status: number
  review_note: string
  install_count: number
  rating: number
  rating_count: number
  version: number
  created_at: string
  updated_at: string
}

const STATUS_LABELS: Record<number, string> = {
  0: '草稿',
  1: '待审核',
  2: '已发布',
  3: '已驳回',
  4: '已下架',
}

const STATUS_FILTERS = [1, 2, 3, 4]

function statusClass(status: number): string {
  if (status === 2) return 'admin-tag ok'
  if (status === 3 || status === 4) return 'admin-tag off'
  return 'admin-tag'
}

function formatTime(ts: string): string {
  const n = Number(ts)
  if (!n) return '—'
  return new Date(n).toLocaleString('zh-CN', { hour12: false })
}

export default function AdminMarketReviewPanel({ profile }: Props) {
  const [items, setItems] = useState<MarketBotInfo[]>([])
  const [status, setStatus] = useState(1)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [active, setActive] = useState<MarketBotInfo | null>(null)
  const [note, setNote] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const load = useCallback(async (nextStatus: number) => {
    setLoading(true)
    setError('')
    const res = await adminApi('GET', `/api/admin/market/reviews?status=${nextStatus}&page_size=100`)
    if (res.code === 0) {
      setItems(res.data?.bots || [])
    } else {
      setError(res.msg || '加载失败')
    }
    setLoading(false)
  }, [])

  useEffect(() => { load(status) }, [status, load])

  const openReview = (item: MarketBotInfo) => {
    setActive(item)
    setNote('')
  }

  const review = async (approve: boolean) => {
    if (!active) return
    setSubmitting(true)
    setError('')
    const action = approve ? 'approve' : 'reject'
    const res = await adminApi('POST', `/api/admin/market/reviews/${active.market_bot_id}/${action}`, { note })
    if (res.code === 0) {
      setActive(null)
      await load(status)
    } else {
      setError(res.data || res.msg || '审核失败')
    }
    setSubmitting(false)
  }

  return (
    <div className="admin-panel">
      <div className="admin-panel-head">
        <h2>市场审核</h2>
        <div className="admin-form-row" style={{ margin: 0 }}>
          <select value={status} onChange={e => setStatus(Number(e.target.value))}>
            {STATUS_FILTERS.map(s => <option key={s} value={s}>{STATUS_LABELS[s]}</option>)}
          </select>
          <button className="admin-btn ghost" onClick={() => load(status)} disabled={loading}>刷新</button>
        </div>
      </div>

      {error && <div className="admin-alert">{error}</div>}

      <table className="admin-table">
        <thead>
          <tr>
            <th>名称</th>
            <th>发布者</th>
            <th>分类</th>
            <th>状态</th>
            <th>安装</th>
            <th>版本</th>
            <th>更新时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {items.map(item => (
            <tr key={item.market_bot_id}>
              <td>{item.name}</td>
              <td>{item.publisher_name || item.publisher_id}</td>
              <td>{item.category || '—'}</td>
              <td><span className={statusClass(item.status)}>{STATUS_LABELS[item.status] || item.status}</span></td>
              <td>{item.install_count}</td>
              <td>v{item.version}</td>
              <td>{formatTime(item.updated_at)}</td>
              <td>
                <button className="admin-btn ghost" onClick={() => openReview(item)}>
                  {item.status === 1 ? '审核' : '查看'}
                </button>
              </td>
            </tr>
          ))}
          {items.length === 0 && !loading && (
            <tr><td colSpan={8} className="admin-empty">暂无条目</td></tr>
          )}
        </tbody>
      </table>

      <p className="admin-hint">
        当前审核员：{profile?.name || '—'}。审核通过后条目上架并递增版本；驳回需填写原因，会展示给发布者。
      </p>

      {active && (
        <div className="admin-modal-overlay" onClick={() => setActive(null)}>
          <div className="admin-modal" onClick={e => e.stopPropagation()}>
            <h3>{active.name}</h3>
            <div className="admin-detail">
              <div><span>发布者</span>{active.publisher_name || active.publisher_id}</div>
              <div><span>分类 / 标签</span>{active.category || '—'} / {active.tags || '—'}</div>
              <div><span>推荐模型</span>{active.model || '—'}</div>
              <div><span>描述</span>{active.description || '—'}</div>
              <div><span>当前状态</span>{STATUS_LABELS[active.status] || active.status}</div>
            </div>
            <div className="admin-prompt-label">系统提示词</div>
            <pre className="admin-prompt">{active.system_prompt}</pre>
            <textarea
              className="admin-note"
              placeholder="审核意见（驳回时必填）"
              value={note}
              onChange={e => setNote(e.target.value)}
              rows={3}
            />
            <div className="admin-modal-actions">
              <button className="admin-btn ghost" onClick={() => setActive(null)}>取消</button>
              <button className="admin-btn danger" disabled={submitting} onClick={() => review(false)}>驳回</button>
              <button className="admin-btn primary" disabled={submitting} onClick={() => review(true)}>通过并上架</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
