import { createContext, useContext, useState, useCallback, useEffect, useRef, type ReactNode } from 'react';
import { api } from '../api/client';

interface AuthState {
  token: string;
  userId: string;
  account: string;
}

interface WsMessage {
  type: string;
  conversation_id?: number;
  peer_id?: number;
  from?: number;
  content?: string;
  msg_id?: number;
  timestamp?: number;
  success?: boolean;
  reason?: string;
}

interface ChatMessage {
  id: string;
  from: number;
  conversationId: number;
  content: string;
  time: number;
  isSent: boolean;
}

interface Conversation {
  id: number;
  name: string;
  type: number;
  memberIds: number[];
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

interface AppState {
  auth: AuthState;
  conversations: Conversation[];
  messages: Record<number, ChatMessage[]>;
  wsConnected: boolean;
  activeConvId: number | null;
  friends: FriendInfo[];
}

interface AppContextType extends AppState {
  login: (token: string, userId: string, account: string) => void;
  logout: () => void;
  wsConnect: () => void;
  wsDisconnect: () => void;
  sendMessage: (convId: number, content: string) => void;
  setActiveConvId: (id: number | null) => void;
  addConversation: (conv: Conversation) => void;
  setFriends: (friends: FriendInfo[]) => void;
  openChatWith: (targetId: number, name: string, type: number) => void;
  loadConversations: () => void;
}

const AppContext = createContext<AppContextType | null>(null);

export function useApp() {
  const ctx = useContext(AppContext);
  if (!ctx) throw new Error('useApp must be used within AppProvider');
  return ctx;
}

export function AppProvider({ children }: { children: ReactNode }) {
  const [auth, setAuth] = useState<AuthState>({
    token: localStorage.getItem('im_token') || '',
    userId: localStorage.getItem('im_userId') || '',
    account: localStorage.getItem('im_account') || '',
  });
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [messages, setMessages] = useState<Record<number, ChatMessage[]>>({});
  const [wsConnected, setWsConnected] = useState(false);
  const [activeConvId, setActiveConvId] = useState<number | null>(null);
  const [friends, setFriends] = useState<FriendInfo[]>([]);
  const wsRef = useRef<WebSocket | null>(null);
  const activeConvRef = useRef<number | null>(null);
  activeConvRef.current = activeConvId;
  const sentMsgIds = useRef<Set<string>>(new Set());
  const historyLoadedRef = useRef<Set<number>>(new Set());
  const conversationsRef = useRef<Conversation[]>(conversations);
  conversationsRef.current = conversations;

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
    const res = await api('GET', '/api/v1/chat/conversations');
    if (res.code === 0 && res.data?.conversations) {
      const loaded: Conversation[] = res.data.conversations.map((c: any) => ({
        id: c.conversation_id,
        name: c.name || `会话 ${c.conversation_id}`,
        type: c.type,
        memberIds: c.member_ids || [],
        lastMsg: '',
        lastTime: 0,
        unread: 0,
      }));
      setConversations(prev => {
        const existingMap = new Map(prev.map(c => [c.id, c]));
        for (const c of loaded) {
          if (existingMap.has(c.id)) {
            const old = existingMap.get(c.id)!;
            c.lastMsg = old.lastMsg;
            c.lastTime = old.lastTime;
            c.unread = old.unread;
          }
        }
        return loaded;
      });
    }
  }, []);

  const loadHistory = useCallback(async (convId: number) => {
    if (historyLoadedRef.current.has(convId)) return;
    const res = await api('GET', `/api/v1/chat/history?conversation_id=${convId}&limit=50`);
    if (res.code === 0 && res.data?.messages) {
      const myId = Number(auth.userId);
      const historyMsgs: ChatMessage[] = res.data.messages.map((m: any) => {
        const ts = m.timestamp || 0;
        const timeSec = ts > 1e12 ? ts / 1000 : ts;
        return {
          id: String(m.msg_id),
          from: m.sender_id,
          conversationId: m.conversation_id,
          content: m.content,
          time: timeSec,
          isSent: m.sender_id === myId,
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
    }
  }, [auth.userId]);

  const wsConnect = useCallback(() => {
    if (wsRef.current) wsRef.current.close();
    if (!auth.token) return;
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${proto}//${window.location.host}/ws?token=${encodeURIComponent(auth.token)}`;
    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => setWsConnected(true);
    ws.onclose = () => { setWsConnected(false); wsRef.current = null; };
    ws.onerror = () => setWsConnected(false);

    ws.onmessage = (e) => {
      try {
        const msg: WsMessage = JSON.parse(e.data);
        if (msg.type === 'chat' && msg.from && msg.content && msg.conversation_id) {
          const convId = msg.conversation_id;
          const myId = Number(auth.userId);
          const isSent = String(msg.from) === String(myId);

          const msgId = msg.msg_id != null ? String(msg.msg_id) : `${msg.from}_${msg.timestamp}_${Date.now()}`;
          if (isSent && sentMsgIds.current.has(msgId)) return;
          if (isSent) sentMsgIds.current.add(msgId);
          if (sentMsgIds.current.size > 1000) {
            const arr = Array.from(sentMsgIds.current);
            sentMsgIds.current = new Set(arr.slice(-500));
          }

          const ts = msg.timestamp || 0;
          const timeSec = ts > 1e12 ? ts / 1000 : ts;

          const chatMsg: ChatMessage = {
            id: msgId,
            from: msg.from,
            conversationId: convId,
            content: msg.content,
            time: timeSec,
            isSent,
          };
          setMessages(prev => {
            const existing = prev[convId] || [];
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
          setConversations(prev => {
            const exists = prev.find(c => c.id === convId);
            if (exists) {
              return prev.map(c => c.id === convId
                ? { ...c, lastMsg: msg.content, lastTime: chatMsg.time, unread: c.id === activeConvRef.current ? c.unread : c.unread + 1 }
                : c);
            }
            const newConv: Conversation = {
              id: convId,
              name: `会话 ${convId}`,
              type: 1,
              memberIds: [],
              lastMsg: msg.content,
              lastTime: chatMsg.time,
              unread: isSent ? 0 : 1,
            };
            return [newConv, ...prev];
          });
        }
      } catch {}
    };
  }, [auth.token, auth.userId]);

  const wsDisconnect = useCallback(() => {
    if (wsRef.current) { wsRef.current.close(); wsRef.current = null; }
  }, []);

  const sendMessage = useCallback((convId: number, content: string) => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return;
    const conv = conversationsRef.current.find(c => c.id === convId);
    if (!conv) return;
    const msg: WsMessage = {
      type: 'chat',
      conversation_id: convId,
      content,
    };
    if (conv.type === 1) {
      const myId = Number(auth.userId);
      const peerId = conv.memberIds.find(id => id !== myId);
      if (peerId) {
        msg.peer_id = peerId;
      }
    }
    const tempId = `temp_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
    sentMsgIds.current.add(tempId);
    wsRef.current.send(JSON.stringify(msg));

    const optimisticMsg: ChatMessage = {
      id: tempId,
      from: Number(auth.userId),
      conversationId: convId,
      content,
      time: Date.now() / 1000,
      isSent: true,
    };
    setMessages(prev => ({
      ...prev,
      [convId]: [...(prev[convId] || []), optimisticMsg],
    }));
    setConversations(prev => prev.map(c =>
      c.id === convId ? { ...c, lastMsg: content, lastTime: optimisticMsg.time } : c
    ));
  }, [auth.userId]);

  const addConversation = useCallback((conv: Conversation) => {
    setConversations(prev => {
      if (prev.find(c => c.id === conv.id)) return prev;
      return [conv, ...prev];
    });
  }, []);

  const openChatWith = useCallback((targetId: number, name: string, type: number) => {
    const existing = conversationsRef.current.find(c => {
      if (type === 1) {
        return c.type === 1 && c.memberIds.includes(Number(auth.userId)) && c.memberIds.includes(targetId) && c.memberIds.length === 2;
      }
      return c.id === targetId;
    });
    if (existing) {
      setActiveConvId(existing.id);
      return;
    }
    const tempConv: Conversation = {
      id: 0,
      name,
      type,
      memberIds: type === 1 ? [Number(auth.userId), targetId] : [],
      unread: 0,
    };
    addConversation(tempConv);
    setActiveConvId(0);
  }, [auth.userId, addConversation]);

  useEffect(() => {
    if (auth.token && !wsRef.current) wsConnect();
  }, [auth.token, wsConnect]);

  useEffect(() => {
    if (activeConvId && activeConvId !== 0) {
      loadHistory(activeConvId);
      setConversations(prev => prev.map(c =>
        c.id === activeConvId ? { ...c, unread: 0 } : c
      ));
    }
  }, [activeConvId, loadHistory]);

  return (
    <AppContext.Provider value={{
      auth, conversations, messages, wsConnected, activeConvId, friends,
      login, logout, wsConnect, wsDisconnect, sendMessage, setActiveConvId, addConversation,
      setFriends, openChatWith, loadConversations,
    }}>
      {children}
    </AppContext.Provider>
  );
}
