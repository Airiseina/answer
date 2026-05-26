import { useRef } from 'react'
import { useApp } from '../store/AppContext'
import { uploadFile } from '../api/client'
import './Sidebar.css'

interface Props {
  account: string
  wsConnected: boolean
  tab: string
  onTabChange: (tab: 'chat' | 'contacts' | 'groups' | 'bots') => void
  onWsToggle: () => void
  onLogout: () => void
  onOpenSystemAI: () => void
}

export default function Sidebar({ account, wsConnected, tab, onTabChange, onWsToggle, onLogout, onOpenSystemAI }: Props) {
  const { memberInfo, updateAvatar } = useApp()
  const avatarInputRef = useRef<HTMLInputElement>(null)
  const avatar = memberInfo[account]?.avatar

  const handleAvatarClick = () => {
    avatarInputRef.current?.click()
  }

  const handleAvatarChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    try {
      const res = await uploadFile(file)
      if (res.code !== 0 || !res.data?.url) {
        alert(res.msg || '上传失败')
        return
      }
      await updateAvatar(res.data.url)
    } catch {
      alert('上传头像失败')
    }
    if (avatarInputRef.current) avatarInputRef.current.value = ''
  }

  return (
    <div className="sidebar">
      <div className="sidebar-avatar" onClick={handleAvatarClick} title="点击更换头像">
        {avatar ? (
          <img src={avatar} alt={account} className="sidebar-avatar-img" />
        ) : (
          <div className="avatar-circle">{account ? account[0].toUpperCase() : '?'}</div>
        )}
        <div className="sidebar-avatar-overlay">📷</div>
      </div>
      <input
        ref={avatarInputRef}
        type="file"
        accept="image/*"
        style={{ display: 'none' }}
        onChange={handleAvatarChange}
      />

      <nav className="sidebar-nav">
        <button className={`sidebar-btn ${tab === 'chat' ? 'active' : ''}`} onClick={() => onTabChange('chat')} title="聊天">
          <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
          </svg>
        </button>
        <button className={`sidebar-btn ${tab === 'bots' ? 'active' : ''}`} onClick={() => onTabChange('bots')} title="AI 助手">
          <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M12 2a4 4 0 0 1 4 4v2a4 4 0 0 1-8 0V6a4 4 0 0 1 4-4z"/>
            <path d="M16 14H8a4 4 0 0 0-4 4v2h16v-2a4 4 0 0 0-4-4z"/>
            <circle cx="9" cy="7" r="0.5" fill="currentColor"/>
            <circle cx="15" cy="7" r="0.5" fill="currentColor"/>
          </svg>
        </button>
        <button className={`sidebar-btn ${tab === 'contacts' ? 'active' : ''}`} onClick={() => onTabChange('contacts')} title="联系人">
          <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/>
            <circle cx="9" cy="7" r="4"/>
            <path d="M23 21v-2a4 4 0 0 0-3-3.87"/>
            <path d="M16 3.13a4 4 0 0 1 0 7.75"/>
          </svg>
        </button>
        <button className={`sidebar-btn ${tab === 'groups' ? 'active' : ''}`} onClick={() => onTabChange('groups')} title="群组">
          <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M12 2L2 7l10 5 10-5-10-5z"/>
            <path d="M2 17l10 5 10-5"/>
            <path d="M2 12l10 5 10-5"/>
          </svg>
        </button>
      </nav>

      <div className="sidebar-bottom">
        <button className={`sidebar-btn ws-btn ${wsConnected ? 'ws-on' : ''}`} onClick={onWsToggle} title={wsConnected ? '断开连接' : '连接服务器'}>
          <span className={`ws-dot ${wsConnected ? 'on' : 'off'}`} />
        </button>
        <button className="sidebar-btn" onClick={onLogout} title="退出登录">
          <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/>
            <polyline points="16 17 21 12 16 7"/>
            <line x1="21" y1="12" x2="9" y2="12"/>
          </svg>
        </button>
      </div>
    </div>
  )
}
