import { createContext, useContext, useState, useCallback, useEffect, useRef, type ReactNode } from 'react';
import { api, parseMessageContent, buildMediaContent } from '../api/client';

interface AuthState {
  token: string;
  userId: string;
  account: string;
}

interface WsMessage {
  type: string;
  conversation_id?: string;
  conversation_type?: number;
  peer_id?: string;
  from?: string;
  from_name?: string;
  content?: string;
  msg_id?: string;
  timestamp?: number;
  success?: boolean;
  reason?: string;
  new_content?: string;
  is_edited?: boolean;
  seq?: number;
  conv_messages?: {
    conversation_id: string;
    messages: {
      msg_id: string;
      client_seq: number;
      sender_id: string;
      conversation_id: string;
      content: string;
      timestamp: number;
      seq: number;
      status: number;
      is_edited: boolean;
    }[];
  }[];
}

interface ChatMessage {
  id: string;
  from: string;
  fromName: string;
  conversationId: string;
  content: string;
  time: number;
  isSent: boolean;
  status?: number;
  isEdited?: boolean;
}

interface Conversation {
  id: string;
  name: string;
  type: number;
  memberIds: string[];
  peerId?: string;
  groupId?: string;
  lastMsg?: string;
  lastTime?: number;
  unread: number;
}

interface FriendInfo {
  friend_id: number;
  name: string;
  remark: string;
  group_id: number;
}

// 用户在线状态映射：userId → online
interface OnlineStatusMap {
  [userId: string]: boolean;
}

// 输入状态映射：conversationId → { userId → timestamp }
interface TypingStatusMap {
  [convId: string]: {
    [userId: string]: number; // 最后一次 typing 事件的时间戳
  };
}

interface AppState {
  auth: AuthState;
  conversations: Conversation[];
  messages: Record<string, ChatMessage[]>;
  wsConnected: boolean;
  activeConvId: string | null;
  friends: FriendInfo[];
  onlineStatus: OnlineStatusMap;
  typingStatus: TypingStatusMap;
}

interface AppContextType extends AppState {
  login: (token: string, userId: string, account: string) => void;
  logout: () => void;
  wsConnect: () => void;
  wsDisconnect: () => void;
  sendMessage: (convId: string, content: string) => void;
  setActiveConvId: (id: string | null) => void;
  addConversation: (conv: Conversation) => void;
  setFriends: (friends: FriendInfo[]) => void;
  openChatWith: (targetId: string, name: string, type: number) => void;
  loadConversations: () => void;
  sendTyping: (convId: string) => void;
  loadOnlineStatus: (userIds: string[]) => void;
  recallMessage: (convId: string, msgId: string) => void;
  editMessage: (convId: string, msgId: string, newContent: string) => void;
}

const AppContext = createContext<AppContextType | null>(null);

export function useApp() {
  const ctx = useContext(AppContext);
  if (!ctx) throw new Error('useApp must be used within AppProvider');
  return ctx;
}

function extractDisplayText(content: string): string {
  const parsed = parseMessageContent(content);
  switch (parsed.type) {
    case 'image': return '[图片]';
    case 'file': return `[文件] ${parsed.filename || ''}`;
    case 'voice': return `[语音] ${parsed.duration || 0}s`;
    default: return parsed.text || content;
  }
}

