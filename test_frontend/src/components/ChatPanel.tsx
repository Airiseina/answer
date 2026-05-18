import { useState, useRef, useEffect } from 'react'
import { useApp } from '../store/AppContext'
import { api } from '../api/client'
import './ChatPanel.css'

export default function ChatPanel() {
  const { conversations, messages, activeConvId, sendMessage, setActiveConvId, friends, openChatWith, auth } = useApp()
  const [input, setInput] = useState('')
  const [showNewChat, setShowNewChat] = useState(false)
  const [newChatType, setNewChatType] = useState<number>(1)
  const [newChatGroupId, setNewChatGroupId] = useState('')
  const [newChatName, setNewChatName] = useState('')
  const [selectedFriendIds, setSelectedFriendIds] = useState<number[]>([])
  const msgEndRef = useRef<HTMLDivElement>(null)

  const conv = conversations.find(c => c.id === activeConvId)
  const msgs = activeConvId ? (messages[activeConvId] || []) : []

  useEffect(() => {
    msgEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [msgs.length])

  const handleSend = () => {
    if (!input.trim() || !activeConvId) return
    sendMessage(activeConvId, input.trim())
    setInput('')
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const handleSelectFriend = (friendId: number, name: string) => {
    openChatWith(friendId, name, 1)
    setShowNewChat(false)
  }

  const handleCreateGroupConversation = async () => {
    if (selectedFriendIds.length === 0) return
    const allMemberIds = [Number(auth.userId), ...selectedFriendIds]
    const res = await api('POST', '/api/v1/chat/conversation', {
      type: 2,
      name: newChatName || `群聊`,
      member_ids: allMemberIds,
    })
    if (res.code === 0 && res.data?.conversation_id) {
      const convId = res.data.conversation_id
      openChatWith(convId, newChatName || `群聊`, 2)
      setShowNewChat(false)
      setNewChatName('')
      setSelectedFriendIds([])
    }
  }

  const toggleFriend = (fid: number) => {
    setSelectedFriendIds(prev =>
      prev.includes(fid) ? prev.filter(id => id !== fid) : [...prev, fid]
    )
  }

  const formatTime = (ts: number) => {
    const d = new Date(ts * 1000)
    return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
  }

  const renderNewChatModal = () => (
    <div className="modal-overlay" onClick={() => setShowNewChat(false)}>
      <div className="modal-card" onClick={e => e.stopPropagation()}>
        <h3>发起新会话</h3>
        <div className="modal-field">
          <label>类型</label>
          <div className="radio-group">
            <label className={`radio ${newChatType === 1 ? 'active' : ''}`}>
              <input type="radio" checked={newChatType === 1} onChange={() => setNewChatType(1)} />
              私聊
            </label>
            <label className={`radio ${newChatType === 2 ? 'active' : ''}`}>
              <input type="radio" checked={newChatType === 2} onChange={() => setNewChatType(2)} />
              群聊
            </label>
          </div>
        </div>
        {newChatType === 1 ? (
          <div className="modal-field friend-picker">
            <label>选择好友</label>
            {friends.length === 0 ? (
              <div className="picker-empty">暂无好友，请先添加好友</div>
            ) : (
              <div className="friend-picker-list">
                {friends.map(f => (
                  <div key={f.friend_id} className="friend-picker-item" onClick={() => handleSelectFriend(f.friend_id, f.remark || f.name)}>
                    <div className="fp-avatar">{(f.remark || f.name || '?')[0].toUpperCase()}</div>
                    <div className="fp-info">
                      <div className="fp-name">{f.remark || f.name}</div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        ) : (
          <div className="modal-field">
            <label>群聊名称</label>
            <input value={newChatName} onChange={e => setNewChatName(e.target.value)} placeholder="输入群聊名称" />
            <label style={{ marginTop: 8 }}>选择好友拉入群聊</label>
            {friends.length === 0 ? (
              <div className="picker-empty">暂无好友</div>
            ) : (
              <div className="friend-picker-list">
                {friends.map(f => (
                  <div key={f.friend_id}
                    className={`friend-picker-item ${selectedFriendIds.includes(f.friend_id) ? 'selected' : ''}`}
                    onClick={() => toggleFriend(f.friend_id)}
                  >
                    <div className="fp-avatar">{(f.remark || f.name || '?')[0].toUpperCase()}</div>
                    <div className="fp-info">
                      <div className="fp-name">{f.remark || f.name}</div>
                    </div>
                    {selectedFriendIds.includes(f.friend_id) && <div className="fp-check">✓</div>}
                  </div>
                ))}
              </div>
            )}
            {selectedFriendIds.length > 0 && (
              <div className="selected-count">已选 {selectedFriendIds.length} 人</div>
            )}
          </div>
        )}
        <div className="modal-actions">
          <button className="btn-cancel" onClick={() => setShowNewChat(false)}>取消</button>
          {newChatType === 2 && (
            <button className="btn-primary" onClick={handleCreateGroupConversation} disabled={selectedFriendIds.length === 0}>创建群聊</button>
          )}
        </div>
      </div>
    </div>
  )

  if (!conv) {
    return (
      <div className="chat-panel empty">
        <div className="chat-empty">
          <div className="chat-empty-icon">💬</div>
          <p>选择一个会话开始聊天</p>
          <button className="new-chat-btn" onClick={() => setShowNewChat(true)}>发起新会话</button>
        </div>
        {showNewChat && renderNewChatModal()}
      </div>
    )
  }

  return (
    <div className="chat-panel">
      <div className="chat-header">
        <div className="chat-header-info">
          <span className="chat-header-name">{conv.name}</span>
          <span className="chat-header-type">{conv.type === 2 ? '群聊' : '私聊'}</span>
        </div>
        <button className="chat-new-btn" onClick={() => setShowNewChat(true)} title="新会话">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
        </button>
      </div>
      <div className="chat-messages">
        {msgs.map(msg => (
          <div key={msg.id} className={`msg-row ${msg.isSent ? 'sent' : 'received'}`}>
            <div className="msg-bubble">
              <div className="msg-content">{msg.content}</div>
              <div className="msg-time">{formatTime(msg.time)}</div>
            </div>
          </div>
        ))}
        <div ref={msgEndRef} />
      </div>
      <div className="chat-input-area">
        <textarea
          value={input}
          onChange={e => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="输入消息，Enter 发送..."
          rows={3}
        />
        <button className="send-btn" onClick={handleSend} disabled={!input.trim()}>
          <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/></svg>
        </button>
      </div>
      {showNewChat && renderNewChatModal()}
    </div>
  )
}
