import { useState } from 'react'
import { useApp } from '../store/AppContext'
import Sidebar from '../components/Sidebar'
import ConvList from '../components/ConvList'
import ChatPanel from '../components/ChatPanel'
import ContactsPanel from '../components/ContactsPanel'
import GroupsPanel from '../components/GroupsPanel'
import './MainPage.css'

type Tab = 'chat' | 'contacts' | 'groups'

export default function MainPage() {
  const { auth, logout, wsConnected, wsConnect, wsDisconnect, activeConvId } = useApp()
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
      />
      <div className="main-middle">
        {tab === 'chat' && <ConvList />}
        {tab === 'contacts' && <ContactsPanel onSwitchToChat={() => setTab('chat')} />}
        {tab === 'groups' && <GroupsPanel onSwitchToChat={() => setTab('chat')} />}
      </div>
      <div className="main-right">
        {showChat ? <ChatPanel /> : (
          <div className="empty-panel">
            <div className="empty-icon">{tab === 'contacts' ? '👤' : tab === 'groups' ? '👥' : '💬'}</div>
            <p>{tab === 'contacts' ? '选择一个联系人开始聊天' : tab === 'groups' ? '选择一个群组查看详情' : '选择一个会话开始聊天'}</p>
          </div>
        )}
      </div>
    </div>
  )
}
