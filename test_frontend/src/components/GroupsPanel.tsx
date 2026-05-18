import { useState } from 'react'
import { api } from '../api/client'
import { useApp } from '../store/AppContext'
import './GroupsPanel.css'

interface GroupInfo {
  group_id: number
  name: string
  owner_id: number
  notice: string
  group_number: string
  create_time: number
  members: GroupMemberInfo[]
}

interface GroupMemberInfo {
  group_id: number
  user_id: number
  role: number
  is_muted: boolean
  join_time: number
  name: string
}

interface UserGroup {
  group_id: number
  name: string
  group_number: string
}

interface GroupSearchResult {
  group_id: number
  name: string
  owner_name: string
  group_number: string
}

interface JoinRequestInfo {
  user_id: number
  name: string
  message: string
  status: number
}

export default function GroupsPanel() {
  const { openChatWith, friends, auth } = useApp()
  const currentUserId = auth.userId ? parseInt(auth.userId) : 0
  const [tab, setTab] = useState<'create' | 'list' | 'search' | 'info'>('list')
  const [createName, setCreateName] = useState('')
  const [selectedFriendIds, setSelectedFriendIds] = useState<number[]>([])
  const [myGroups, setMyGroups] = useState<UserGroup[]>([])
  const [searchNumber, setSearchNumber] = useState('')
  const [searchResult, setSearchResult] = useState<GroupSearchResult | null>(null)
  const [joinMessage, setJoinMessage] = useState('')
  const [groupInfo, setGroupInfo] = useState<GroupInfo | null>(null)
  const [toast, setToast] = useState('')
  const [manageMode, setManageMode] = useState<string | null>(null)
  const [joinRequests, setJoinRequests] = useState<JoinRequestInfo[]>([])

  const showToast = (msg: string) => {
    setToast(msg)
    setTimeout(() => setToast(''), 3000)
  }

  const handleCreate = async () => {
    if (!createName) return
    const res = await api('POST', '/api/create_group', { name: createName, initial_members: selectedFriendIds })
    showToast(res.code === 0 ? `群组创建成功，群号: ${res.data?.group_number || ''}` : res.msg || '创建失败')
    if (res.code === 0) { setCreateName(''); setSelectedFriendIds([]) }
  }

  const toggleFriend = (fid: number) => {
    setSelectedFriendIds(prev =>
      prev.includes(fid) ? prev.filter(id => id !== fid) : [...prev, fid]
    )
  }

  const loadMyGroups = async () => {
    const res = await api('GET', '/api/get_user_groups')
    if (res.code === 0) setMyGroups(res.data || [])
    else showToast(res.msg || '获取失败')
  }

  const handleSearchGroup = async () => {
    if (!searchNumber) return
    const res = await api('GET', `/api/search_group?group_number=${searchNumber}`)
    if (res.code === 0) setSearchResult(res.data)
    else { setSearchResult(null); showToast(res.msg || '搜索失败') }
  }

  const handleJoinGroup = async () => {
    if (!searchResult) return
    const res = await api('POST', '/api/join_group', { group_number: searchNumber, message: joinMessage })
    showToast(res.code === 0 ? '申请已发送' : res.msg || '申请失败')
    if (res.code === 0) setJoinMessage('')
  }

  const loadGroupInfo = async (groupId: number) => {
    const res = await api('GET', `/api/get_group_info?group_id=${groupId}`)
    if (res.code === 0) { setGroupInfo(res.data); setManageMode(null) }
    else showToast(res.msg || '获取失败')
  }

  const loadJoinRequests = async (groupId: number) => {
    const res = await api('GET', `/api/get_join_requests?group_id=${groupId}`)
    if (res.code === 0) setJoinRequests(res.data || [])
    else showToast(res.msg || '获取失败')
  }

  const handleInvite = async (groupId: number, userIds: number[]) => {
    const res = await api('POST', '/api/invite_members', { group_id: groupId, user_ids: userIds })
    showToast(res.code === 0 ? '邀请成功' : res.msg || '邀请失败')
    if (res.code === 0) { loadGroupInfo(groupId); setSelectedInviteFriends([]); setManageMode(null) }
  }

  const handleKick = async (groupId: number, userId: number) => {
    const res = await api('POST', '/api/kick_members', { group_id: groupId, user_ids: [userId] })
    showToast(res.code === 0 ? '已踢出' : res.msg || '操作失败')
    if (res.code === 0) loadGroupInfo(groupId)
  }

  const handleChangeOwner = async (groupId: number, newId: number) => {
    const res = await api('POST', '/api/change_owner', { group_id: groupId, new_id: newId })
    showToast(res.code === 0 ? '转让成功' : res.msg || '操作失败')
    if (res.code === 0) loadGroupInfo(groupId)
  }

  const handleChangeNotice = async (groupId: number, notice: string) => {
    if (!notice.trim()) return
    const res = await api('POST', '/api/change_notice', { group_id: groupId, notice })
    showToast(res.code === 0 ? '公告修改成功' : res.msg || '操作失败')
    if (res.code === 0) { loadGroupInfo(groupId); setNoticeInput('') }
  }

  const handleMuted = async (groupId: number, mutedId: number, isMuted: boolean) => {
    const res = await api('POST', '/api/muted', { group_id: groupId, muted_id: mutedId, is_muted: isMuted })
    showToast(res.code === 0 ? '禁言状态修改成功' : res.msg || '操作失败')
    if (res.code === 0) loadGroupInfo(groupId)
  }

  const handleSetAdmin = async (groupId: number, targetId: number, role: number) => {
    const res = await api('POST', '/api/set_admin', { group_id: groupId, target_id: targetId, role })
    showToast(res.code === 0 ? '管理员设置修改成功' : res.msg || '操作失败')
    if (res.code === 0) loadGroupInfo(groupId)
  }

  const handleJoinReq = async (groupId: number, userId: number, accept: boolean) => {
    const res = await api('POST', '/api/handle_join_req', { group_id: groupId, user_id: userId, accept })
    showToast(res.code === 0 ? '处理成功' : res.msg || '操作失败')
    if (res.code === 0) loadJoinRequests(groupId)
  }

  const startGroupChat = (gid: number, name: string) => {
    openChatWith(gid, name || `群组 ${gid}`, 2)
  }

  const getRoleLabel = (role: number) => {
    if (role === 2) return '群主'
    if (role === 1) return '管理员'
    return '成员'
  }

  const isOwnerOrAdmin = (group: GroupInfo) => {
    if (!currentUserId) return false
    if (group.owner_id === currentUserId) return true
    const member = group.members?.find((m: GroupMemberInfo) => m.user_id === currentUserId)
    return member?.role === 1
  }

  const isOwner = (group: GroupInfo) => {
    if (!currentUserId) return false
    return group.owner_id === currentUserId
  }

  const [noticeInput, setNoticeInput] = useState('')
  const [selectedInviteFriends, setSelectedInviteFriends] = useState<number[]>([])

  const openGroupDetail = (groupId: number) => {
    loadGroupInfo(groupId)
    setTab('info')
  }

  return (
    <div className="groups-panel">
      <div className="groups-header">
        <h3>群组</h3>
      </div>
      <div className="groups-tabs">
        <button className={`ctab ${tab === 'create' ? 'active' : ''}`} onClick={() => setTab('create')}>创建</button>
        <button className={`ctab ${tab === 'list' ? 'active' : ''}`} onClick={() => { setTab('list'); loadMyGroups() }}>我的群</button>
        <button className={`ctab ${tab === 'search' ? 'active' : ''}`} onClick={() => setTab('search')}>加群</button>
      </div>
      <div className="groups-content">
        {tab === 'create' && (
          <div className="add-form">
            <div className="form-field">
              <label>群组名称</label>
              <input value={createName} onChange={e => setCreateName(e.target.value)} placeholder="输入群组名称" />
            </div>
            <div className="form-field">
              <label>选择好友拉入群聊</label>
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
                      {selectedFriendIds.includes(f.friend_id) && (
                        <div className="fp-check">✓</div>
                      )}
                    </div>
                  ))}
                </div>
              )}
              {selectedFriendIds.length > 0 && (
                <div className="selected-count">已选 {selectedFriendIds.length} 人</div>
              )}
            </div>
            <button className="btn-primary full" onClick={handleCreate}>创建群组</button>
          </div>
        )}
        {tab === 'list' && (
          <div className="group-list">
            {myGroups.map(g => (
              <div key={g.group_id} className="friend-item" onClick={() => openGroupDetail(g.group_id)}>
                <div className="friend-avatar">👥</div>
                <div className="friend-info">
                  <div className="friend-name">{g.name}</div>
                  <div className="friend-sub">群号: {g.group_number}</div>
                </div>
              </div>
            ))}
            {myGroups.length === 0 && <div className="contacts-empty">暂无群组</div>}
          </div>
        )}
        {tab === 'search' && (
          <div className="add-form">
            <div className="form-field">
              <label>搜索群号</label>
              <input type="text" value={searchNumber} onChange={e => setSearchNumber(e.target.value.replace(/\D/g, ''))} placeholder="输入群号" />
            </div>
            <button className="btn-primary full" onClick={handleSearchGroup}>搜索</button>
            {searchResult && (
              <div className="group-info-card">
                <div className="gi-header">
                  <div className="gi-avatar">👥</div>
                  <div>
                    <div className="gi-name">{searchResult.name}</div>
                    <div className="gi-id">群号: {searchResult.group_number}</div>
                  </div>
                </div>
                <div className="gi-row"><span>群主</span><span>{searchResult.owner_name}</span></div>
                <div className="form-field" style={{ marginTop: 12 }}>
                  <label>申请消息</label>
                  <input value={joinMessage} onChange={e => setJoinMessage(e.target.value)} placeholder="请输入申请消息（可选）" />
                </div>
                <button className="btn-primary full" style={{ marginTop: 8 }} onClick={handleJoinGroup}>申请加入</button>
              </div>
            )}
          </div>
        )}
        {tab === 'info' && (
          <div className="add-form">
            {!groupInfo ? (
              <div className="contacts-empty">请从"我的群"列表选择群组查看详情</div>
            ) : (
              <div className="group-info-card" style={{ marginTop: 0 }}>
                <div className="gi-header">
                  <div className="gi-avatar">👥</div>
                  <div>
                    <div className="gi-name">{groupInfo.name}</div>
                    <div className="gi-id">群号: {groupInfo.group_number}</div>
                  </div>
                </div>
                <div className="gi-row"><span>群主</span><span>{groupInfo.members?.find((m: GroupMemberInfo) => m.role === 2)?.name || '未知'}</span></div>
                <div className="gi-row"><span>公告</span><span>{groupInfo.notice || '无'}</span></div>
                <div className="gi-row"><span>成员数</span><span>{groupInfo.members?.length || 0}</span></div>
                <button className="btn-primary full" style={{ marginTop: 8 }} onClick={() => startGroupChat(groupInfo.group_id, groupInfo.name)}>
                  进入群聊
                </button>
                {isOwnerOrAdmin(groupInfo) && (
                  <>
                    <button className="btn-secondary full" style={{ marginTop: 8 }} onClick={() => { setManageMode(manageMode === 'requests' ? null : 'requests'); if (manageMode !== 'requests') loadJoinRequests(groupInfo.group_id) }}>
                      {manageMode === 'requests' ? '收起申请' : '入群申请'}
                    </button>
                    {manageMode === 'requests' && (
                      <div className="manage-section" style={{ marginTop: 8 }}>
                        {joinRequests.length === 0 ? (
                          <div className="picker-empty">暂无申请</div>
                        ) : (
                          joinRequests.map((r, i) => (
                            <div key={i} className="gi-member" style={{ flexDirection: 'column', alignItems: 'flex-start', gap: 4 }}>
                              <div style={{ display: 'flex', justifyContent: 'space-between', width: '100%' }}>
                                <span>{r.name || `用户${r.user_id}`}</span>
                                <span className={`role-tag ${r.status === 0 ? 'role-0' : r.status === 1 ? 'role-2' : 'role-1'}`}>
                                  {r.status === 0 ? '待审核' : r.status === 1 ? '已通过' : '已拒绝'}
                                </span>
                              </div>
                              {r.message && <div style={{ fontSize: 12, color: 'var(--text-secondary)' }}>{r.message}</div>}
                              {r.status === 0 && (
                                <div style={{ display: 'flex', gap: 8, marginTop: 4 }}>
                                  <button className="btn-primary" style={{ padding: '4px 12px', fontSize: 12 }} onClick={() => handleJoinReq(groupInfo.group_id, r.user_id, true)}>通过</button>
                                  <button className="btn-danger" style={{ padding: '4px 12px', fontSize: 12 }} onClick={() => handleJoinReq(groupInfo.group_id, r.user_id, false)}>拒绝</button>
                                </div>
                              )}
                            </div>
                          ))
                        )}
                      </div>
                    )}
                    <button className="btn-secondary full" style={{ marginTop: 8 }} onClick={() => { setManageMode(manageMode === 'invite' ? null : 'invite'); setSelectedInviteFriends([]) }}>
                      {manageMode === 'invite' ? '收起邀请' : '邀请成员'}
                    </button>
                    {manageMode === 'invite' && (
                      <div className="manage-section" style={{ marginTop: 8 }}>
                        {friends.length === 0 ? (
                          <div className="picker-empty">暂无好友可邀请</div>
                        ) : (
                          <>
                            <div className="friend-picker-list" style={{ maxHeight: 150 }}>
                              {friends.map(f => {
                                const inGroup = groupInfo.members?.some((m: GroupMemberInfo) => m.user_id === f.friend_id)
                                if (inGroup) return null
                                return (
                                  <div key={f.friend_id}
                                    className={`friend-picker-item ${selectedInviteFriends.includes(f.friend_id) ? 'selected' : ''}`}
                                    onClick={() => setSelectedInviteFriends(prev => prev.includes(f.friend_id) ? prev.filter(id => id !== f.friend_id) : [...prev, f.friend_id])}
                                  >
                                    <div className="fp-avatar">{(f.remark || f.name || '?')[0].toUpperCase()}</div>
                                    <div className="fp-info"><div className="fp-name">{f.remark || f.name}</div></div>
                                    {selectedInviteFriends.includes(f.friend_id) && <div className="fp-check">✓</div>}
                                  </div>
                                )
                              })}
                            </div>
                            {selectedInviteFriends.length > 0 && (
                              <button className="btn-primary full" style={{ marginTop: 8 }} onClick={() => handleInvite(groupInfo.group_id, selectedInviteFriends)}>
                                邀请选中好友 ({selectedInviteFriends.length}人)
                              </button>
                            )}
                          </>
                        )}
                      </div>
                    )}
                  </>
                )}
                {groupInfo.members && groupInfo.members.length > 0 && (
                  <div className="gi-members">
                    <div className="gi-members-title">成员列表</div>
                    {groupInfo.members.map((m: GroupMemberInfo, i: number) => (
                      <div key={i} className="gi-member">
                        <div className="gi-member-left">
                          <span className="gi-member-name">{m.name || `用户${m.user_id}`}</span>
                          <span className={`role-tag role-${m.role}`}>{getRoleLabel(m.role)}</span>
                          {m.is_muted && <span className="role-tag role-muted">禁言</span>}
                        </div>
                        {isOwnerOrAdmin(groupInfo) && m.user_id !== currentUserId && m.role !== 2 && (
                          <div className="gi-member-actions">
                            <button className="gi-action-btn" onClick={() => handleKick(groupInfo.group_id, m.user_id)} title="踢出">✕</button>
                            {isOwner(groupInfo) && (
                              <>
                                <button className="gi-action-btn" onClick={() => handleSetAdmin(groupInfo.group_id, m.user_id, m.role === 1 ? 0 : 1)} title={m.role === 1 ? '取消管理员' : '设为管理员'}>
                                  {m.role === 1 ? '↓' : '↑'}
                                </button>
                                <button className="gi-action-btn" onClick={() => handleMuted(groupInfo.group_id, m.user_id, !m.is_muted)} title={m.is_muted ? '解除禁言' : '禁言'}>
                                  {m.is_muted ? '🔊' : '🔇'}
                                </button>
                              </>
                            )}
                            {!isOwner(groupInfo) && m.role === 0 && (
                              <button className="gi-action-btn" onClick={() => handleMuted(groupInfo.group_id, m.user_id, !m.is_muted)} title={m.is_muted ? '解除禁言' : '禁言'}>
                                {m.is_muted ? '🔊' : '🔇'}
                              </button>
                            )}
                          </div>
                        )}
                        {isOwner(groupInfo) && m.role !== 2 && (
                          <button className="gi-action-btn" onClick={() => handleChangeOwner(groupInfo.group_id, m.user_id)} title="转让群主">👑</button>
                        )}
                      </div>
                    ))}
                  </div>
                )}
                {isOwner(groupInfo) && (
                  <>
                    <div className="manage-divider" />
                    <div className="manage-section">
                      <h4>修改公告</h4>
                      <input value={noticeInput} onChange={e => setNoticeInput(e.target.value)} placeholder={groupInfo.notice || '输入新公告'} />
                      <button className="btn-primary full" style={{ marginTop: 8 }} onClick={() => handleChangeNotice(groupInfo.group_id, noticeInput)}>修改公告</button>
                    </div>
                  </>
                )}
              </div>
            )}
          </div>
        )}
      </div>
      {toast && <div className="toast">{toast}</div>}
    </div>
  )
}
