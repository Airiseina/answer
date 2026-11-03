import { useCallback, useEffect, useState } from 'react'
import { adminApi, ADMIN_ROLE_LABELS, ADMIN_STATUS_LABELS, type AdminAccount, type AdminProfile } from './adminApi.ts'

interface Props {
  profile: AdminProfile | null
}

const ROLE_OPTIONS = ['ops', 'reviewer']

function formatTime(ts: number): string {
  if (!ts) return '—'
  return new Date(ts).toLocaleString('zh-CN', { hour12: false })
}

export default function AdminAccountsPanel({ profile }: Props) {
  const isSuper = profile?.role === 'super'
  const [accounts, setAccounts] = useState<AdminAccount[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [form, setForm] = useState({ account: '', name: '', role: 'ops', password: '' })
  const [creating, setCreating] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    const res = await adminApi('GET', '/api/admin/accounts')
    if (res.code === 0) {
      setAccounts(res.data || [])
    } else {
      setError(res.msg || '加载失败')
    }
    setLoading(false)
  }, [])

  useEffect(() => { load() }, [load])

  const create = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.account || !form.name || !form.password) { setError('请填写账号、名称与初始口令'); return }
    setCreating(true)
    setError('')
    const res = await adminApi('POST', '/api/admin/accounts/create', form)
    if (res.code === 0) {
      setForm({ account: '', name: '', role: 'ops', password: '' })
      await load()
    } else {
      setError(res.data || res.msg || '创建失败')
    }
    setCreating(false)
  }

  const toggleStatus = async (item: AdminAccount) => {
    const next = item.status === 0 ? 1 : 0
    const res = await adminApi('POST', '/api/admin/accounts/status', {
      admin_id: Number(item.admin_id),
      status: next,
    })
    if (res.code === 0) {
      await load()
    } else {
      setError(res.data || res.msg || '操作失败')
    }
  }

  return (
    <div className="admin-panel">
      <div className="admin-panel-head">
        <h2>后台账号</h2>
        <button className="admin-btn ghost" onClick={load} disabled={loading}>刷新</button>
      </div>

      {error && <div className="admin-alert">{error}</div>}

      <table className="admin-table">
        <thead>
          <tr>
            <th>账号</th>
            <th>名称</th>
            <th>角色</th>
            <th>状态</th>
            <th>最后登录</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {accounts.map(a => (
            <tr key={a.admin_id}>
              <td>{a.account}</td>
              <td>{a.name}</td>
              <td>{ADMIN_ROLE_LABELS[a.role] || a.role}</td>
              <td>
                <span className={a.status === 0 ? 'admin-tag ok' : 'admin-tag off'}>
                  {ADMIN_STATUS_LABELS[a.status] || a.status}
                </span>
              </td>
              <td>{formatTime(a.last_login_at)}</td>
              <td>
                <button
                  className="admin-btn ghost"
                  disabled={!isSuper || a.admin_id === profile?.admin_id}
                  onClick={() => toggleStatus(a)}
                >
                  {a.status === 0 ? '停用' : '启用'}
                </button>
              </td>
            </tr>
          ))}
          {accounts.length === 0 && !loading && (
            <tr><td colSpan={6} className="admin-empty">暂无账号</td></tr>
          )}
        </tbody>
      </table>

      {isSuper && (
        <form className="admin-inline-form" onSubmit={create}>
          <h3>新建账号</h3>
          <div className="admin-form-row">
            <input
              placeholder="账号"
              value={form.account}
              onChange={e => setForm({ ...form, account: e.target.value })}
            />
            <input
              placeholder="名称"
              value={form.name}
              onChange={e => setForm({ ...form, name: e.target.value })}
            />
            <select value={form.role} onChange={e => setForm({ ...form, role: e.target.value })}>
              {ROLE_OPTIONS.map(r => <option key={r} value={r}>{ADMIN_ROLE_LABELS[r]}</option>)}
            </select>
            <input
              type="password"
              placeholder="初始口令"
              value={form.password}
              onChange={e => setForm({ ...form, password: e.target.value })}
            />
            <button type="submit" className="admin-btn primary" disabled={creating}>
              {creating ? '创建中...' : '创建'}
            </button>
          </div>
        </form>
      )}
      {!isSuper && <p className="admin-hint">仅超级管理员可创建或启停后台账号。</p>}
    </div>
  )
}
