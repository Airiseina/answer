import { useState } from 'react'
import { useApp } from '../store/AppContext.tsx'
import Sidebar from '../components/Sidebar.tsx'
import ConvList from '../components/ConvList.tsx'
import ChatPanel from '../components/ChatPanel.tsx'
import ContactsPanel from '../components/ContactsPanel.tsx'
import GroupsPanel from '../components/GroupsPanel.tsx'
import BotsPanel from '../components/BotsPanel.tsx'
import './MainPage.css'

type Tab = 'chat' | 'contacts' | 'groups' | 'bots'

export default function MainPage() {
  const { auth, logout, wsConnected, wsConnect, wsDisconnect, activeConvId, openSystemAI } = useApp()
  const [tab, setTab] = useState<Tab>('chat')

  const showChat = tab === 'chat' || (activeConvId !== null)

  return (
    <div className="main-page">
      <Sidebar
        account={auth.account}
        wsConnected={wsConnected}
        tab={tab}
        onTabChange={setTab}
        onWsToggle={wsConnected ? wsDisconnect : wsConnect}
        onLogout={logout}
        onOpenSystemAI={openSystemAI}
      />
      <div className="main-middle">
        {tab === 'chat' && <ConvList />}
        {tab === 'contacts' && <ContactsPanel onSwitchToChat={() => setTab('chat')} />}
        {tab === 'groups' && <GroupsPanel onSwitchToChat={() => setTab('chat')} />}
        {tab === 'bots' && <BotsPanel onSwitchToChat={() => setTab('chat')} />}
      </div>
      <div className="main-right">
        {showChat ? <ChatPanel /> : (
          <div className="empty-panel">
            <div className="empty-icon">{tab === 'contacts' ? '👤' : tab === 'groups' ? '👥' : tab === 'bots' ? '🤖' : '💬'}</div>
            <p>{tab === 'contacts' ? '选择一个联系人开始聊天' : tab === 'groups' ? '选择一个群组查看详情' : tab === 'bots' ? '创建或选择一个 AI Bot' : '选择一个会话开始聊天'}</p>
          </div>
        )}
      </div>
    </div>
  )
}
