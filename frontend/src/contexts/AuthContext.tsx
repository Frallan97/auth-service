import { createContext, useContext, useState, useEffect, ReactNode } from 'react'
import { authAPI, User } from '../services/api'

interface AuthContextType {
  user: User | null
  loading: boolean
  isAdmin: boolean
  login: () => void
  logout: () => Promise<void>
  refreshUser: () => Promise<void>
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  // Compute isAdmin based on user role
  const isAdmin = user?.role === 'admin'

  useEffect(() => {
    checkAuth()
  }, [])

  const checkAuth = async () => {
    const token = sessionStorage.getItem('access_token')
    if (!token) {
      setLoading(false)
      return
    }

    try {
      const currentUser = await authAPI.getCurrentUser()
      setUser(currentUser)
    } catch (error) {
      console.error('Failed to get current user:', error)
      sessionStorage.removeItem('access_token')
    } finally {
      setLoading(false)
    }
  }

  const login = () => {
    authAPI.login()
  }

  const logout = async () => {
    try {
      await authAPI.logout()
    } catch (error) {
      console.error('Logout failed:', error)
    } finally {
      setUser(null)
      sessionStorage.removeItem('access_token')
      window.location.href = '/login'
    }
  }

  const refreshUser = async () => {
    try {
      const currentUser = await authAPI.getCurrentUser()
      setUser(currentUser)
    } catch (error) {
      console.error('Failed to refresh user:', error)
    }
  }

  return (
    <AuthContext.Provider value={{ user, loading, isAdmin, login, logout, refreshUser }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
