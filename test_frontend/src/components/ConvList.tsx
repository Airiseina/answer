import { useEffect, useMemo } from 'react'
import { useApp } from '../store/AppContext'
import './ConvList.css'

export default function ConvList() {
  const { conversations, activeConvId, setActiveConvId, loadConversations, onlineStatus, loadOnlineStatus, memberInfo, loadConversationMembers, auth } = useApp()

  useEffect(() => {
    loadConversations()
  }, [loadConversations])

  const allMemberAccounts = useMemo(() => {
    const accounts = new Set<string>()
    for (const conv of conversations) {
      if (conv.type === 1) {
        for (const acc of conv.memberAccounts) {
          if (acc !== auth.account) accounts.add(acc)
        }
      }
    }
    return Array.from(accounts)
  }, [conversations, auth.account])

  useEffect(() => {
    if (allMemberAccounts.length > 0) {
      loadOnlineStatus(allMemberAccounts)
      loadConversationMembers(allMemberAccounts)
    }
    const timer = setInterval(() => {
      if (allMemberAccounts.length > 0) {
        loadOnlineStatus(allMemberAccounts)
      }
    }, 30000)
    return () => clearInterval(timer)
  }, [allMemberAccounts, loadOnlineStatus, loadConversationMembers])

  return (
    <div className="conv-list">
      <div className="conv-header">
        <h3>消息</h3>
      </div>
      <div className="conv-search">
        <input placeholder="搜索联系人..." />
      </div>
      <div className="conv-items">
        {conversations.length === 0 ? (
          <div className="conv-empty">
            <p>暂无会话</p>
            <p className="conv-empty-hint">通过联系人或群组开始聊天</p>
          </div>
        ) : (
          conversations.map(conv => {
            const peerAccount = conv.type === 1
              ? conv.memberAccounts.find(acc => acc !== auth.account)
              : undefined
            const isOnline = peerAccount ? onlineStatus[peerAccount] === true : false
            return (
              <div
                key={conv.id}
                className={`conv-item ${activeConvId === conv.id ? 'active' : ''}`}
                onClick={() => setActiveConvId(conv.id)}
              >
                <div className="conv-avatar-wrap">
                  {memberInfo[peerAccount!]?.avatar ? (
                    <img src={memberInfo[peerAccount!].avatar} alt={conv.name} className="conv-avatar-img" />
                  ) : (
                    <div className={`conv-avatar ${conv.type === 2 ? 'group' : 'private'}`}>
                      {conv.type === 2 ? '👥' : conv.name[0]?.toUpperCase() || '?'}
                    </div>
                  )}
                  {conv.type === 1 && (
                    <span className={`online-dot ${isOnline ? 'online' : 'offline'}`} />
                  )}
                </div>
                <div className="conv-info">
                  <div className="conv-name">{conv.name}</div>
                  <div className="conv-last">{conv.lastMsg || ''}</div>
                </div>
                {conv.unread > 0 && <div className="conv-badge">{conv.unread}</div>}
              </div>
            )
          })
        )}
      </div>
    </div>
  )
}
