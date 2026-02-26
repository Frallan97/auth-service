import { useState, useEffect } from 'react'
import {
  AreaChart, Area, BarChart, Bar, XAxis, YAxis, CartesianGrid,
  Tooltip, ResponsiveContainer,
} from 'recharts'
import {
  statsAPI, LoginStats, UserLoginStats, AppLoginStats,
  DailyLoginStats, AppUserStats,
} from '../../services/api'

type Tab = 'overview' | 'users' | 'applications'
type TimeRange = 7 | 30 | 90

export default function Analytics() {
  const [overallStats, setOverallStats] = useState<LoginStats | null>(null)
  const [userStats, setUserStats] = useState<UserLoginStats[]>([])
  const [appStats, setAppStats] = useState<AppLoginStats[]>([])
  const [dailyStats, setDailyStats] = useState<DailyLoginStats[]>([])
  const [loading, setLoading] = useState(true)
  const [activeTab, setActiveTab] = useState<Tab>('overview')
  const [timeRange, setTimeRange] = useState<TimeRange>(30)

  // App user drill-down state
  const [expandedAppId, setExpandedAppId] = useState<string | null>(null)
  const [appUsers, setAppUsers] = useState<AppUserStats[]>([])
  const [appUsersLoading, setAppUsersLoading] = useState(false)

  useEffect(() => {
    loadStats()
  }, [])

  useEffect(() => {
    loadDailyStats()
  }, [timeRange])

  const loadStats = async () => {
    try {
      setLoading(true)
      const [overall, users, apps, daily] = await Promise.all([
        statsAPI.getOverallStats(),
        statsAPI.getUserStats(),
        statsAPI.getApplicationStats(),
        statsAPI.getDailyStats(timeRange),
      ])
      setOverallStats(overall)
      setUserStats(users)
      setAppStats(apps)
      setDailyStats(daily)
    } catch (error) {
      console.error('Failed to load statistics:', error)
    } finally {
      setLoading(false)
    }
  }

  const loadDailyStats = async () => {
    try {
      const daily = await statsAPI.getDailyStats(timeRange)
      setDailyStats(daily)
    } catch (error) {
      console.error('Failed to load daily stats:', error)
    }
  }

  const toggleAppUsers = async (appId: string) => {
    if (expandedAppId === appId) {
      setExpandedAppId(null)
      setAppUsers([])
      return
    }
    try {
      setExpandedAppId(appId)
      setAppUsersLoading(true)
      const users = await statsAPI.getApplicationUsers(appId)
      setAppUsers(users)
    } catch (error) {
      console.error('Failed to load application users:', error)
    } finally {
      setAppUsersLoading(false)
    }
  }

  const formatDate = (dateString: string | null) => {
    if (!dateString) return 'Never'
    return new Date(dateString).toLocaleString()
  }

  const formatShortDate = (dateString: string) => {
    const d = new Date(dateString)
    return `${d.getMonth() + 1}/${d.getDate()}`
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-gray-600 dark:text-gray-400">Loading statistics...</div>
      </div>
    )
  }

  const tabClass = (tab: Tab) =>
    `${
      activeTab === tab
        ? 'border-blue-500 text-blue-600 dark:text-blue-400'
        : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
    } whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm`

  const timeRangeBtn = (range_: TimeRange) =>
    `px-3 py-1 text-sm rounded-md ${
      timeRange === range_
        ? 'bg-blue-600 text-white'
        : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600'
    }`

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
      <h1 className="text-2xl sm:text-3xl font-bold text-gray-900 dark:text-white mb-8">Login Analytics</h1>

      {/* Tabs */}
      <div className="border-b border-gray-200 dark:border-gray-700 mb-6">
        <nav className="-mb-px flex space-x-4 sm:space-x-8 overflow-x-auto">
          <button onClick={() => setActiveTab('overview')} className={tabClass('overview')}>Overview</button>
          <button onClick={() => setActiveTab('applications')} className={tabClass('applications')}>Applications</button>
          <button onClick={() => setActiveTab('users')} className={tabClass('users')}>Users</button>
        </nav>
      </div>

      {/* Overview Tab */}
      {activeTab === 'overview' && overallStats && (
        <div className="space-y-8">
          {/* Stat Cards */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            <StatCard label="Total Logins" value={overallStats.total_logins} />
            <StatCard label="Unique Users" value={overallStats.unique_users} />
            <StatCard label="Unique Applications" value={overallStats.unique_applications} />
            <StatCard label="Last 24 Hours" value={overallStats.logins_last_24_hours} highlight />
            <StatCard label="Last 7 Days" value={overallStats.logins_last_7_days} highlight />
            <StatCard label="Last 30 Days" value={overallStats.logins_last_30_days} highlight />
          </div>

          {/* Daily Login Trend Chart */}
          <div className="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Daily Login Trend</h2>
              <div className="flex gap-2">
                <button onClick={() => setTimeRange(7)} className={timeRangeBtn(7)}>7d</button>
                <button onClick={() => setTimeRange(30)} className={timeRangeBtn(30)}>30d</button>
                <button onClick={() => setTimeRange(90)} className={timeRangeBtn(90)}>90d</button>
              </div>
            </div>
            <div className="h-72">
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={dailyStats}>
                  <defs>
                    <linearGradient id="colorLogins" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.3} />
                      <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="#374151" opacity={0.3} />
                  <XAxis
                    dataKey="date"
                    tickFormatter={formatShortDate}
                    stroke="#9ca3af"
                    fontSize={12}
                  />
                  <YAxis stroke="#9ca3af" fontSize={12} allowDecimals={false} />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: '#1f2937',
                      border: '1px solid #374151',
                      borderRadius: '8px',
                      color: '#f3f4f6',
                    }}
                    labelFormatter={(label) => new Date(label).toLocaleDateString()}
                  />
                  <Area
                    type="monotone"
                    dataKey="count"
                    stroke="#3b82f6"
                    fillOpacity={1}
                    fill="url(#colorLogins)"
                    strokeWidth={2}
                    name="Logins"
                  />
                </AreaChart>
              </ResponsiveContainer>
            </div>
          </div>
        </div>
      )}

      {/* Applications Tab */}
      {activeTab === 'applications' && (
        <div className="space-y-8">
          {/* Bar Chart */}
          {appStats.length > 0 && (
            <div className="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Logins by Application</h2>
              <div className="h-72">
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={appStats}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#374151" opacity={0.3} />
                    <XAxis dataKey="name" stroke="#9ca3af" fontSize={12} />
                    <YAxis stroke="#9ca3af" fontSize={12} allowDecimals={false} />
                    <Tooltip
                      contentStyle={{
                        backgroundColor: '#1f2937',
                        border: '1px solid #374151',
                        borderRadius: '8px',
                        color: '#f3f4f6',
                      }}
                    />
                    <Bar dataKey="login_count" fill="#3b82f6" radius={[4, 4, 0, 0]} name="Total Logins" />
                    <Bar dataKey="unique_users" fill="#10b981" radius={[4, 4, 0, 0]} name="Unique Users" />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            </div>
          )}

          {/* Applications Table */}
          <div className="bg-white dark:bg-gray-800 shadow rounded-lg overflow-hidden">
            <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
              <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Application Login Statistics</h2>
              <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                Click an application to see its users
              </p>
            </div>
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                <thead className="bg-gray-50 dark:bg-gray-900">
                  <tr>
                    <TH>Application</TH>
                    <TH>Total Logins</TH>
                    <TH>Unique Users</TH>
                    <TH>Last Login</TH>
                  </tr>
                </thead>
                <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                  {appStats.map((app) => (
                    <>
                      <tr
                        key={app.app_id}
                        onClick={() => toggleAppUsers(app.app_id)}
                        className="hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer"
                      >
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div>
                            <div className="text-sm font-medium text-gray-900 dark:text-white">{app.name}</div>
                            <div className="text-sm text-gray-500 dark:text-gray-400">{app.slug}</div>
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <span className="text-sm text-gray-900 dark:text-white font-semibold">{app.login_count}</span>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <span className="text-sm text-gray-900 dark:text-white">{app.unique_users}</span>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                          {formatDate(app.last_login)}
                        </td>
                      </tr>
                      {expandedAppId === app.app_id && (
                        <tr key={`${app.app_id}-users`}>
                          <td colSpan={4} className="px-6 py-4 bg-gray-50 dark:bg-gray-900">
                            {appUsersLoading ? (
                              <div className="text-sm text-gray-500 dark:text-gray-400 py-2">Loading users...</div>
                            ) : appUsers.length === 0 ? (
                              <div className="text-sm text-gray-500 dark:text-gray-400 py-2">No users found</div>
                            ) : (
                              <div>
                                <h4 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">
                                  Users ({appUsers.length})
                                </h4>
                                <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                                  <thead>
                                    <tr>
                                      <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">User</th>
                                      <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Logins</th>
                                      <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Last Login</th>
                                    </tr>
                                  </thead>
                                  <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                                    {appUsers.map((user) => (
                                      <tr key={user.user_id}>
                                        <td className="px-4 py-2">
                                          <div className="flex items-center gap-2">
                                            {user.avatar_url && (
                                              <img src={user.avatar_url} alt="" className="w-6 h-6 rounded-full" />
                                            )}
                                            <div>
                                              <div className="text-sm font-medium text-gray-900 dark:text-white">{user.name}</div>
                                              <div className="text-xs text-gray-500 dark:text-gray-400">{user.email}</div>
                                            </div>
                                          </div>
                                        </td>
                                        <td className="px-4 py-2 text-sm text-gray-900 dark:text-white font-semibold">
                                          {user.login_count}
                                        </td>
                                        <td className="px-4 py-2 text-sm text-gray-500 dark:text-gray-400">
                                          {formatDate(user.last_login)}
                                        </td>
                                      </tr>
                                    ))}
                                  </tbody>
                                </table>
                              </div>
                            )}
                          </td>
                        </tr>
                      )}
                    </>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* Users Tab */}
      {activeTab === 'users' && (
        <div className="bg-white dark:bg-gray-800 shadow rounded-lg overflow-hidden">
          <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h2 className="text-xl font-semibold text-gray-900 dark:text-white">User Login Statistics</h2>
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
              Showing top 100 users by login count
            </p>
          </div>
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
              <thead className="bg-gray-50 dark:bg-gray-900">
                <tr>
                  <TH>User</TH>
                  <TH>Total Logins</TH>
                  <TH>Unique Apps</TH>
                  <TH>Last Login</TH>
                </tr>
              </thead>
              <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                {userStats.map((user) => (
                  <tr key={user.user_id} className="hover:bg-gray-50 dark:hover:bg-gray-700">
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div>
                        <div className="text-sm font-medium text-gray-900 dark:text-white">{user.name}</div>
                        <div className="text-sm text-gray-500 dark:text-gray-400">{user.email}</div>
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className="text-sm text-gray-900 dark:text-white font-semibold">{user.login_count}</span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className="text-sm text-gray-900 dark:text-white">{user.unique_apps}</span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                      {formatDate(user.last_login)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}

function StatCard({ label, value, highlight }: { label: string; value: number; highlight?: boolean }) {
  return (
    <div className="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
      <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">{label}</h3>
      <p className={`text-3xl font-bold ${highlight ? 'text-blue-600 dark:text-blue-400' : 'text-gray-900 dark:text-white'}`}>
        {value}
      </p>
    </div>
  )
}

function TH({ children }: { children: React.ReactNode }) {
  return (
    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
      {children}
    </th>
  )
}
