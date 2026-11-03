import React from 'react'
import ReactDOM from 'react-dom/client'
import { AppProvider } from './store/AppContext.tsx'
import App from './App.tsx'
import AdminApp from './admin/AdminApp.tsx'
import './styles/global.css'

// 管理后台（/admin）是独立入口：不挂载聊天端 Provider，登录态与普通用户隔离。
const isAdminRoute = window.location.pathname.startsWith('/admin')

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    {isAdminRoute ? (
      <AdminApp />
    ) : (
      <AppProvider>
        <App />
      </AppProvider>
    )}
  </React.StrictMode>,
)
