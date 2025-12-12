import { useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useAuth } from '../contexts/AuthContext'

export default function AuthCallback() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const { refreshUser } = useAuth()

  useEffect(() => {
    const accessToken = searchParams.get('access_token')

    if (accessToken) {
      sessionStorage.setItem('access_token', accessToken)
      refreshUser().then(() => {
        navigate('/dashboard')
      })
    } else {
      console.error('No access token in callback')
      navigate('/login')
    }
  }, [searchParams, navigate, refreshUser])

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900">
      <div className="text-center">
        <div className="text-lg text-gray-900 dark:text-white">
          Completing sign in...
        </div>
      </div>
    </div>
  )
}
