import { createContext, useContext, useState, useCallback, useEffect, useRef, type ReactNode } from 'react';
import { api, parseMessageContent, buildMediaContent } from '../api/client';

interface AuthState {
  token: string;
  account: string;
}

interface WsMessage {
  type: string;
  conversation_id?: string;
  conversation_type?: number;
  peer_account?: string;
  from_account?: string;
  from_name?: string;
  content?: string;
  msg_id?: string;
  timestamp?: number;
  success?: boolean;
  reason?: string;
  new_content?: string;
  is_edited?: boolean;
  seq?: number;
  client_seq?: number;
  conv_messages?: {
    conversation_id: string;
    messages: {
      msg_id: string;
      client_seq: number;
      sender_account: string;
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
  memberAccounts: string[];
  peerAccount?: string;
  groupNumber?: string;
  lastMsg?: string;
  lastTime?: number;
  unread: number;
}

interface FriendInfo {
  friend_account: string;
  name: string;
  remark: string;
  group_id: number;
}

interface OnlineStatusMap {
  [account: string]: boolean;
}

interface MemberInfoMap {
  [account: string]: {
    name: string;
    avatar: string;
  };
}

interface TypingStatusMap {
  [convId: string]: {
    [account: string]: number;
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
  memberInfo: MemberInfoMap;
  typingStatus: TypingStatusMap;
  systemNotification: string | null;
}

interface AppContextType extends AppState {
  login: (token: string, account: string, avatarUrl?: string) => void;
  logout: () => void;
  wsConnect: () => void;
  wsDisconnect: () => void;
  sendMessage: (convId: string, content: string, mentionedIds?: number[]) => void;
  setActiveConvId: (id: string | null) => void;
  addConversation: (conv: Conversation) => void;
  setFriends: (friends: FriendInfo[]) => void;
  openChatWith: (targetAccount: string, name: string, type: number) => void;
  openSystemAI: () => Promise<void>;
  loadConversations: () => void;
  sendTyping: (convId: string) => void;
  loadOnlineStatus: (accounts: string[]) => void;
  loadConversationMembers: (accounts: string[]) => void;
  updateAvatar: (avatarUrl: string) => Promise<boolean>;
  recallMessage: (convId: string, msgId: string) => void;
  editMessage: (convId: string, msgId: string, newContent: string) => void;
  clearSystemNotification: () => void;
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
    account: localStorage.getItem('im_account') || '',
  });
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [messages, setMessages] = useState<Record<string, ChatMessage[]>>({});
  const [wsConnected, setWsConnected] = useState(false);
  const [activeConvId, setActiveConvId] = useState<string | null>(null);
  const [friends, setFriends] = useState<FriendInfo[]>([]);
  const [onlineStatus, setOnlineStatus] = useState<OnlineStatusMap>({});
  const [memberInfo, setMemberInfo] = useState<MemberInfoMap>({});
  const [typingStatus, setTypingStatus] = useState<TypingStatusMap>({});
  const [systemNotification, setSystemNotification] = useState<string | null>(null);
  const [convMaxSeqs, setConvMaxSeqs] = useState<Record<string, number>>({});
  const wsRef = useRef<WebSocket | null>(null);
  const activeConvRef = useRef<string | null>(null);
  activeConvRef.current = activeConvId;
  const sentMsgIds = useRef<Set<string>>(new Set());
  const historyLoadedRef = useRef<Set<string>>(new Set());
  const conversationsRef = useRef<Conversation[]>(conversations);
  conversationsRef.current = conversations;
  const clientSeqRef = useRef<number>(0);

  const login = useCallback((token: string, account: string, avatarUrl?: string) => {
    localStorage.setItem('im_token', token);
    localStorage.setItem('im_account', account);
    setAuth({ token, account });
    if (avatarUrl) {
      setMemberInfo(prev => ({
        ...prev,
        [account]: { name: account, avatar: avatarUrl },
      }));
    }
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem('im_token');
    localStorage.removeItem('im_account');
    setAuth({ token: '', account: '' });
    if (wsRef.current) { wsRef.current.close(); wsRef.current = null; }
    setWsConnected(false);
    setConversations([]);
    setMessages({});
    setActiveConvId(null);
    setFriends([]);
    setMemberInfo({});
    historyLoadedRef.current.clear();
  }, []);

  const loadConversations = useCallback(async () => {
    const res = await api('GET', '/api/chat/conversations');
    if (res.code === 0 && res.data?.conversations) {
      const loaded: Conversation[] = res.data.conversations.map((c: any) => ({
        id: String(c.conversation_id),
        name: c.name || `会话 ${c.conversation_id}`,
        type: c.type,
        memberAccounts: (c.member_accounts || []).map((a: any) => String(a)),
        groupNumber: c.group_number && c.group_number !== '0' ? String(c.group_number) : undefined,
        lastMsg: '',
        lastTime: 0,
        unread: c.unread_count || 0,
      }));
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
      const myAccount = auth.account;
      const historyMsgs: ChatMessage[] = res.data.messages.map((m: any) => {
        const ts = m.timestamp || 0;
        const timeSec = ts > 1e12 ? ts / 1000 : ts;
        return {
          id: String(m.msg_id),
          from: String(m.sender_account),
          fromName: m.sender_name || '',
          conversationId: String(m.conversation_id),
          content: m.content,
          time: timeSec,
          isSent: String(m.sender_account) === myAccount,
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
  }, [auth.account]);

  const wsConnect = useCallback(() => {
    if (wsRef.current) wsRef.current.close();
    if (!auth.token) return;
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${proto}//${window.location.host}/ws?token=${encodeURIComponent(auth.token)}`;
    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      setWsConnected(true);
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
          from_account: raw.from_account != null ? String(raw.from_account) : undefined,
          msg_id: raw.msg_id != null ? String(raw.msg_id) : undefined,
          peer_account: raw.peer_account != null ? String(raw.peer_account) : undefined,
        };
        if (msg.type === 'typing' && msg.from_account && msg.conversation_id) {
          setTypingStatus(prev => ({
            ...prev,
            [msg.conversation_id!]: {
              ...prev[msg.conversation_id!],
              [msg.from_account!]: Date.now(),
            },
          }));
        }
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
        if (msg.type === 'system' && msg.reason) {
          const convId = msg.conversation_id;
          if (convId) {
            setMessages(prev => {
              const msgs = prev[convId] || [];
              const filtered = msgs.filter(m => !(m.isSent && m.id.startsWith('temp_')));
              if (filtered.length === msgs.length) return prev;
              return { ...prev, [convId]: filtered };
            });
          }
          setSystemNotification(msg.reason);
          setTimeout(() => setSystemNotification(null), 3000);
        }
        if (msg.type === 'sync' && msg.success && msg.conv_messages) {
          const myAccount = auth.account;
          for (const cm of msg.conv_messages) {
            const convId = String(cm.conversation_id);
            if (!cm.messages || cm.messages.length === 0) continue;
            const syncMsgs: ChatMessage[] = cm.messages.map((m: any) => {
              const ts = m.timestamp || 0;
              const timeSec = ts > 1e12 ? ts / 1000 : ts;
              return {
                id: String(m.msg_id),
                from: String(m.sender_account),
                fromName: '',
                conversationId: convId,
                content: m.content,
                time: timeSec,
                isSent: String(m.sender_account) === myAccount,
                status: m.status || 0,
                isEdited: m.is_edited || false,
              };
            });
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
        if (msg.type === 'chat' && msg.from_account && msg.content && msg.conversation_id) {
          const convId = msg.conversation_id;
          const myAccount = auth.account;
          const isSent = msg.from_account === myAccount;

          const msgId = msg.msg_id || `${msg.from_account}_${msg.timestamp}_${Date.now()}`;
          if (isSent && sentMsgIds.current.has(msgId)) return;
          if (isSent) sentMsgIds.current.add(msgId);
          if (sentMsgIds.current.size > 1000) {
            const arr = Array.from(sentMsgIds.current);
            sentMsgIds.current = new Set(arr.slice(-500));
          }

          const ts = msg.timestamp || 0;
          const timeSec = ts > 1e12 ? ts / 1000 : ts;

          if (msg.seq && msg.seq > 0 && convId) {
            setConvMaxSeqs(prev => {
              const cur = prev[convId] || 0;
              if (msg.seq! > cur) return { ...prev, [convId]: msg.seq! };
              return prev;
            });
          }

          const chatMsg: ChatMessage = {
            id: msgId,
            from: msg.from_account,
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
              const peerAccount = tempConv.peerAccount || tempConv.memberAccounts.find(a => a !== myAccount);
              const isMatch = (isSent && peerAccount) || (!isSent && msg.from_account === peerAccount);
              if (isMatch) {
                const upgraded = prev.map(c => {
                  if (c.id === '' && c.type === 1) {
                    return { ...c, id: convId, peerAccount: undefined };
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
              memberAccounts: [],
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
  }, [auth.token, auth.account]);

  const wsDisconnect = useCallback(() => {
    if (wsRef.current) { wsRef.current.close(); wsRef.current = null; }
  }, []);

  const sendMessage = useCallback((convId: string, content: string, mentionedIds?: number[]) => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return;
    const conv = conversationsRef.current.find(c => c.id === convId);
    if (!conv) return;
    clientSeqRef.current += 1;
    const msg: any = {
      type: 'chat',
      conversation_id: convId === '' ? '0' : convId,
      content,
      client_seq: clientSeqRef.current,
    };
    if (mentionedIds && mentionedIds.length > 0) {
      msg.mentioned_ids = mentionedIds;
    }
    if (conv.type === 1) {
      const myAccount = auth.account;
      const peerAccount = conv.peerAccount || conv.memberAccounts.find(a => a !== myAccount);
      if (peerAccount) {
        msg.peer_account = peerAccount;
      }
    }
    const tempId = `temp_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
    sentMsgIds.current.add(tempId);
    wsRef.current.send(JSON.stringify(msg));

    const optimisticMsg: ChatMessage = {
      id: tempId,
      from: auth.account,
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
  }, [auth.account]);

  const addConversation = useCallback((conv: Conversation) => {
    setConversations(prev => {
      if (prev.find(c => c.id === conv.id)) return prev;
      return [conv, ...prev];
    });
  }, []);

  const openChatWith = useCallback((targetAccount: string, name: string, type: number) => {
    const existing = conversationsRef.current.find(c => {
      if (type === 1) {
        return c.type === 1 && c.memberAccounts.includes(auth.account) && c.memberAccounts.includes(targetAccount) && c.memberAccounts.length === 2;
      }
      return c.groupNumber === targetAccount;
    });
    if (existing) {
      setActiveConvId(existing.id);
      return;
    }
    const tempConv: Conversation = {
      id: '',
      name,
      type,
      memberAccounts: type === 1 ? [auth.account, targetAccount] : [],
      peerAccount: type === 1 ? targetAccount : undefined,
      unread: 0,
    };
    addConversation(tempConv);
    setActiveConvId('');
  }, [auth.account, addConversation]);

  const openSystemAI = useCallback(async () => {
    const sysRes = await api('GET', '/api/bot/system');
    if (sysRes.code !== 0 || !sysRes.data?.bot_id) {
      return;
    }
    const botId = sysRes.data.bot_id;
    const addRes = await api('POST', '/api/bot/add_to_conversation', {
      bot_id: botId,
      conversation_id: 0,
      conversation_type: 1,
    });
    if (addRes.code !== 0) {
      return;
    }
    const convId = String(addRes.data?.conversation_id || '');
    if (!convId || convId === '0') {
      const existing = conversationsRef.current.find(c =>
        c.type === 1 && c.memberAccounts.includes(String(botId))
      );
      if (existing) {
        setActiveConvId(existing.id);
      }
      return;
    }
    await loadConversations();
    setActiveConvId(convId);
  }, [auth.account, loadConversations]);

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

  const loadOnlineStatus = useCallback(async (accounts: string[]) => {
    if (accounts.length === 0) return;
    const res = await api('POST', '/api/chat/online_status', { accounts });
    if (res.code === 0 && res.data?.statuses) {
      const map: OnlineStatusMap = {};
      for (const s of res.data.statuses) {
        map[s.account] = s.online;
      }
      setOnlineStatus(prev => ({ ...prev, ...map }));
    }
  }, []);

  const loadConversationMembers = useCallback(async (accounts: string[]) => {
    if (accounts.length === 0) return;
    const needLoad = accounts.filter(a => !memberInfo[a]);
    if (needLoad.length === 0) return;
    const res = await api('POST', '/api/chat/conversation_members', { accounts: needLoad });
    if (res.code === 0 && res.data?.members) {
      const map: MemberInfoMap = {};
      for (const m of res.data.members) {
        map[m.account] = { name: m.name, avatar: m.avatar_url || '' };
      }
      setMemberInfo(prev => ({ ...prev, ...map }));
    }
  }, [memberInfo]);

  const updateAvatar = useCallback(async (avatarUrl: string): Promise<boolean> => {
    const res = await api('POST', '/api/update_avatar', { avatar_url: avatarUrl });
    if (res.code === 0) {
      setMemberInfo(prev => ({
        ...prev,
        [auth.account]: { ...prev[auth.account], avatar: avatarUrl },
      }));
      return true;
    }
    return false;
  }, [auth.account]);

  const recallMessage = useCallback((convId: string, msgId: string) => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return;
    wsRef.current.send(JSON.stringify({
      type: 'recall',
      conversation_id: convId,
      msg_id: msgId,
    }));
  }, []);

  const editMessage = useCallback((convId: string, msgId: string, newContent: string) => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return;
    wsRef.current.send(JSON.stringify({
      type: 'edit',
      conversation_id: convId,
      msg_id: msgId,
      new_content: newContent,
    }));
  }, []);

  const clearSystemNotification = useCallback(() => {
    setSystemNotification(null);
  }, []);

  useEffect(() => {
    const timer = setInterval(() => {
      const now = Date.now();
      setTypingStatus(prev => {
        const next: TypingStatusMap = {};
        let changed = false;
        for (const convId of Object.keys(prev)) {
          const users = prev[convId];
          const filtered: { [acc: string]: number } = {};
          for (const acc of Object.keys(users)) {
            if (now - users[acc] < 5000) {
              filtered[acc] = users[acc];
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
    if (auth.token && auth.account && !memberInfo[auth.account]) {
      loadConversationMembers([auth.account]);
    }
  }, [auth.token, auth.account]);

  useEffect(() => {
    if (activeConvId && activeConvId !== '') {
      loadHistory(activeConvId);
      api('POST', `/api/chat/mark_read/${activeConvId}`).catch(() => {});
      setConversations(prev => prev.map(c =>
        c.id === activeConvId ? { ...c, unread: 0 } : c
      ));
    }
  }, [activeConvId, loadHistory]);

  return (
    <AppContext.Provider value={{
      auth, conversations, messages, wsConnected, activeConvId, friends, onlineStatus, memberInfo, typingStatus, systemNotification,
      login, logout, wsConnect, wsDisconnect, sendMessage, setActiveConvId, addConversation,
      setFriends, openChatWith, openSystemAI, loadConversations, sendTyping, loadOnlineStatus, loadConversationMembers, updateAvatar, recallMessage, editMessage, clearSystemNotification,
    }}>
      {children}
    </AppContext.Provider>
  );
}
