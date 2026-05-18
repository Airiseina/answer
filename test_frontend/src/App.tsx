import { useApp } from './store/AppContext'
import LoginPage from './pages/LoginPage'
import MainPage from './pages/MainPage'

export default function App() {
  const { auth } = useApp()
  return auth.token ? <MainPage /> : <LoginPage />
}
