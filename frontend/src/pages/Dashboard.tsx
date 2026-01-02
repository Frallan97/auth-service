import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../contexts/AuthContext'
import UserDashboard from './User/Dashboard'

export default function Dashboard() {
  const { isAdmin, loading } = useAuth()
  const navigate = useNavigate()

  useEffect(() => {
    // Redirect admins to the admin users page
    if (!loading && isAdmin) {
      navigate('/admin/users')
    }
  }, [isAdmin, loading, navigate])

  // Show loading state while checking auth
  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-gray-600 dark:text-gray-400">Loading...</div>
      </div>
    )
  }

  // Show user dashboard for non-admin users
  return <UserDashboard />
}
