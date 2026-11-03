import { useCallback, useEffect, useState } from 'react'
import AdminLoginPage from './AdminLoginPage.tsx'
import AdminAccountsPanel from './AdminAccountsPanel.tsx'
import AdminMarketReviewPanel from './AdminMarketReviewPanel.tsx'
import AdminPricingPanel from './AdminPricingPanel.tsx'
import { adminApi, clearAdminToken, getAdminToken, ADMIN_ROLE_LABELS, type AdminProfile } from './adminApi.ts'
import './admin.css'

type TabKey = 'overview' | 'accounts' | 'review' | 'pricing' | 'tenants'

interface NavItem {
  key: TabKey
  label: string
  ready: boolean
}

// 骨架阶段就先固定导航结构，后续市场审核、参考单价、租户治理各自替换占位页。
const NAV_ITEMS: NavItem[] = [
  { key: 'overview', label: '概览', ready: true },
  { key: 'accounts', label: '账号管理', ready: true },
  { key: 'review', label: '市场审核', ready: true },
  { key: 'pricing', label: '参考单价', ready: true },
  { key: 'tenants', label: '租户治理', ready: false },
]

export default function AdminApp() {
  const [token, setToken] = useState(getAdminToken())
  const [profile, setProfile] = useState<AdminProfile | null>(null)
  const [tab, setTab] = useState<TabKey>('overview')

  const loadProfile = useCallback(async () => {
    const res = await adminApi('GET', '/api/admin/profile')
    if (res.code === 0) {
      setProfile(res.data)
    } else {
      clearAdminToken()
      setToken('')
      setProfile(null)
    }
  }, [])

  useEffect(() => {
    if (!token) return
    loadProfile()
  }, [token, loadProfile])

  const logout = useCallback(() => {
    clearAdminToken()
    setToken('')
    setProfile(null)
  }, [])

  if (!token) {
    return <AdminLoginPage onSuccess={() => setToken(getAdminToken())} />
  }

  if (!profile) {
    return <div className="admin-loading">加载中...</div>
  }

  const current = NAV_ITEMS.find(n => n.key === tab) || NAV_ITEMS[0]

  return (
    <div className="admin-layout">
      <aside className="admin-sidebar">
        <div className="admin-sidebar-title">平台管理后台</div>
        <nav>
          {NAV_ITEMS.map(item => (
            <button
              key={item.key}
              className={item.key === tab ? 'admin-nav-item active' : 'admin-nav-item'}
              onClick={() => setTab(item.key)}
            >
              {item.label}
              {!item.ready && <span className="admin-nav-badge">待上线</span>}
            </button>
          ))}
        </nav>
      </aside>
      <div className="admin-main">
        <header className="admin-header">
          <div className="admin-header-title">{current.label}</div>
          <div className="admin-header-user">
            <span className="admin-user-name">{profile.name}</span>
            <span className="admin-user-role">{ADMIN_ROLE_LABELS[profile.role] || profile.role}</span>
            <button className="admin-btn ghost" onClick={logout}>退出登录</button>
          </div>
        </header>
        <main className="admin-content">
          {tab === 'overview' && <AdminOverview profile={profile} />}
          {tab === 'accounts' && <AdminAccountsPanel profile={profile} />}
          {tab === 'review' && <AdminMarketReviewPanel profile={profile} />}
          {tab === 'pricing' && <AdminPricingPanel profile={profile} />}
          {tab !== 'overview' && tab !== 'accounts' && tab !== 'review' && tab !== 'pricing' && (
            <div className="admin-placeholder">
              <h2>{current.label}</h2>
              <p>该模块将在后续阶段接入。当前已完成独立管理后台账号体系。</p>
            </div>
          )}
        </main>
      </div>
    </div>
  )
}

function AdminOverview({ profile }: { profile: AdminProfile }) {
  const cards = [
    { label: '登录账号', value: profile.account },
    { label: '显示名称', value: profile.name },
    { label: '角色', value: ADMIN_ROLE_LABELS[profile.role] || profile.role },
    { label: '最后登录', value: profile.last_login_at ? new Date(profile.last_login_at).toLocaleString('zh-CN', { hour12: false }) : '—' },
  ]
  return (
    <div className="admin-panel">
      <div className="admin-panel-head"><h2>概览</h2></div>
      <div className="admin-cards">
        {cards.map(c => (
          <div className="admin-card" key={c.label}>
            <span className="admin-card-label">{c.label}</span>
            <span className="admin-card-value">{c.value}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
