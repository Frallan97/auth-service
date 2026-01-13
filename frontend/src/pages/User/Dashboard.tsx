import { useState, useEffect } from 'react'
import { useAuth } from '../../contexts/AuthContext'
import { statsAPI, MyLoginStats, MyAppLoginStats } from '../../services/api'

export default function UserDashboard() {
  const { user } = useAuth()
  const [stats, setStats] = useState<MyLoginStats | null>(null)
  const [appLogins, setAppLogins] = useState<MyAppLoginStats[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    loadStats()
  }, [])

  const loadStats = async () => {
    try {
      setLoading(true)
      const [myStats, myAppLogins] = await Promise.all([
        statsAPI.getMyStats(),
        statsAPI.getMyLoginsByApp(),
      ])
      setStats(myStats)
      setAppLogins(myAppLogins)
    } catch (error) {
      console.error('Failed to load statistics:', error)
    } finally {
      setLoading(false)
    }
  }

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString()
  }

  return (
    <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
      <h1 className="text-2xl sm:text-3xl font-bold text-gray-900 dark:text-white mb-8">
        Welcome, {user?.name}
      </h1>

      <div className="bg-white dark:bg-gray-800 shadow rounded-lg p-6 mb-6">
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-4">
          Your Profile
        </h2>
        <div className="space-y-3">
          <div>
            <span className="text-sm text-gray-500 dark:text-gray-400">Email:</span>
            <p className="text-gray-900 dark:text-white">{user?.email}</p>
          </div>
          <div>
            <span className="text-sm text-gray-500 dark:text-gray-400">Name:</span>
            <p className="text-gray-900 dark:text-white">{user?.name}</p>
          </div>
          <div>
            <span className="text-sm text-gray-500 dark:text-gray-400">Status:</span>
            <span
              className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ml-2 ${
                user?.is_active
                  ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                  : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
              }`}
            >
              {user?.is_active ? 'Active' : 'Inactive'}
            </span>
          </div>
        </div>
      </div>

      {/* Login Statistics */}
      {!loading && stats && (
        <div className="bg-white dark:bg-gray-800 shadow rounded-lg p-6 mb-6">
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-4">
            Your Login Activity
          </h2>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
            <div className="bg-blue-50 dark:bg-blue-900/20 rounded-lg p-4">
              <p className="text-sm text-gray-600 dark:text-gray-400 mb-1">Total Logins</p>
              <p className="text-2xl font-bold text-blue-600 dark:text-blue-400">{stats.total_logins}</p>
            </div>
            <div className="bg-green-50 dark:bg-green-900/20 rounded-lg p-4">
              <p className="text-sm text-gray-600 dark:text-gray-400 mb-1">Applications</p>
              <p className="text-2xl font-bold text-green-600 dark:text-green-400">{stats.unique_applications}</p>
            </div>
            <div className="bg-purple-50 dark:bg-purple-900/20 rounded-lg p-4">
              <p className="text-sm text-gray-600 dark:text-gray-400 mb-1">Last 7 Days</p>
              <p className="text-2xl font-bold text-purple-600 dark:text-purple-400">{stats.logins_last_7_days}</p>
            </div>
            <div className="bg-orange-50 dark:bg-orange-900/20 rounded-lg p-4">
              <p className="text-sm text-gray-600 dark:text-gray-400 mb-1">Last 30 Days</p>
              <p className="text-2xl font-bold text-orange-600 dark:text-orange-400">{stats.logins_last_30_days}</p>
            </div>
          </div>

          {/* Application breakdown */}
          {appLogins.length > 0 && (
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-3">Login History by Application</h3>
              <div className="space-y-3">
                {appLogins.map((app) => (
                  <div
                    key={app.app_id}
                    className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg"
                  >
                    <div className="flex-1">
                      <p className="text-sm font-medium text-gray-900 dark:text-white">{app.name}</p>
                      <p className="text-xs text-gray-500 dark:text-gray-400">Last login: {formatDate(app.last_login)}</p>
                    </div>
                    <div className="text-right">
                      <p className="text-lg font-semibold text-gray-900 dark:text-white">{app.login_count}</p>
                      <p className="text-xs text-gray-500 dark:text-gray-400">logins</p>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {appLogins.length === 0 && (
            <p className="text-gray-600 dark:text-gray-400 text-sm">
              No login activity yet. Your logins to integrated applications will appear here.
            </p>
          )}
        </div>
      )}

      <div className="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-4">
          Authentication Service
        </h2>
        <p className="text-gray-700 dark:text-gray-300">
          You are successfully authenticated with this service. Your account provides
          single sign-on access to integrated applications.
        </p>
      </div>
    </div>
  )
}
