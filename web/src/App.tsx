import { Routes, Route, Link, useLocation, useNavigate } from 'react-router-dom'
import { Container, LayoutDashboard, LogOut, Plus } from 'lucide-react'
import { useAuth } from './lib/auth'
import ProtectedRoute from './components/ProtectedRoute'
import Dashboard from './pages/Dashboard'
import NewProject from './pages/NewProject'
import ProjectDetail from './pages/ProjectDetail'
import Login from './pages/Login'
import Register from './pages/Register'

function App() {
  const location = useLocation()
  const navigate = useNavigate()
  const { user, logout } = useAuth()

  const handleLogout = async () => {
    await logout()
    navigate('/login')
  }

  // Don't show header on login/register pages
  const isAuthPage = location.pathname === '/login' || location.pathname === '/register'

  if (isAuthPage) {
    return (
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/register" element={<Register />} />
      </Routes>
    )
  }

  return (
    <div className="min-h-screen bg-slate-50">
      {/* Header */}
      <header className="bg-white border-b border-slate-200 sticky top-0 z-10">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16">
            <Link to="/" className="flex items-center gap-2 text-slate-900 no-underline">
              <Container className="w-6 h-6 text-blue-600" />
              <span className="text-xl font-bold">devctl</span>
            </Link>
            <nav className="flex items-center gap-4">
              <Link
                to="/"
                className={`flex items-center gap-1.5 px-3 py-2 rounded-md text-sm font-medium no-underline transition-colors ${
                  location.pathname === '/'
                    ? 'bg-blue-50 text-blue-700'
                    : 'text-slate-600 hover:text-slate-900 hover:bg-slate-100'
                }`}
              >
                <LayoutDashboard className="w-4 h-4" />
                Dashboard
              </Link>
              <Link
                to="/new"
                className="flex items-center gap-1.5 px-4 py-2 rounded-md text-sm font-medium bg-blue-600 text-white no-underline hover:bg-blue-700 transition-colors"
              >
                <Plus className="w-4 h-4" />
                Novo Projeto
              </Link>
              {user && (
                <>
                  <span className="text-sm text-slate-500">{user.name}</span>
                  <button
                    onClick={handleLogout}
                    className="flex items-center gap-1.5 px-3 py-2 rounded-md text-sm font-medium text-slate-600 hover:text-slate-900 hover:bg-slate-100 transition-colors bg-transparent border-0 cursor-pointer"
                  >
                    <LogOut className="w-4 h-4" />
                    Sair
                  </button>
                </>
              )}
            </nav>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <Routes>
          <Route path="/" element={<ProtectedRoute><Dashboard /></ProtectedRoute>} />
          <Route path="/new" element={<ProtectedRoute><NewProject /></ProtectedRoute>} />
          <Route path="/projects/:id" element={<ProtectedRoute><ProjectDetail /></ProtectedRoute>} />
        </Routes>
      </main>
    </div>
  )
}

export default App
