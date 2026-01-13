import { useAuth } from '../../contexts/AuthContext'
import { Link, useLocation } from 'react-router-dom'

export default function Header() {
  const { user, logout } = useAuth()
  const location = useLocation()

  const navLinks = user?.role === 'admin' ? [
    { to: '/admin/users', label: 'Users' },
    { to: '/admin/applications', label: 'Applications' },
    { to: '/admin/organizations', label: 'Organizations' },
    { to: '/admin/analytics', label: 'Analytics' },
  ] : [
    { to: '/dashboard', label: 'Dashboard' },
  ]

  return (
    <header className="bg-white dark:bg-gray-800 shadow">
      <nav className="container mx-auto px-4 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-8">
            <div className="text-xl font-bold text-gray-900 dark:text-white">
              Auth Service
            </div>
            {user && (
              <div className="flex space-x-4">
                {navLinks.map((link) => (
                  <Link
                    key={link.to}
                    to={link.to}
                    className={`px-3 py-2 text-sm font-medium rounded-md transition-colors ${
                      location.pathname === link.to
                        ? 'bg-blue-600 text-white'
                        : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
                    }`}
                  >
                    {link.label}
                  </Link>
                ))}
              </div>
            )}
          </div>

          {user && (
            <div className="flex items-center space-x-4">
              <div className="flex items-center space-x-3">
                {user.avatar_url && (
                  <img
                    src={user.avatar_url}
                    alt={user.name}
                    className="w-8 h-8 rounded-full"
                  />
                )}
                <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {user.name}
                </span>
              </div>
              <button
                onClick={logout}
                className="px-4 py-2 text-sm font-medium text-white bg-red-600 rounded-md hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500"
              >
                Logout
              </button>
            </div>
          )}
        </div>
      </nav>
    </header>
  )
}
