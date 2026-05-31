import { useState } from 'react'
import { useApp } from '../store/AppContext.tsx'
import { api } from '../api/client.ts'
import './LoginPage.css'

export default function LoginPage() {
  const { login } = useApp()
  const [isRegister, setIsRegister] = useState(false)
  const [account, setAccount] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      if (isRegister) {
        if (!account || !username || !password) { setError('请填写所有字段'); return }
        const res = await api('POST', '/register', { account, name: username, password })
        if (res.code === 0) {
          setIsRegister(false)
          setError('')
          setAccount('')
          setUsername('')
          setPassword('')
        } else {
          setError(res.msg || '注册失败')
        }
      } else {
        if (!account || !password) { setError('请填写所有字段'); return }
        const res = await api('POST', '/login', { account, password })
        if (res.code === 0 && res.data?.token) {
          login(String(res.data.token), String(res.data.account), res.data.avatar_url || '')
        } else {
          setError(res.msg || '登录失败')
        }
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-page">
      <div className="login-bg">
        <div className="login-circle login-circle-1" />
        <div className="login-circle login-circle-2" />
        <div className="login-circle login-circle-3" />
      </div>
      <div className="login-card">
        <div className="login-header">
          <div className="login-logo">
            <svg viewBox="0 0 40 40" width="48" height="48">
              <circle cx="20" cy="20" r="18" fill="#12B7F5" />
              <text x="20" y="26" textAnchor="middle" fill="white" fontSize="16" fontWeight="bold">IM</text>
            </svg>
          </div>
          <h1>{isRegister ? '创建账号' : '欢迎回来'}</h1>
          <p>{isRegister ? '注册一个新账号开始聊天' : '登录你的账号继续交流'}</p>
        </div>
        <form onSubmit={handleSubmit} className="login-form">
          <div className="form-field">
            <label>账号</label>
            <input
              type="text"
              value={account}
              onChange={e => setAccount(e.target.value)}
              placeholder="请输入账号"
              autoComplete="username"
            />
          </div>
          {isRegister && (
            <div className="form-field">
              <label>用户名</label>
              <input
                type="text"
                value={username}
                onChange={e => setUsername(e.target.value)}
                placeholder="请输入用户名"
              />
            </div>
          )}
          <div className="form-field">
            <label>密码</label>
            <input
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              placeholder="请输入密码"
              autoComplete={isRegister ? 'new-password' : 'current-password'}
            />
          </div>
          {error && <div className="login-error">{error}</div>}
          <button type="submit" className="login-btn" disabled={loading}>
            {loading ? '请稍候...' : isRegister ? '注 册' : '登 录'}
          </button>
        </form>
        <div className="login-switch">
          {isRegister ? (
            <span>已有账号？<button onClick={() => { setIsRegister(false); setError('') }}>立即登录</button></span>
          ) : (
            <span>没有账号？<button onClick={() => { setIsRegister(true); setError('') }}>立即注册</button></span>
          )}
        </div>
      </div>
    </div>
  )
}
