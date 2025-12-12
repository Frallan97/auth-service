import { useAuth } from '../contexts/AuthContext'

export default function Dashboard() {
  const { user } = useAuth()

  return (
    <div className="max-w-7xl mx-auto">
      <h1 className="text-3xl font-bold text-gray-900 dark:text-white mb-8">
        Dashboard
      </h1>

      <div className="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-4">
          Welcome, {user?.name}!
        </h2>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <p className="text-sm font-medium text-gray-500 dark:text-gray-400">
              Email
            </p>
            <p className="mt-1 text-sm text-gray-900 dark:text-white">
              {user?.email}
            </p>
          </div>

          <div>
            <p className="text-sm font-medium text-gray-500 dark:text-gray-400">
              Account Status
            </p>
            <p className="mt-1 text-sm">
              <span
                className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                  user?.is_active
                    ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                    : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                }`}
              >
                {user?.is_active ? 'Active' : 'Inactive'}
              </span>
            </p>
          </div>

          <div>
            <p className="text-sm font-medium text-gray-500 dark:text-gray-400">
              User ID
            </p>
            <p className="mt-1 text-sm text-gray-900 dark:text-white font-mono">
              {user?.id}
            </p>
          </div>

          <div>
            <p className="text-sm font-medium text-gray-500 dark:text-gray-400">
              Member Since
            </p>
            <p className="mt-1 text-sm text-gray-900 dark:text-white">
              {user?.created_at ? new Date(user.created_at).toLocaleDateString() : 'N/A'}
            </p>
          </div>
        </div>
      </div>

      <div className="mt-8 bg-blue-50 dark:bg-blue-900 border-l-4 border-blue-400 p-4">
        <div className="flex">
          <div className="flex-shrink-0">
            <svg
              className="h-5 w-5 text-blue-400"
              viewBox="0 0 20 20"
              fill="currentColor"
            >
              <path
                fillRule="evenodd"
                d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z"
                clipRule="evenodd"
              />
            </svg>
          </div>
          <div className="ml-3">
            <p className="text-sm text-blue-700 dark:text-blue-200">
              This is a centralized authentication service. Your JWT token can be used to
              authenticate with other services in the ecosystem.
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
