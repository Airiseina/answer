import { useApp } from './store/AppContext.tsx'
import LoginPage from './pages/LoginPage.tsx'
import MainPage from './pages/MainPage.tsx'

export default function App() {
  const { auth } = useApp()
  return auth.token ? <MainPage /> : <LoginPage />
}
