import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider } from './contexts/AuthContext'
import { ProtectedRoute } from './components/Auth/ProtectedRoute'
import AdminRoute from './components/Auth/AdminRoute'
import LoginPage from './pages/Login'
import AuthCallback from './pages/AuthCallback'
import Dashboard from './pages/Dashboard'
import Users from './pages/Admin/Users'
import Origins from './pages/Admin/Origins'
import Applications from './pages/Admin/Applications'
import Organizations from './pages/Admin/Organizations'
import OrganizationMembers from './pages/Admin/OrganizationMembers'
import Layout from './components/Layout/Layout'

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/auth/callback" element={<AuthCallback />} />

          <Route element={<Layout />}>
            <Route
              path="/dashboard"
              element={
                <ProtectedRoute>
                  <Dashboard />
                </ProtectedRoute>
              }
            />
            <Route
              path="/admin/users"
              element={
                <ProtectedRoute>
                  <AdminRoute>
                    <Users />
                  </AdminRoute>
                </ProtectedRoute>
              }
            />
            <Route
              path="/admin/origins"
              element={
                <ProtectedRoute>
                  <AdminRoute>
                    <Origins />
                  </AdminRoute>
                </ProtectedRoute>
              }
            />
            <Route
              path="/admin/applications"
              element={
                <ProtectedRoute>
                  <AdminRoute>
                    <Applications />
                  </AdminRoute>
                </ProtectedRoute>
              }
            />
            <Route
              path="/admin/organizations"
              element={
                <ProtectedRoute>
                  <AdminRoute>
                    <Organizations />
                  </AdminRoute>
                </ProtectedRoute>
              }
            />
            <Route
              path="/admin/organizations/:id/members"
              element={
                <ProtectedRoute>
                  <AdminRoute>
                    <OrganizationMembers />
                  </AdminRoute>
                </ProtectedRoute>
              }
            />
          </Route>

          <Route path="/" element={<Navigate to="/dashboard" replace />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}

export default App
