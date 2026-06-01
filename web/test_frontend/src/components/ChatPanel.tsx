import { useState, useRef, useEffect } from 'react'
import { useApp } from '../store/AppContext.tsx'
import { api, uploadFile, parseMessageContent, buildTextContent, buildMediaContent, type MentionItem } from '../api/client.ts'
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
  const [bots, setBots] = useState<{ bot_id: string; name: string; is_system: boolean }[]>([])
  const [selectedBotIds, setSelectedBotIds] = useState<string[]>([])
  const [uploading, setUploading] = useState(false)
  const [pendingAttachment, setPendingAttachment] = useState<PendingAttachment | null>(null)
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; msgId: string; isSent: boolean; isRecalled: boolean; senderAccount?: string } | null>(null)
  const [editingMsgId, setEditingMsgId] = useState<string | null>(null)
  const [editInput, setEditInput] = useState('')
  const [editHistoryModal, setEditHistoryModal] = useState<{ msgId: string; histories: any[] } | null>(null)
  const [mentionPickerVisible, setMentionPickerVisible] = useState(false)
  const [mentionSearch, setMentionSearch] = useState('')
  const [mentionStartPos, setMentionStartPos] = useState(-1)
  const [pendingMentions, setPendingMentions] = useState<MentionItem[]>([])
  const [summaryLoading, setSummaryLoading] = useState(false)
  const [summaryModal, setSummaryModal] = useState<string | null>(null)
  const [repliesLoading, setRepliesLoading] = useState(false)
  const [suggestedReplies, setSuggestedReplies] = useState<string[] | null>(null)
  const [quotingMsg, setQuotingMsg] = useState<{ id: string; fromName: string; content: string } | null>(null)
  const [translatingMsgId, setTranslatingMsgId] = useState<string | null>(null)
  const [translatedMsgs, setTranslatedMsgs] = useState<Record<string, { content: string; lang: string }>>({})
  const [translatePickerMsgId, setTranslatePickerMsgId] = useState<string | null>(null)
  const msgEndRef = useRef<HTMLDivElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const conv = conversations.find(c => c.id === activeConvId)
  const msgs = activeConvId !== null ? (messages[activeConvId] || []) : []

  useEffect(() => {
    msgEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [msgs.length])

  const handleSend = () => {
    if (activeConvId === null) return

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
        const mentionsToSend = pendingMentions.length > 0 ? pendingMentions : undefined
        sendMessage(activeConvId, buildTextContent(input.trim(), mentionsToSend), mentionsToSend?.map(m => m.user_id), quotingMsg?.id)
        setInput('')
        setPendingMentions([])
        setQuotingMsg(null)
      }
      return
    }

    if (input.trim()) {
      const mentionsToSend = pendingMentions.length > 0 ? pendingMentions : undefined
      sendMessage(activeConvId, buildTextContent(input.trim(), mentionsToSend), mentionsToSend?.map(m => m.user_id), quotingMsg?.id)
      setInput('')
      setPendingMentions([])
      setMentionPickerVisible(false)
      setMentionStartPos(-1)
      setQuotingMsg(null)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (mentionPickerVisible && e.key === 'Escape') {
      e.preventDefault()
      setMentionPickerVisible(false)
      setMentionStartPos(-1)
      return
    }
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const handleFileSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file || activeConvId === null) return
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
    if (selectedFriendAccounts.length === 0 && selectedBotIds.length === 0) return
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
      if (selectedBotIds.length > 0) {
        for (const botId of selectedBotIds) {
          await api('POST', '/api/bot/add_to_conversation', {
            bot_id: botId,
            conversation_id: convId,
            conversation_type: 2,
          })
        }
      }
      setShowNewChat(false)
      setNewChatName('')
      setSelectedFriendAccounts([])
      setSelectedBotIds([])
    }
  }

  const toggleFriend = (faccount: string) => {
    setSelectedFriendAccounts(prev =>
      prev.includes(faccount) ? prev.filter(a => a !== faccount) : [...prev, faccount]
    )
  }

  const loadBots = async () => {
    const res = await api('GET', '/api/bot/list')
    if (res.code === 0) setBots(res.data?.bots || [])
    else setBots([])
  }

  useEffect(() => {
    loadBots()
  }, [])

  const formatTime = (ts: number) => {
    const d = new Date(ts * 1000)
    return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
  }

  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  }

  const handleContextMenu = (e: React.MouseEvent, msgId: string, isSent: boolean, isRecalled: boolean, senderAccount?: string) => {
    e.preventDefault()
    setContextMenu({ x: e.clientX, y: e.clientY, msgId, isSent, isRecalled, senderAccount })
  }

  const handleRecall = () => {
    if (contextMenu && activeConvId !== null) {
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
    if (editingMsgId && activeConvId !== null && editInput.trim()) {
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
    if (activeConvId === null) return
    const res = await api('POST', `/api/chat/edit_history/${msgId}`, { conversation_id: activeConvId })
    if (res.code === 0 && res.data?.histories) {
      setEditHistoryModal({ msgId, histories: res.data.histories })
    }
  }

  const handleSummarize = async () => {
    if (activeConvId === null) return
    setSummaryLoading(true)
    try {
      const res = await api('POST', '/api/chat/summarize', { conversation_id: activeConvId })
      if (res.code === 0 && res.data?.summary) {
        setSummaryModal(res.data.summary)
      } else {
        alert(res.msg || '生成总结失败')
      }
    } catch {
      alert('生成总结失败')
    } finally {
      setSummaryLoading(false)
    }
  }

  const handleSuggestReplies = async () => {
    if (activeConvId === null) return
    setRepliesLoading(true)
    try {
      const res = await api('POST', '/api/chat/suggest_replies', { conversation_id: activeConvId })
      if (res.code === 0 && res.data?.replies) {
        setSuggestedReplies(res.data.replies)
      } else {
        alert(res.msg || '生成回复候选失败')
      }
    } catch {
      alert('生成回复候选失败')
    } finally {
      setRepliesLoading(false)
    }
  }

  const handleSelectReply = (reply: string) => {
    setInput(reply)
    setSuggestedReplies(null)
    setTimeout(() => textareaRef.current?.focus(), 0)
  }

  const LANGUAGES = [
    { code: '中文', label: '中文' },
    { code: '英语', label: 'English' },
    { code: '日语', label: '日本語' },
    { code: '韩语', label: '한국어' },
    { code: '法语', label: 'Français' },
    { code: '德语', label: 'Deutsch' },
    { code: '西班牙语', label: 'Español' },
    { code: '俄语', label: 'Русский' },
  ]

  const handleTranslate = async (msgId: string, content: string, targetLang: string) => {
    setTranslatePickerMsgId(null)
    setTranslatingMsgId(msgId)
    try {
      const parsed = parseMessageContent(content)
      const textToTranslate = parsed.text || content
      const res = await api('POST', '/api/chat/translate', { content: textToTranslate, target_lang: targetLang })
      if (res.code === 0 && res.data?.translated_content) {
        const translated = res.data.translated_content.trim()
        if (translated && translated !== textToTranslate) {
          setTranslatedMsgs(prev => ({ ...prev, [msgId]: { content: translated, lang: targetLang } }))
        }
      }
    } catch { }
    setTranslatingMsgId(null)
  }

  const renderTextWithMentions = (text: string, mentions?: MentionItem[]) => {
    if (!mentions || mentions.length === 0) {
      return text
    }
    const parts: React.ReactNode[] = []
    let remaining = text
    for (const mention of mentions) {
      const mentionTag = `@${mention.name}`
      const idx = remaining.indexOf(mentionTag)
      if (idx === -1) continue
      if (idx > 0) {
        parts.push(remaining.substring(0, idx))
      }
      parts.push(
        <span key={`mention-${mention.user_id}-${idx}`} className="mention-tag">
          {mentionTag}
        </span>
      )
      remaining = remaining.substring(idx + mentionTag.length)
    }
    if (remaining) {
      parts.push(remaining)
    }
    return parts.length > 0 ? parts : text
  }

  const getMentionCandidates = () => {
    if (!conv) return []
    const myAccount = auth.account
    const candidates: { account: string; name: string; userId: string }[] = []
    const seen = new Set<string>()
    for (const account of conv.memberAccounts) {
      if (account === myAccount) continue
      if (seen.has(account)) continue
      seen.add(account)
      const info = memberInfo[account]
      const name = info?.name || account
      candidates.push({ account, name, userId: account })
    }
    for (const f of friends) {
      if (seen.has(f.friend_account)) continue
      if (conv.type === 2) continue
      seen.add(f.friend_account)
      const name = f.remark || f.name
      candidates.push({ account: f.friend_account, name, userId: f.friend_account })
    }
    return candidates
  }

  const handleInputChange = (value: string) => {
    setInput(value)
    if (activeConvId && value.trim()) {
      sendTyping(activeConvId)
    }
    if (textareaRef.current) {
      const cursorPos = textareaRef.current.selectionStart
      const textBeforeCursor = value.substring(0, cursorPos)
      const atMatch = textBeforeCursor.match(/@([^\s@]*)$/)
      if (atMatch) {
        setMentionPickerVisible(true)
        setMentionSearch(atMatch[1])
        setMentionStartPos(cursorPos - atMatch[0].length)
      } else {
        setMentionPickerVisible(false)
        setMentionStartPos(-1)
      }
    }
  }

  const handleMentionSelect = (candidate: { account: string; name: string; userId: string }) => {
    if (mentionStartPos === -1) return
    const before = input.substring(0, mentionStartPos)
    const after = input.substring(textareaRef.current?.selectionStart || input.length)
    const newText = `${before}@${candidate.name} ${after}`
    setInput(newText)
    setMentionPickerVisible(false)
    setMentionStartPos(-1)
    setMentionSearch('')
    setPendingMentions(prev => {
      if (prev.find(m => m.user_id === candidate.userId)) return prev
      return [...prev, { user_id: candidate.userId, name: candidate.name }]
    })
    setTimeout(() => textareaRef.current?.focus(), 0)
  }

  const renderMentionPicker = () => {
    const candidates = getMentionCandidates()
    const filtered = mentionSearch
      ? candidates.filter(c => c.name.toLowerCase().includes(mentionSearch.toLowerCase()) || c.account.toLowerCase().includes(mentionSearch.toLowerCase()))
      : candidates
    if (filtered.length === 0) return null
    return (
      <div className="mention-picker">
        {filtered.map(c => (
          <div key={c.account} className="mention-picker-item" onClick={() => handleMentionSelect(c)}>
            <div className="mp-avatar">{c.name[0]?.toUpperCase() || '?'}</div>
            <div className="mp-info">
              <div className="mp-name">{c.name}</div>
              <div className="mp-account">{c.account}</div>
            </div>
          </div>
        ))}
      </div>
    )
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
        return (
          <div className="msg-text">
            {renderTextWithMentions(parsed.text || content, parsed.mentions)}
          </div>
        )
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
            {friends.length > 0 && (
              <>
                <label style={{ marginTop: 8 }}>选择好友拉入群聊</label>
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
              </>
            )}
            {bots.length > 0 && (
              <>
                <label style={{ marginTop: 8 }}>选择 Bot 拉入群聊</label>
                <div className="friend-picker-list">
                  {bots.map(b => (
                    <div key={b.bot_id}
                      className={`friend-picker-item ${selectedBotIds.includes(b.bot_id) ? 'selected' : ''}`}
                      onClick={() => setSelectedBotIds(prev => prev.includes(b.bot_id) ? prev.filter(id => id !== b.bot_id) : [...prev, b.bot_id])}
                    >
                      <div className="fp-avatar">🤖</div>
                      <div className="fp-info">
                        <div className="fp-name">{b.name}{b.is_system ? ' (系统)' : ''}</div>
                      </div>
                      {selectedBotIds.includes(b.bot_id) && <div className="fp-check">✓</div>}
                    </div>
                  ))}
                </div>
              </>
            )}
            {friends.length === 0 && bots.length === 0 && (
              <div className="picker-empty">暂无好友或 Bot</div>
            )}
            {(selectedFriendAccounts.length > 0 || selectedBotIds.length > 0) && (
              <div className="selected-count">已选 {selectedFriendAccounts.length} 位好友{selectedBotIds.length > 0 ? ` + ${selectedBotIds.length} 个Bot` : ''}</div>
            )}
          </div>
        )}
        <div className="modal-actions">
          <button className="btn-cancel" onClick={() => setShowNewChat(false)}>取消</button>
          {newChatType === 2 && (
            <button className="btn-primary" onClick={handleCreateGroupConversation} disabled={selectedFriendAccounts.length === 0 && selectedBotIds.length === 0}>创建群聊</button>
          )}
        </div>
      </div>
    </div>
  )

  const peerAccount = conv?.type === 1 ? (conv.peerAccount || conv.memberAccounts.find(a => a !== auth.account)) : undefined
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
        <div className="chat-header-actions">
          <button className="chat-header-action-btn" onClick={handleSummarize} disabled={summaryLoading} title="总结聊天">
            {summaryLoading ? '⏳' : '📋'}
          </button>
          <button className="chat-header-action-btn" onClick={handleSuggestReplies} disabled={repliesLoading} title="回复候选">
            {repliesLoading ? '⏳' : '💬'}
          </button>
          <button className="chat-new-btn" onClick={() => setShowNewChat(true)} title="新会话">
            <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
          </button>
        </div>
      </div>
      <div className="chat-messages" onClick={() => { setContextMenu(null); setTranslatePickerMsgId(null) }}>
        {msgs.map(msg => (
          <div key={msg.id} className={`msg-row ${msg.isSent ? 'sent' : 'received'}`} data-msg-id={msg.id}
            onContextMenu={(e) => handleContextMenu(e, msg.id, msg.isSent, msg.status === 1, msg.from)}
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
                  {msg.quoteMsgId && (() => {
                    const quotedMsg = msgs.find(m => m.id === msg.quoteMsgId)
                    if (quotedMsg) {
                      const parsed = parseMessageContent(quotedMsg.content)
                      return (
                        <div className="msg-quote-card" onClick={() => {
                          const el = document.querySelector(`[data-msg-id="${quotedMsg.id}"]`)
                          el?.scrollIntoView({ behavior: 'smooth', block: 'center' })
                          el?.classList.add('msg-highlight')
                          setTimeout(() => el?.classList.remove('msg-highlight'), 1500)
                        }}>
                          <div className="msg-quote-sender">{quotedMsg.fromName || '未知'}</div>
                          <div className="msg-quote-content">{parsed.text || quotedMsg.content}</div>
                        </div>
                      )
                    }
                    return null
                  })()}
                  {renderMessageContent(msg.content)}
                  {msg.isEdited && (
                    <div className="msg-edited-tag" onClick={() => handleViewEditHistory(msg.id)} style={{ cursor: 'pointer' }}>
                      已编辑
                    </div>
                  )}
                </>
              )}
              {msg.status !== 1 && (
                <div className="msg-actions-row">
                  <div className="msg-time">{formatTime(msg.time)}</div>
                  {(() => {
                    const parsed = parseMessageContent(msg.content)
                    return parsed.type === 'text' && parsed.text && (
                      <button className="msg-translate-btn" onClick={(e) => {
                        e.stopPropagation()
                        setTranslatePickerMsgId(translatePickerMsgId === msg.id ? null : msg.id)
                      }} title="翻译">🌐</button>
                    )
                  })()}
                </div>
              )}
              {translatePickerMsgId === msg.id && (
                <div className="translate-lang-picker" onClick={e => e.stopPropagation()}>
                  {LANGUAGES.map(lang => (
                    <button key={lang.code} className="translate-lang-item" onClick={() => handleTranslate(msg.id, msg.content, lang.code)}>
                      {lang.label}
                    </button>
                  ))}
                </div>
              )}
              {translatingMsgId === msg.id && (
                <div className="msg-translating">翻译中...</div>
              )}
              {translatedMsgs[msg.id] && (
                <div className="msg-translated">
                  <div className="msg-translated-lang">{translatedMsgs[msg.id].lang}</div>
                  <div className="msg-translated-content">{translatedMsgs[msg.id].content}</div>
                </div>
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
          {!contextMenu.isRecalled && (
            <div className="context-menu-item" onClick={() => {
              const m = msgs.find(m => m.id === contextMenu.msgId)
              if (m) {
                const parsed = parseMessageContent(m.content)
                setQuotingMsg({ id: m.id, fromName: m.fromName, content: parsed.text || m.content })
              }
              setContextMenu(null)
              setTimeout(() => textareaRef.current?.focus(), 0)
            }}>引用</div>
          )}
          {!contextMenu.isRecalled && (contextMenu.isSent || (conv?.type === 2)) && (
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
        {quotingMsg && (
          <div className="quote-preview-bar">
            <div className="quote-preview-content">
              <div className="quote-preview-label">回复 {quotingMsg.fromName}</div>
              <div className="quote-preview-text">{quotingMsg.content.length > 60 ? quotingMsg.content.slice(0, 60) + '...' : quotingMsg.content}</div>
            </div>
            <button className="quote-preview-close" onClick={() => setQuotingMsg(null)}>✕</button>
          </div>
        )}
        {suggestedReplies && (
          <div className="suggest-replies-bar">
            <span className="suggest-replies-label">回复候选：</span>
            <div className="suggest-replies-list">
              {suggestedReplies.map((reply, idx) => (
                <button key={idx} className="suggest-reply-item" onClick={() => handleSelectReply(reply)}>
                  {reply}
                </button>
              ))}
            </div>
            <button className="suggest-replies-close" onClick={() => setSuggestedReplies(null)}>✕</button>
          </div>
        )}
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
        {mentionPickerVisible && renderMentionPicker()}
        <div className="chat-input-row">
          <textarea
            ref={textareaRef}
            value={input}
            onChange={e => handleInputChange(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={pendingAttachment ? '添加消息说明（可选），Enter 发送...' : '输入消息，Enter 发送... @提及成员'}
            rows={3}
          />
          <button className="send-btn" onClick={handleSend} disabled={!input.trim() && !pendingAttachment}>
            <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/></svg>
          </button>
        </div>
      </div>
      {summaryModal && (
        <div className="modal-overlay" onClick={() => setSummaryModal(null)}>
          <div className="modal-card" onClick={e => e.stopPropagation()} style={{ width: 460 }}>
            <div className="modal-card-header">
              <h3>聊天总结</h3>
              <button className="modal-card-close" onClick={() => setSummaryModal(null)}>✕</button>
            </div>
            <div className="summary-content">{summaryModal}</div>
          </div>
        </div>
      )}
      {showNewChat && renderNewChatModal()}
    </div>
  )
}
