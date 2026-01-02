import { useAuth } from '../../contexts/AuthContext'

export default function UserDashboard() {
  const { user } = useAuth()

  return (
    <div className="max-w-4xl mx-auto p-6">
      <h1 className="text-3xl font-bold text-gray-900 dark:text-white mb-8">
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
