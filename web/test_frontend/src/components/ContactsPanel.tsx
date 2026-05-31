import { useState, useEffect } from 'react'
import { api } from '../api/client.ts'
import { useApp } from '../store/AppContext.tsx'
import './ContactsPanel.css'

interface FriendReq {
  sender_account: string
  receiver_account: string
  message: string
  status: number
}

interface FriendGroup {
  group_id: number
  name: string
}

export default function ContactsPanel({ onSwitchToChat }: { onSwitchToChat: () => void }) {
  const { openChatWith, auth, setFriends, friends, memberInfo, loadConversationMembers } = useApp()
  const [requests, setRequests] = useState<FriendReq[]>([])
  const [tab, setTab] = useState<'list' | 'req' | 'add' | 'group'>('list')
  const [searchAccount, setSearchAccount] = useState('')
  const [searchResult, setSearchResult] = useState<{ account: string; name: string } | null>(null)
  const [addMsg, setAddMsg] = useState('')
  const [groupName, setGroupName] = useState('')
  const [friendGroups, setFriendGroups] = useState<FriendGroup[]>([])
  const [movingFriendAccount, setMovingFriendAccount] = useState<string | null>(null)
  const [editingRemarkAccount, setEditingRemarkAccount] = useState<string | null>(null)
  const [remarkInput, setRemarkInput] = useState('')
  const [toast, setToast] = useState('')

  const showToast = (msg: string) => {
    setToast(msg)
    setTimeout(() => setToast(''), 3000)
  }

  const loadFriends = async () => {
    const res = await api('GET', '/api/get_friend_list')
    if (res.code === 0) setFriends(res.data || [])
  }

  const loadFriendGroups = async () => {
    const res = await api('GET', '/api/get_friend_groups')
    if (res.code === 0) setFriendGroups(res.data || [])
  }

  const loadRequests = async () => {
    const res = await api('GET', '/api/get_friend_requests')
    if (res.code === 0) setRequests(res.data || [])
  }

  useEffect(() => { loadFriends(); loadFriendGroups() }, [])

  useEffect(() => {
    const accounts = friends.map(f => f.friend_account).filter(a => a)
    if (accounts.length > 0) {
      loadConversationMembers(accounts)
    }
  }, [friends, loadConversationMembers])

  const handleSearchUser = async () => {
    if (!searchAccount) return
    const res = await api('GET', `/api/search_user?account=${searchAccount}`)
    if (res.code === 0) setSearchResult(res.data)
    else { setSearchResult(null); showToast(res.msg || '搜索失败') }
  }

  const handleAddFriend = async () => {
    if (!searchResult) return
    const res = await api('POST', '/api/add_friend', { receiver_account: searchResult.account, message: addMsg })
    showToast(res.code === 0 ? '好友请求已发送' : res.msg || '发送失败')
    if (res.code === 0) { setSearchAccount(''); setAddMsg(''); setSearchResult(null) }
  }

  const handleFriendReq = async (senderAccount: string, accept: boolean) => {
    const res = await api('POST', '/api/handle_friend_req', { sender_account: senderAccount, accept })
    showToast(res.code === 0 ? (accept ? '已添加好友' : '已拒绝') : res.msg || '操作失败')
    if (res.code === 0) { loadRequests(); loadFriends() }
  }

  const handleCreateGroup = async () => {
    if (!groupName) return
    const res = await api('POST', '/api/create_friend_group', { name: groupName })
    showToast(res.code === 0 ? '分组创建成功' : res.msg || '创建失败')
    if (res.code === 0) { setGroupName(''); loadFriendGroups() }
  }

  const handleMoveFriend = async (friendAccount: string, groupId: number) => {
    const res = await api('POST', '/api/move_friend_to_group', { friend_account: friendAccount, group_id: groupId })
    showToast(res.code === 0 ? '移动成功' : res.msg || '移动失败')
    if (res.code === 0) { setMovingFriendAccount(null); loadFriends() }
  }

  const handleDeleteGroup = async (groupId: number) => {
    const res = await api('POST', '/api/delete_friend_group', { group_id: groupId })
    showToast(res.code === 0 ? '分组已删除' : res.msg || '删除失败')
    if (res.code === 0) { loadFriendGroups(); loadFriends() }
  }

  const handleUpdateRemark = async (friendAccount: string, remark: string) => {
    const res = await api('POST', '/api/update_friend_remark', { friend_account: friendAccount, remark })
    showToast(res.code === 0 ? '备注修改成功' : res.msg || '修改失败')
    if (res.code === 0) { setEditingRemarkAccount(null); setRemarkInput(''); loadFriends() }
  }

  const startEditRemark = (friendAccount: string, currentRemark: string) => {
    setEditingRemarkAccount(friendAccount)
    setRemarkInput(currentRemark)
    setMovingFriendAccount(null)
  }

  const startChat = (friendAccount: string, name: string) => {
    openChatWith(friendAccount, name, 1)
    onSwitchToChat()
  }

  const getGroupName = (gid: number) => {
    if (gid === 0) return '我的好友'
    const g = friendGroups.find(g => g.group_id === gid)
    return g ? g.name : `分组 ${gid}`
  }

  const grouped: Record<number, typeof friends> = {}
  friends.forEach(f => {
    const gid = f.group_id || 0
    if (!grouped[gid]) grouped[gid] = []
    grouped[gid].push(f)
  })

  return (
    <div className="contacts-panel">
      <div className="contacts-header">
        <h3>联系人</h3>
      </div>
      <div className="contacts-tabs">
        <button className={`ctab ${tab === 'list' ? 'active' : ''}`} onClick={() => { setTab('list'); loadFriends() }}>好友</button>
        <button className={`ctab ${tab === 'req' ? 'active' : ''}`} onClick={() => { setTab('req'); loadRequests() }}>请求</button>
        <button className={`ctab ${tab === 'add' ? 'active' : ''}`} onClick={() => setTab('add')}>添加</button>
        <button className={`ctab ${tab === 'group' ? 'active' : ''}`} onClick={() => { setTab('group'); loadFriendGroups() }}>分组</button>
      </div>
      <div className="contacts-content">
        {tab === 'list' && (
          <div className="friend-list">
            {Object.keys(grouped).sort((a, b) => Number(a) - Number(b)).map(gid => (
              <div key={gid} className="friend-group">
                <div className="friend-group-title">
                  {getGroupName(Number(gid))} <span className="count">{grouped[Number(gid)].length}</span>
                </div>
                {grouped[Number(gid)].map(f => (
                  <div key={f.friend_account} className="friend-item">
                    <div className="friend-main" onClick={() => startChat(f.friend_account, f.remark || f.name)}>
                      {memberInfo[f.friend_account]?.avatar ? (
                        <img src={memberInfo[f.friend_account].avatar} alt={f.name} className="friend-avatar-img" />
                      ) : (
                        <div className="friend-avatar">{(f.remark || f.name || '?')[0].toUpperCase()}</div>
                      )}
                      <div className="friend-info">
                        <div className="friend-name">{f.remark || f.name}</div>
                        {f.remark && <div className="friend-sub-name">昵称: {f.name}</div>}
                      </div>
                    </div>
                    <div className="friend-actions">
                      <button className="btn-icon" title="修改备注" onClick={() => startEditRemark(f.friend_account, f.remark)}>✏️</button>
                      <button className="btn-icon" title="移动到分组" onClick={() => setMovingFriendAccount(movingFriendAccount === f.friend_account ? null : f.friend_account)}>☰</button>
                    </div>
                    {editingRemarkAccount === f.friend_account && (
                      <div className="move-group-dropdown">
                        <div className="move-group-label">修改备注：</div>
                        <input
                          value={remarkInput}
                          onChange={e => setRemarkInput(e.target.value)}
                          placeholder={f.name}
                          onKeyDown={e => {
                            if (e.key === 'Enter') handleUpdateRemark(f.friend_account, remarkInput)
                            if (e.key === 'Escape') { setEditingRemarkAccount(null); setRemarkInput('') }
                          }}
                          autoFocus
                          style={{ width: '100%', padding: '4px 8px', fontSize: 13, border: '1px solid var(--border)', borderRadius: 4, marginTop: 4 }}
                        />
                        <div style={{ display: 'flex', gap: 8, marginTop: 6 }}>
                          <button className="btn-sm btn-accept" onClick={() => handleUpdateRemark(f.friend_account, remarkInput)}>保存</button>
                          <button className="btn-sm btn-reject" onClick={() => { setEditingRemarkAccount(null); setRemarkInput('') }}>取消</button>
                        </div>
                      </div>
                    )}
                    {movingFriendAccount === f.friend_account && editingRemarkAccount !== f.friend_account && (
                      <div className="move-group-dropdown">
                        <div className="move-group-label">移动到：</div>
                        <div className={`move-group-item ${f.group_id === 0 ? 'active' : ''}`} onClick={() => handleMoveFriend(f.friend_account, 0)}>我的好友</div>
                        {friendGroups.map(g => (
                          <div key={g.group_id} className={`move-group-item ${f.group_id === g.group_id ? 'active' : ''}`} onClick={() => handleMoveFriend(f.friend_account, g.group_id)}>
                            {g.name}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            ))}
            {friends.length === 0 && <div className="contacts-empty">暂无好友</div>}
          </div>
        )}
        {tab === 'req' && (
          <div className="req-list">
            {requests.map((r, i) => (
              <div key={i} className="req-item">
                <div className="req-info">
                  <span className="req-from">{r.sender_account}</span>
                  <span className="req-msg">{r.message}</span>
                  <span className={`req-status status-${r.status}`}>
                    {r.status === 0 ? '待处理' : r.status === 1 ? '已同意' : '已拒绝'}
                  </span>
                </div>
                {r.status === 0 && (
                  <div className="req-actions">
                    <button className="btn-sm btn-accept" onClick={() => handleFriendReq(r.sender_account, true)}>同意</button>
                    <button className="btn-sm btn-reject" onClick={() => handleFriendReq(r.sender_account, false)}>拒绝</button>
                  </div>
                )}
              </div>
            ))}
            {requests.length === 0 && <div className="contacts-empty">暂无好友请求</div>}
          </div>
        )}
        {tab === 'add' && (
          <div className="add-form">
            <div className="form-field">
              <label>搜索账号</label>
              <input value={searchAccount} onChange={e => setSearchAccount(e.target.value)} placeholder="输入对方账号" />
            </div>
            <button className="btn-primary full" onClick={handleSearchUser}>搜索</button>
            {searchResult && (
              <div className="group-info-card" style={{ marginTop: 12 }}>
                <div className="gi-row"><span>账号</span><span>{searchResult.account}</span></div>
                <div className="gi-row"><span>昵称</span><span>{searchResult.name}</span></div>
                <div className="form-field" style={{ marginTop: 8 }}>
                  <label>验证消息</label>
                  <input value={addMsg} onChange={e => setAddMsg(e.target.value)} placeholder="我是..." />
                </div>
                <button className="btn-primary full" style={{ marginTop: 8 }} onClick={handleAddFriend}>发送请求</button>
              </div>
            )}
          </div>
        )}
        {tab === 'group' && (
          <div className="add-form">
            <div className="form-field">
              <label>分组名称</label>
              <input value={groupName} onChange={e => setGroupName(e.target.value)} placeholder="输入分组名称" />
            </div>
            <button className="btn-primary full" onClick={handleCreateGroup}>创建分组</button>
            {friendGroups.length > 0 && (
              <div className="group-manage-list">
                <div className="group-manage-title">已有分组</div>
                {friendGroups.map(g => (
                  <div key={g.group_id} className="group-manage-item">
                    <span className="group-manage-name">{g.name}</span>
                    <button className="btn-sm btn-danger" onClick={() => handleDeleteGroup(g.group_id)}>删除</button>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
      {toast && <div className="toast">{toast}</div>}
    </div>
  )
}
