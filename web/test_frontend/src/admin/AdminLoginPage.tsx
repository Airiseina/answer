import { useState } from 'react'
import { adminApi, setAdminToken } from './adminApi.ts'
import './admin.css'

interface Props {
  onSuccess: () => void
}

export default function AdminLoginPage({ onSuccess }: Props) {
  const [account, setAccount] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!account || !password) { setError('请输入账号与密码'); return }
    setError('')
    setLoading(true)
    try {
      const res = await adminApi('POST', '/api/admin/login', { account, password })
      if (res.code === 0 && res.data?.token) {
        setAdminToken(String(res.data.token))
        onSuccess()
      } else {
        setError(res.data || res.msg || '登录失败')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="admin-login-page">
      <form className="admin-login-card" onSubmit={handleSubmit}>
        <div className="admin-login-brand">
          <span className="admin-login-mark">A</span>
          <div>
            <h1>平台管理后台</h1>
            <p>管理员账号独立于普通用户体系</p>
          </div>
        </div>
        <label className="admin-field">
          <span>账号</span>
          <input
            type="text"
            value={account}
            onChange={e => setAccount(e.target.value)}
            placeholder="请输入管理员账号"
            autoComplete="username"
          />
        </label>
        <label className="admin-field">
          <span>密码</span>
          <input
            type="password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            placeholder="请输入密码"
            autoComplete="current-password"
          />
        </label>
        {error && <div className="admin-login-error">{error}</div>}
        <button type="submit" className="admin-login-btn" disabled={loading}>
          {loading ? '登录中...' : '登 录'}
        </button>
      </form>
    </div>
  )
}