export function AppProvider({ children }: { children: ReactNode }) {
  const [auth, setAuth] = useState<AuthState>({
    token: localStorage.getItem('im_token') || '',
    userId: localStorage.getItem('im_userId') || '',
    account: localStorage.getItem('im_account') || '',
  });
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [messages, setMessages] = useState<Record<string, ChatMessage[]>>({});
  const [wsConnected, setWsConnected] = useState(false);
  const [activeConvId, setActiveConvId] = useState<string | null>(null);
  const [friends, setFriends] = useState<FriendInfo[]>([]);
  const [onlineStatus, setOnlineStatus] = useState<OnlineStatusMap>({});
  const [typingStatus, setTypingStatus] = useState<TypingStatusMap>({});
  const [convMaxSeqs, setConvMaxSeqs] = useState<Record<string, number>>({});
  const wsRef = useRef<WebSocket | null>(null);
  const activeConvRef = useRef<string | null>(null);
  activeConvRef.current = activeConvId;
  const sentMsgIds = useRef<Set<string>>(new Set());
  const historyLoadedRef = useRef<Set<string>>(new Set());
  const conversationsRef = useRef<Conversation[]>(conversations);
  conversationsRef.current = conversations;
  // 客户端序列号，用于消息去重：每次发送递增，保证同一用户的每次发送请求唯一
  const clientSeqRef = useRef<number>(0);

  const login = useCallback((token: string, userId: string, account: string) => {
    localStorage.setItem('im_token', token);
    localStorage.setItem('im_userId', userId);
    localStorage.setItem('im_account', account);
    setAuth({ token, userId, account });
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem('im_token');
    localStorage.removeItem('im_userId');
    localStorage.removeItem('im_account');
    setAuth({ token: '', userId: '', account: '' });
    if (wsRef.current) { wsRef.current.close(); wsRef.current = null; }
    setWsConnected(false);
    setConversations([]);
    setMessages({});
    setActiveConvId(null);
    setFriends([]);
    historyLoadedRef.current.clear();
  }, []);

  const loadConversations = useCallback(async () => {
    const res = await api('GET', '/api/chat/conversations');
    if (res.code === 0 && res.data?.conversations) {
      const loaded: Conversation[] = res.data.conversations.map((c: any) => ({
        id: String(c.conversation_id),
        name: c.name || `会话 ${c.conversation_id}`,
        type: c.type,
        memberIds: (c.member_ids || []).map((id: any) => String(id)),
        groupId: c.group_id ? String(c.group_id) : undefined,
        lastMsg: '',
        lastTime: 0,
        unread: c.unread_count || 0,
      }));
      // 从会话列表中提取 max_seq，初始化本地 seq 记录
      const seqMap: Record<string, number> = {};
      for (const c of res.data.conversations) {
        if (c.max_seq && c.max_seq > 0) {
          seqMap[String(c.conversation_id)] = c.max_seq;
        }
      }
      if (Object.keys(seqMap).length > 0) {
        setConvMaxSeqs(prev => {
          const next = { ...prev };
          for (const [convId, seq] of Object.entries(seqMap)) {
            if ((next[convId] || 0) < seq) next[convId] = seq;
          }
          return next;
        });
      }
      setConversations(prev => {
        const existingMap = new Map(prev.map(c => [c.id, c]));
        for (const c of loaded) {
          if (existingMap.has(c.id)) {
            const old = existingMap.get(c.id)!;
            c.lastMsg = old.lastMsg;
            c.lastTime = old.lastTime;
          }
        }
        return loaded;
      });
    }
  }, []);

  const loadHistory = useCallback(async (convId: string) => {
    if (historyLoadedRef.current.has(convId)) return;
    const res = await api('GET', `/api/chat/messages?conversation_id=${convId}&limit=50`);
    if (res.code === 0 && res.data?.messages) {
      const myId = auth.userId;
      const historyMsgs: ChatMessage[] = res.data.messages.map((m: any) => {
        const ts = m.timestamp || 0;
        const timeSec = ts > 1e12 ? ts / 1000 : ts;
        return {
          id: String(m.msg_id),
          from: String(m.sender_id),
          fromName: m.sender_name || '',
          conversationId: String(m.conversation_id),
          content: m.content,
          time: timeSec,
          isSent: String(m.sender_id) === myId,
          status: m.status || 0,
          isEdited: m.is_edited || false,
        };
      });
      historyLoadedRef.current.add(convId);
      setMessages(prev => {
        const existing = prev[convId] || [];
        const existingIds = new Set(existing.map(m => m.id));
        const newMsgs = historyMsgs.filter(m => !existingIds.has(m.id));
        const merged = [...newMsgs, ...existing].sort((a, b) => a.time - b.time);
        return { ...prev, [convId]: merged };
      });
      // 从历史消息中提取最大 seq，更新本地 seq 记录
      if (res.data.messages.length > 0) {
        let maxSeq = 0;
        for (const m of res.data.messages) {
          if (m.seq && m.seq > maxSeq) maxSeq = m.seq;
        }
        if (maxSeq > 0) {
          setConvMaxSeqs(prev => {
            const cur = prev[convId] || 0;
            if (maxSeq > cur) return { ...prev, [convId]: maxSeq };
            return prev;
          });
        }
      }
    }
  }, [auth.userId]);

  const wsConnect = useCallback(() => {
    if (wsRef.current) wsRef.current.close();
    if (!auth.token) return;
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${proto}//${window.location.host}/ws?token=${encodeURIComponent(auth.token)}`;
    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      setWsConnected(true);
      // WS 重连后，通过 WS 发送 sync 请求拉取断线期间的增量消息
      // 遍历本地所有会话的 maxSeq，请求服务端返回 seq > maxSeq 的消息
      setConvMaxSeqs(prev => {
        const seqs = Object.entries(prev);
        if (seqs.length === 0) return prev;
        const convSeqs = seqs.map(([convId, lastSeq]) => ({
          conversation_id: convId,
          last_seq: lastSeq,
        }));
        ws.send(JSON.stringify({
          type: 'sync',
          conv_seqs: convSeqs,
          limit: 50,
        }));
        return prev;
      });
    };
    ws.onclose = () => { setWsConnected(false); wsRef.current = null; };
    ws.onerror = () => setWsConnected(false);

    ws.onmessage = (e) => {
      try {
        const raw = JSON.parse(e.data);
        const msg: WsMessage = {
          ...raw,
          conversation_id: raw.conversation_id != null ? String(raw.conversation_id) : undefined,
          from: raw.from != null ? String(raw.from) : undefined,
          msg_id: raw.msg_id != null ? String(raw.msg_id) : undefined,
          peer_id: raw.peer_id != null ? String(raw.peer_id) : undefined,
        };
        if (msg.type === 'typing' && msg.from && msg.conversation_id) {
          setTypingStatus(prev => ({
            ...prev,
            [msg.conversation_id!]: {
              ...prev[msg.conversation_id!],
              [msg.from!]: Date.now(),
            },
          }));
        }
        // 处理撤回消息事件：将消息标记为已撤回
        if (msg.type === 'recall' && msg.msg_id && msg.conversation_id) {
          const convId = msg.conversation_id;
          const msgId = String(msg.msg_id);
          setMessages(prev => {
            const msgs = prev[convId] || [];
            return {
              ...prev,
              [convId]: msgs.map(m =>
                m.id === msgId ? { ...m, status: 1, content: '' } : m
              ),
            };
          });
        }
        // 处理编辑消息事件：更新消息内容和编辑标记
        if (msg.type === 'edit' && msg.msg_id && msg.conversation_id) {
          const convId = msg.conversation_id;
          const msgId = String(msg.msg_id);
          const newContent = msg.new_content || '';
          setMessages(prev => {
            const msgs = prev[convId] || [];
            return {
              ...prev,
              [convId]: msgs.map(m =>
                m.id === msgId ? { ...m, content: newContent, isEdited: true } : m
              ),
            };
          });
        }
        // 处理同步消息响应：将增量消息合并到本地消息列表
        if (msg.type === 'sync' && msg.success && msg.conv_messages) {
          const myId = auth.userId;
          for (const cm of msg.conv_messages) {
            const convId = String(cm.conversation_id);
            if (!cm.messages || cm.messages.length === 0) continue;
            const syncMsgs: ChatMessage[] = cm.messages.map((m: any) => {
              const ts = m.timestamp || 0;
              const timeSec = ts > 1e12 ? ts / 1000 : ts;
              return {
                id: String(m.msg_id),
                from: String(m.sender_id),
                fromName: '',
                conversationId: convId,
                content: m.content,
                time: timeSec,
                isSent: String(m.sender_id) === myId,
                status: m.status || 0,
                isEdited: m.is_edited || false,
              };
            });
            // 更新该会话的最大 seq
            let maxSeq = 0;
            for (const m of cm.messages) {
              if (m.seq && m.seq > maxSeq) maxSeq = m.seq;
            }
            if (maxSeq > 0) {
              setConvMaxSeqs(prev => {
                const cur = prev[convId] || 0;
                if (maxSeq > cur) return { ...prev, [convId]: maxSeq };
                return prev;
              });
            }
            setMessages(prev => {
              const existing = prev[convId] || [];
              const existingIds = new Set(existing.map(m => m.id));
              const newMsgs = syncMsgs.filter(m => !existingIds.has(m.id));
              if (newMsgs.length === 0) return prev;
              const merged = [...existing, ...newMsgs].sort((a, b) => a.time - b.time);
              return { ...prev, [convId]: merged };
            });
            // 更新会话列表的 lastMsg
            const lastSyncMsg = syncMsgs[syncMsgs.length - 1];
            if (lastSyncMsg) {
              const displayText = extractDisplayText(lastSyncMsg.content);
              setConversations(prev => {
                const exists = prev.find(c => c.id === convId);
                if (exists) {
                  return prev.map(c => c.id === convId
                    ? { ...c, lastMsg: displayText, lastTime: lastSyncMsg.time, unread: c.id === activeConvRef.current ? c.unread : c.unread + 1 }
                    : c);
                }
                return prev;
              });
            }
          }
        }
        if (msg.type === 'chat' && msg.from && msg.content && msg.conversation_id) {
          const convId = msg.conversation_id;
          const myId = auth.userId;
          const isSent = msg.from === myId;

          const msgId = msg.msg_id || `${msg.from}_${msg.timestamp}_${Date.now()}`;
          if (isSent && sentMsgIds.current.has(msgId)) return;
          if (isSent) sentMsgIds.current.add(msgId);
          if (sentMsgIds.current.size > 1000) {
            const arr = Array.from(sentMsgIds.current);
            sentMsgIds.current = new Set(arr.slice(-500));
          }

          const ts = msg.timestamp || 0;
          const timeSec = ts > 1e12 ? ts / 1000 : ts;

          // 更新该会话的最大 seq，用于断线重连后的增量同步
          if (msg.seq && msg.seq > 0 && convId) {
            setConvMaxSeqs(prev => {
              const cur = prev[convId] || 0;
              if (msg.seq! > cur) return { ...prev, [convId]: msg.seq! };
              return prev;
            });
          }

          const chatMsg: ChatMessage = {
            id: msgId,
            from: msg.from,
            fromName: msg.from_name || '',
            conversationId: convId,
            content: msg.content,
            time: timeSec,
            isSent,
          };

          const displayText = extractDisplayText(msg.content);

          setConversations(prev => {
            const tempConv = prev.find(c => c.id === '' && c.type === 1);
            if (tempConv && convId !== '') {
              const peerId = tempConv.peerId || tempConv.memberIds.find(id => id !== myId);
              const isMatch = (isSent && peerId) || (!isSent && msg.from === peerId);
              if (isMatch) {
                const upgraded = prev.map(c => {
                  if (c.id === '' && c.type === 1) {
                    return { ...c, id: convId, peerId: undefined };
                  }
                  return c;
                });
                const already = upgraded.find(c => c.id === convId);
                if (already) {
                  return upgraded.map(c => c.id === convId
                    ? { ...c, lastMsg: displayText, lastTime: chatMsg.time, unread: c.id === activeConvRef.current ? c.unread : c.unread + 1 }
                    : c);
                }
                return upgraded.map(c => c.id === convId
                  ? { ...c, lastMsg: displayText, lastTime: chatMsg.time }
                  : c);
              }
            }
            const exists = prev.find(c => c.id === convId);
            if (exists) {
              return prev.map(c => c.id === convId
                ? { ...c, lastMsg: displayText, lastTime: chatMsg.time, unread: c.id === activeConvRef.current ? c.unread : c.unread + 1 }
                : c);
            }
            const newConv: Conversation = {
              id: convId,
              name: msg.from_name || `会话 ${convId}`,
              type: msg.conversation_type || 1,
              memberIds: [],
              lastMsg: displayText,
              lastTime: chatMsg.time,
              unread: isSent ? 0 : 1,
            };
            return [newConv, ...prev];
          });

          setMessages(prev => {
            const tempMsgs = prev[''] || [];
            const existing = prev[convId] || [];
            if (tempMsgs.length > 0 && convId !== '') {
              const migrated = tempMsgs.map(m => ({ ...m, conversationId: convId }));
              const merged = [...migrated, ...existing];
              if (isSent) {
                const dupIdx = merged.findIndex(m =>
                  m.isSent && m.content === chatMsg.content && m.id.startsWith('temp_')
                );
                if (dupIdx !== -1) {
                  merged[dupIdx] = chatMsg;
                } else {
                  merged.push(chatMsg);
                }
              } else {
                merged.push(chatMsg);
              }
              const { ['']: _, ...rest } = prev;
              return { ...rest, [convId]: merged };
            }
            if (isSent) {
              const dupIdx = existing.findIndex(m =>
                m.isSent && m.content === chatMsg.content && m.id.startsWith('temp_')
              );
              if (dupIdx !== -1) {
                const updated = [...existing];
                updated[dupIdx] = chatMsg;
                return { ...prev, [convId]: updated };
              }
            }
            return { ...prev, [convId]: [...existing, chatMsg] };
          });

          if (activeConvRef.current === '' && convId !== '') {
            setActiveConvId(convId);
          }
          return;
        }
      } catch {}
    };
  }, [auth.token, auth.userId]);

  const wsDisconnect = useCallback(() => {
    if (wsRef.current) { wsRef.current.close(); wsRef.current = null; }
  }, []);

  const sendMessage = useCallback((convId: string, content: string) => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return;
    const conv = conversationsRef.current.find(c => c.id === convId);
    if (!conv) return;
    // 递增 clientSeq，保证每次发送请求携带唯一序列号，服务端据此去重
    clientSeqRef.current += 1;
    const msg: any = {
      type: 'chat',
      conversation_id: convId === '' ? '0' : convId,
      content,
      client_seq: clientSeqRef.current,
    };
    if (conv.type === 1) {
      const myId = auth.userId;
      const peerId = conv.peerId || conv.memberIds.find(id => id !== myId);
      if (peerId) {
        msg.peer_id = peerId;
      }
    }
    const tempId = `temp_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
    sentMsgIds.current.add(tempId);
    wsRef.current.send(JSON.stringify(msg));

    const optimisticMsg: ChatMessage = {
      id: tempId,
      from: auth.userId,
      fromName: '',
      conversationId: convId,
      content,
      time: Date.now() / 1000,
      isSent: true,
    };
    setMessages(prev => ({
      ...prev,
      [convId]: [...(prev[convId] || []), optimisticMsg],
    }));
    const displayText = extractDisplayText(content);
    setConversations(prev => prev.map(c =>
      c.id === convId ? { ...c, lastMsg: displayText, lastTime: optimisticMsg.time } : c
    ));
  }, [auth.userId]);

  const addConversation = useCallback((conv: Conversation) => {
    setConversations(prev => {
      if (prev.find(c => c.id === conv.id)) return prev;
      return [conv, ...prev];
    });
  }, []);

  const openChatWith = useCallback((targetId: string, name: string, type: number) => {
    const existing = conversationsRef.current.find(c => {
      if (type === 1) {
        return c.type === 1 && c.memberIds.includes(auth.userId) && c.memberIds.includes(targetId) && c.memberIds.length === 2;
      }
      return c.groupId === targetId;
    });
    if (existing) {
      setActiveConvId(existing.id);
      return;
    }
    const tempConv: Conversation = {
      id: '',
      name,
      type,
      memberIds: type === 1 ? [auth.userId, targetId] : [],
      peerId: type === 1 ? targetId : undefined,
      unread: 0,
    };
    addConversation(tempConv);
    setActiveConvId('');
  }, [auth.userId, addConversation]);

  // 发送输入状态事件：通知会话中的其他成员"我正在输入"
  // 节流控制：3 秒内只发送一次，避免高频消息
  const lastTypingSentRef = useRef<number>(0);
  const sendTyping = useCallback((convId: string) => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return;
    if (!convId || convId === '') return;
    const now = Date.now();
    if (now - lastTypingSentRef.current < 3000) return;
    lastTypingSentRef.current = now;
    wsRef.current.send(JSON.stringify({
      type: 'typing',
      conversation_id: convId,
    }));
  }, []);

  // 批量查询用户在线状态
  // 传入一组 userId，调用 API Gateway 查询后更新 onlineStatus 状态
  const loadOnlineStatus = useCallback(async (userIds: string[]) => {
    if (userIds.length === 0) return;
    const res = await api('POST', '/api/chat/online_status', { user_ids: userIds });
    if (res.code === 0 && res.data?.statuses) {
      const map: OnlineStatusMap = {};
      for (const s of res.data.statuses) {
        map[s.user_id] = s.online;
      }
      setOnlineStatus(prev => ({ ...prev, ...map }));
    }
  }, []);

  // 通过 WS 发送撤回消息请求
  const recallMessage = useCallback((convId: string, msgId: string) => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return;
    wsRef.current.send(JSON.stringify({
      type: 'recall',
      conversation_id: convId,
      msg_id: msgId,
    }));
  }, []);

  // 通过 WS 发送编辑消息请求
  const editMessage = useCallback((convId: string, msgId: string, newContent: string) => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return;
    wsRef.current.send(JSON.stringify({
      type: 'edit',
      conversation_id: convId,
      msg_id: msgId,
      new_content: newContent,
    }));
  }, []);

  // 自动清理过期的 typing 状态（超过 5 秒的视为过期）
  useEffect(() => {
    const timer = setInterval(() => {
      const now = Date.now();
      setTypingStatus(prev => {
        const next: TypingStatusMap = {};
        let changed = false;
        for (const convId of Object.keys(prev)) {
          const users = prev[convId];
          const filtered: { [uid: string]: number } = {};
          for (const uid of Object.keys(users)) {
            if (now - users[uid] < 5000) {
              filtered[uid] = users[uid];
            } else {
              changed = true;
            }
          }
          if (Object.keys(filtered).length > 0) {
            next[convId] = filtered;
          } else {
            changed = true;
          }
        }
        return changed ? next : prev;
      });
    }, 2000);
    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    if (auth.token && !wsRef.current) wsConnect();
  }, [auth.token, wsConnect]);

  useEffect(() => {
    if (activeConvId && activeConvId !== '') {
      loadHistory(activeConvId);
      // 切换到该会话时，通知服务端标记已读，清除服务端未读数
      api('POST', `/api/chat/mark_read/${activeConvId}`).catch(() => {});
      setConversations(prev => prev.map(c =>
        c.id === activeConvId ? { ...c, unread: 0 } : c
      ));
    }
  }, [activeConvId, loadHistory]);

  return (
    <AppContext.Provider value={{
      auth, conversations, messages, wsConnected, activeConvId, friends, onlineStatus, typingStatus,
      login, logout, wsConnect, wsDisconnect, sendMessage, setActiveConvId, addConversation,
      setFriends, openChatWith, loadConversations, sendTyping, loadOnlineStatus, recallMessage, editMessage,
    }}>
      {children}
    </AppContext.Provider>
  );
}
