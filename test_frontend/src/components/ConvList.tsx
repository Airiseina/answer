import { useApp } from '../store/AppContext'
import './ConvList.css'

export default function ConvList() {
  const { conversations, activeConvId, setActiveConvId } = useApp()

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
          conversations.map(conv => (
            <div
              key={conv.id}
              className={`conv-item ${activeConvId === conv.id ? 'active' : ''}`}
              onClick={() => setActiveConvId(conv.id)}
            >
              <div className={`conv-avatar ${conv.type === 2 ? 'group' : 'private'}`}>
                {conv.type === 2 ? '👥' : conv.name[0]?.toUpperCase() || '?'}
              </div>
              <div className="conv-info">
                <div className="conv-name">{conv.name}</div>
                <div className="conv-last">{conv.lastMsg || ''}</div>
              </div>
              {conv.unread > 0 && <div className="conv-badge">{conv.unread}</div>}
            </div>
          ))
        )}
      </div>
    </div>
  )
}
