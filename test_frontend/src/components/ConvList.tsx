import { useEffect, useMemo } from 'react'
import { useApp } from '../store/AppContext'
import './ConvList.css'

export default function ConvList() {
  const { conversations, activeConvId, setActiveConvId, loadConversations, onlineStatus, loadOnlineStatus, auth } = useApp()

  useEffect(() => {
    loadConversations()
  }, [loadConversations])

  // 收集所有会话中需要查询在线状态的用户ID（去重，排除自己）
  const allMemberIds = useMemo(() => {
    const ids = new Set<string>()
    for (const conv of conversations) {
      if (conv.type === 1) {
        for (const mid of conv.memberIds) {
          if (mid !== auth.userId) ids.add(mid)
        }
      }
    }
    return Array.from(ids)
  }, [conversations, auth.userId])

  // 当会话列表变化时，批量查询在线状态
  useEffect(() => {
    if (allMemberIds.length > 0) {
      loadOnlineStatus(allMemberIds)
    }
    // 每 30 秒刷新一次在线状态
    const timer = setInterval(() => {
      if (allMemberIds.length > 0) {
        loadOnlineStatus(allMemberIds)
      }
    }, 30000)
    return () => clearInterval(timer)
  }, [allMemberIds, loadOnlineStatus])

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
            // 单聊时，查找对方的在线状态
            const peerId = conv.type === 1
              ? conv.memberIds.find(id => id !== auth.userId)
              : undefined
            const isOnline = peerId ? onlineStatus[peerId] === true : false
            return (
              <div
                key={conv.id}
                className={`conv-item ${activeConvId === conv.id ? 'active' : ''}`}
                onClick={() => setActiveConvId(conv.id)}
              >
                <div className="conv-avatar-wrap">
                  <div className={`conv-avatar ${conv.type === 2 ? 'group' : 'private'}`}>
                    {conv.type === 2 ? '👥' : conv.name[0]?.toUpperCase() || '?'}
                  </div>
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
