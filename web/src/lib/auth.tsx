import { createContext, useContext, type ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type User } from './api'

interface AuthContextType {
  user: User | null
  isLoading: boolean
  login: (email: string, password: string) => Promise<User>
  register: (name: string, email: string, password: string) => Promise<User>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()

  const { data: user, isLoading } = useQuery({
    queryKey: ['auth', 'me'],
    queryFn: api.me,
    retry: false,
  })

  const login = async (email: string, password: string) => {
    const u = await api.login({ email, password })
    queryClient.setQueryData(['auth', 'me'], u)
    return u
  }

  const register = async (name: string, email: string, password: string) => {
    const u = await api.register({ name, email, password })
    queryClient.setQueryData(['auth', 'me'], u)
    return u
  }

  const logout = async () => {
    await api.logout()
    queryClient.setQueryData(['auth', 'me'], null)
    queryClient.clear()
  }

  return (
    <AuthContext.Provider value={{ user: user ?? null, isLoading, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
