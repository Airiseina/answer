import { useState, useRef, useEffect } from 'react'
import { useApp } from '../store/AppContext'
import { api, uploadFile, parseMessageContent, buildTextContent, buildMediaContent } from '../api/client'
import './ChatPanel.css'

interface PendingAttachment {
  url: string
  mediaType: 'image' | 'voice' | 'file'
  fileName: string
  fileSize: number
}

export default function ChatPanel() {
  const { conversations, messages, activeConvId, sendMessage, setActiveConvId, friends, openChatWith, auth, addConversation, loadConversations, onlineStatus, loadOnlineStatus, memberInfo, loadConversationMembers, typingStatus, sendTyping, recallMessage, editMessage, systemNotification, clearSystemNotification } = useApp()
  const [input, setInput] = useState('')
  const [showNewChat, setShowNewChat] = useState(false)
  const [newChatType, setNewChatType] = useState<number>(1)
  const [newChatName, setNewChatName] = useState('')
  const [selectedFriendAccounts, setSelectedFriendAccounts] = useState<string[]>([])
  const [uploading, setUploading] = useState(false)
  const [pendingAttachment, setPendingAttachment] = useState<PendingAttachment | null>(null)
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; msgId: string; isSent: boolean; isRecalled: boolean } | null>(null)
  const [editingMsgId, setEditingMsgId] = useState<string | null>(null)
  const [editInput, setEditInput] = useState('')
  const [editHistoryModal, setEditHistoryModal] = useState<{ msgId: string; histories: any[] } | null>(null)
  const msgEndRef = useRef<HTMLDivElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const conv = conversations.find(c => c.id === activeConvId)
  const msgs = activeConvId ? (messages[activeConvId] || []) : []

  useEffect(() => {
    msgEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [msgs.length])

  const handleSend = () => {
    if (!activeConvId) return

    if (pendingAttachment) {
      const { url, mediaType, fileName, fileSize } = pendingAttachment
      let content: string
      if (mediaType === 'image') {
        content = buildMediaContent('image', url, { size: fileSize })
      } else if (mediaType === 'voice') {
        content = buildMediaContent('voice', url, { size: fileSize })
      } else {
        content = buildMediaContent('file', url, { filename: fileName, size: fileSize })
      }
      sendMessage(activeConvId, content)
      setPendingAttachment(null)

      if (input.trim()) {
        sendMessage(activeConvId, buildTextContent(input.trim()))
        setInput('')
      }
      return
    }

    if (input.trim()) {
      sendMessage(activeConvId, buildTextContent(input.trim()))
      setInput('')
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const handleFileSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file || !activeConvId) return
    setUploading(true)
    try {
      const res = await uploadFile(file)
      if (res.code !== 0 || !res.data) {
        alert(res.msg || '上传失败')
        return
      }
      const { url, media_type, file_name, file_size } = res.data
      setPendingAttachment({
        url,
        mediaType: media_type,
        fileName: file_name,
        fileSize: file_size,
      })
    } catch {
      alert('上传失败')
    } finally {
      setUploading(false)
      if (fileInputRef.current) fileInputRef.current.value = ''
    }
  }

  const handleSelectFriend = (friendAccount: string, name: string) => {
    openChatWith(friendAccount, name, 1)
    setShowNewChat(false)
  }

  const handleCreateGroupConversation = async () => {
    if (selectedFriendAccounts.length === 0) return
    const res = await api('POST', '/api/create_group', {
      name: newChatName || `群聊`,
      initial_accounts: selectedFriendAccounts,
    })
    if (res.code === 0 && res.data?.conversation_id) {
      const convId = String(res.data.conversation_id)
      const groupNumber = res.data.group_number
      addConversation({
        id: convId,
        name: newChatName || `群聊`,
        type: 2,
        memberAccounts: [auth.account, ...selectedFriendAccounts],
        groupNumber: groupNumber ? String(groupNumber) : undefined,
        unread: 0,
      })
      setActiveConvId(convId)
      loadConversations()
      setShowNewChat(false)
      setNewChatName('')
      setSelectedFriendAccounts([])
    }
  }

  const toggleFriend = (faccount: string) => {
    setSelectedFriendAccounts(prev =>
      prev.includes(faccount) ? prev.filter(a => a !== faccount) : [...prev, faccount]
    )
  }

  const formatTime = (ts: number) => {
    const d = new Date(ts * 1000)
    return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
  }

  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  }

  const handleContextMenu = (e: React.MouseEvent, msgId: string, isSent: boolean, isRecalled: boolean) => {
    e.preventDefault()
    setContextMenu({ x: e.clientX, y: e.clientY, msgId, isSent, isRecalled })
  }

  const handleRecall = () => {
    if (contextMenu && activeConvId) {
      recallMessage(activeConvId, contextMenu.msgId)
      setContextMenu(null)
    }
  }

  const handleStartEdit = (msgId: string, currentContent: string) => {
    const parsed = parseMessageContent(currentContent)
    setEditingMsgId(msgId)
    setEditInput(parsed.text || '')
    setContextMenu(null)
  }

  const handleSaveEdit = () => {
    if (editingMsgId && activeConvId && editInput.trim()) {
      editMessage(activeConvId, editingMsgId, editInput.trim())
      setEditingMsgId(null)
      setEditInput('')
    }
  }

  const handleCancelEdit = () => {
    setEditingMsgId(null)
    setEditInput('')
  }

  const handleViewEditHistory = async (msgId: string) => {
    if (!activeConvId) return
    const res = await api('POST', `/api/chat/edit_history/${msgId}`, { conversation_id: activeConvId })
    if (res.code === 0 && res.data?.histories) {
      setEditHistoryModal({ msgId, histories: res.data.histories })
    }
  }

  const renderMessageContent = (content: string) => {
    const parsed = parseMessageContent(content)
    switch (parsed.type) {
      case 'image':
        return (
          <div className="msg-image">
            <img src={parsed.url} alt="图片" loading="lazy" onClick={() => window.open(parsed.url, '_blank')} />
          </div>
        )
      case 'file':
        return (
          <div className="msg-file" onClick={() => window.open(parsed.url, '_blank')}>
            <div className="msg-file-icon">📎</div>
            <div className="msg-file-info">
              <div className="msg-file-name">{parsed.filename || '文件'}</div>
              <div className="msg-file-size">{parsed.size ? formatFileSize(parsed.size) : ''}</div>
            </div>
          </div>
        )
      case 'voice':
        return (
          <div className="msg-voice">
            <span className="msg-voice-icon">🎤</span>
            <span>{parsed.duration || 0}″</span>
            <audio controls src={parsed.url} style={{ height: 32, marginLeft: 8 }} />
          </div>
        )
      default:
        return <div className="msg-text">{parsed.text || content}</div>
    }
  }

  const renderPendingAttachment = () => {
    if (!pendingAttachment) return null
    return (
      <div className="pending-attachment">
        <div className="pending-preview">
          {pendingAttachment.mediaType === 'image' ? (
            <img src={pendingAttachment.url} className="pending-thumb" alt="预览" />
          ) : pendingAttachment.mediaType === 'voice' ? (
            <div className="pending-file-badge">🎤 {pendingAttachment.fileName}</div>
          ) : (
            <div className="pending-file-badge">📎 {pendingAttachment.fileName}</div>
          )}
        </div>
        <button className="pending-remove" onClick={() => setPendingAttachment(null)} title="移除附件">✕</button>
      </div>
    )
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
                  <div key={f.friend_account} className="friend-picker-item" onClick={() => handleSelectFriend(f.friend_account, f.remark || f.name)}>
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
                  <div key={f.friend_account}
                    className={`friend-picker-item ${selectedFriendAccounts.includes(f.friend_account) ? 'selected' : ''}`}
                    onClick={() => toggleFriend(f.friend_account)}
                  >
                    <div className="fp-avatar">{(f.remark || f.name || '?')[0].toUpperCase()}</div>
                    <div className="fp-info">
                      <div className="fp-name">{f.remark || f.name}</div>
                    </div>
                    {selectedFriendAccounts.includes(f.friend_account) && <div className="fp-check">✓</div>}
                  </div>
                ))}
              </div>
            )}
            {selectedFriendAccounts.length > 0 && (
              <div className="selected-count">已选 {selectedFriendAccounts.length} 人</div>
            )}
          </div>
        )}
        <div className="modal-actions">
          <button className="btn-cancel" onClick={() => setShowNewChat(false)}>取消</button>
          {newChatType === 2 && (
            <button className="btn-primary" onClick={handleCreateGroupConversation} disabled={selectedFriendAccounts.length === 0}>创建群聊</button>
          )}
        </div>
      </div>
    </div>
  )

  const peerAccount = conv?.type === 1 ? conv.memberAccounts.find(a => a !== auth.account) : undefined
  const isPeerOnline = peerAccount ? onlineStatus[peerAccount] === true : false

  const activeTypingUsers = activeConvId && typingStatus[activeConvId]
    ? Object.entries(typingStatus[activeConvId])
        .filter(([acc]) => acc !== auth.account && Date.now() - typingStatus[activeConvId][acc] < 5000)
        .map(([acc]) => {
          const friend = friends.find(f => f.friend_account === acc)
          return friend?.remark || friend?.name || acc
        })
    : []

  useEffect(() => {
    if (peerAccount) {
      loadOnlineStatus([peerAccount])
    }
  }, [peerAccount, loadOnlineStatus])

  useEffect(() => {
    if (conv?.memberAccounts && conv.memberAccounts.length > 0) {
      loadConversationMembers(conv.memberAccounts)
    }
  }, [activeConvId, conv?.memberAccounts, loadConversationMembers])

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
          {conv.type === 1 && (
            <span className={`chat-header-online ${isPeerOnline ? 'online' : 'offline'}`}>
              {isPeerOnline ? '在线' : '离线'}
            </span>
          )}
        </div>
        <button className="chat-new-btn" onClick={() => setShowNewChat(true)} title="新会话">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
        </button>
      </div>
      <div className="chat-messages" onClick={() => setContextMenu(null)}>
        {msgs.map(msg => (
          <div key={msg.id} className={`msg-row ${msg.isSent ? 'sent' : 'received'}`}
            onContextMenu={(e) => handleContextMenu(e, msg.id, msg.isSent, msg.status === 1)}
          >
            <div className="msg-avatar">
              {memberInfo[msg.from]?.avatar ? (
                <img src={memberInfo[msg.from].avatar} alt={msg.fromName} className="avatar-img" />
              ) : (
                <div className="avatar-placeholder">{(msg.fromName || msg.from || '?')[0].toUpperCase()}</div>
              )}
            </div>
            <div className="msg-bubble">
              {!msg.isSent && msg.fromName && (
                <div className="msg-sender-name">{msg.fromName}</div>
              )}
              {msg.status === 1 ? (
                <div className="msg-recalled">
                  {msg.isSent ? '你撤回了一条消息' : `${msg.fromName || '对方'}撤回了一条消息`}
                </div>
              ) : editingMsgId === msg.id ? (
                <div className="msg-edit-area">
                  <textarea
                    value={editInput}
                    onChange={e => setEditInput(e.target.value)}
                    onKeyDown={e => {
                      if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSaveEdit() }
                      if (e.key === 'Escape') handleCancelEdit()
                    }}
                    rows={2}
                    autoFocus
                  />
                  <div className="msg-edit-actions">
                    <button className="edit-save-btn" onClick={handleSaveEdit}>保存</button>
                    <button className="edit-cancel-btn" onClick={handleCancelEdit}>取消</button>
                  </div>
                </div>
              ) : (
                <>
                  {renderMessageContent(msg.content)}
                  {msg.isEdited && (
                    <div className="msg-edited-tag" onClick={() => handleViewEditHistory(msg.id)} style={{ cursor: 'pointer' }}>
                      已编辑
                    </div>
                  )}
                </>
              )}
              {msg.status !== 1 && (
                <div className="msg-time">{formatTime(msg.time)}</div>
              )}
            </div>
          </div>
        ))}
        <div ref={msgEndRef} />
      </div>
      {contextMenu && (
        <div className="context-menu" style={{ left: contextMenu.x, top: contextMenu.y }}
          onClick={e => e.stopPropagation()}
        >
          {contextMenu.isSent && !contextMenu.isRecalled && (
            <div className="context-menu-item" onClick={handleRecall}>撤回</div>
          )}
          {contextMenu.isSent && !contextMenu.isRecalled && (
            <div className="context-menu-item" onClick={() => {
              const m = msgs.find(m => m.id === contextMenu.msgId)
              if (m) handleStartEdit(m.id, m.content)
            }}>编辑</div>
          )}
          {(() => {
            const m = msgs.find(m => m.id === contextMenu.msgId)
            return m?.isEdited ? (
              <div className="context-menu-item" onClick={() => {
                handleViewEditHistory(contextMenu.msgId)
                setContextMenu(null)
              }}>查看编辑历史</div>
            ) : null
          })()}
        </div>
      )}
      {editHistoryModal && (
        <div className="modal-overlay" onClick={() => setEditHistoryModal(null)}>
          <div className="modal-card" onClick={e => e.stopPropagation()}>
            <div className="modal-card-header">
              <h3>编辑历史</h3>
              <button className="modal-card-close" onClick={() => setEditHistoryModal(null)}>✕</button>
            </div>
            <div className="edit-history-list">
              {editHistoryModal.histories.length === 0 ? (
                <div className="edit-history-empty">暂无编辑历史</div>
              ) : (
                editHistoryModal.histories.map((h: any) => {
                  const parsed = parseMessageContent(h.old_content)
                  const timeSec = h.edited_at > 1e12 ? h.edited_at / 1000 : h.edited_at
                  return (
                    <div key={h.id} className="edit-history-item">
                      <div className="edit-history-meta">
                        版本 {h.version} · {new Date(timeSec * 1000).toLocaleString()}
                      </div>
                      <div className="edit-history-content">
                        {parsed.type === 'text' ? parsed.text : parsed.type === 'image' ? '[图片]' : parsed.type === 'file' ? `[文件] ${parsed.filename || ''}` : parsed.type === 'voice' ? '[语音]' : h.old_content}
                      </div>
                    </div>
                  )
                })
              )}
            </div>
          </div>
        </div>
      )}
      {activeTypingUsers.length > 0 && (
        <div className="typing-indicator">
          <span className="typing-dots">
            <span className="typing-dot" />
            <span className="typing-dot" />
            <span className="typing-dot" />
          </span>
          <span className="typing-text">
            {activeTypingUsers.length === 1
              ? `${activeTypingUsers[0]} 正在输入...`
              : `${activeTypingUsers.length} 人正在输入...`}
          </span>
        </div>
      )}
      {systemNotification && (
        <div className="system-notification" onClick={clearSystemNotification}>
          {systemNotification}
        </div>
      )}
      <div className="chat-input-area">
        <div className="chat-toolbar">
          <button
            className="toolbar-btn"
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading}
            title="发送文件/图片"
          >
            {uploading ? '⏳' : '📎'}
          </button>
          <input
            ref={fileInputRef}
            type="file"
            style={{ display: 'none' }}
            accept="image/*,audio/*,.pdf,.doc,.docx,.xls,.xlsx,.zip,.rar,.txt"
            onChange={handleFileSelect}
          />
        </div>
        {renderPendingAttachment()}
        <div className="chat-input-row">
          <textarea
            value={input}
            onChange={e => {
              setInput(e.target.value)
              if (activeConvId && e.target.value.trim()) {
                sendTyping(activeConvId)
              }
            }}
            onKeyDown={handleKeyDown}
            placeholder={pendingAttachment ? '添加消息说明（可选），Enter 发送...' : '输入消息，Enter 发送...'}
            rows={3}
          />
          <button className="send-btn" onClick={handleSend} disabled={!input.trim() && !pendingAttachment}>
            <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/></svg>
          </button>
        </div>
      </div>
      {showNewChat && renderNewChatModal()}
    </div>
  )
}
