import axios from 'axios'

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

export const api = axios.create({
  baseURL: API_URL,
  withCredentials: true,
})

// Add token to requests
api.interceptors.request.use((config) => {
  const token = sessionStorage.getItem('access_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Handle 401 errors
api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config

    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true

      try {
        const response = await api.post('/api/auth/refresh')
        const { access_token } = response.data

        sessionStorage.setItem('access_token', access_token)
        originalRequest.headers.Authorization = `Bearer ${access_token}`

        return api(originalRequest)
      } catch (refreshError) {
        sessionStorage.removeItem('access_token')
        window.location.href = '/login'
        return Promise.reject(refreshError)
      }
    }

    return Promise.reject(error)
  }
)

export interface User {
  id: string
  email: string
  name: string
  avatar_url?: string
  role: string
  is_super_admin: boolean
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface OrganizationClaim {
  id: string
  slug: string
  name: string
  role: string
}

export interface ListUsersResponse {
  users: User[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

// Auth API
export const authAPI = {
  login: () => {
    window.location.href = `${API_URL}/api/auth/google/login`
  },

  logout: async () => {
    await api.post('/api/auth/logout')
    sessionStorage.removeItem('access_token')
  },

  getCurrentUser: async (): Promise<User> => {
    const response = await api.get('/api/auth/me')
    return response.data
  },

  refreshToken: async () => {
    const response = await api.post('/api/auth/refresh')
    return response.data
  },
}

// Users API
export const usersAPI = {
  list: async (page = 1, pageSize = 20, applicationId?: string): Promise<ListUsersResponse> => {
    const params: any = { page, page_size: pageSize }
    if (applicationId) {
      params.application_id = applicationId
    }
    const response = await api.get('/api/users', { params })
    return response.data
  },

  get: async (id: string): Promise<User> => {
    const response = await api.get(`/api/users/${id}`)
    return response.data
  },

  update: async (id: string, data: Partial<User>): Promise<User> => {
    const response = await api.put(`/api/users/${id}`, data)
    return response.data
  },

  delete: async (id: string): Promise<void> => {
    await api.delete(`/api/users/${id}`)
  },

  activate: async (id: string): Promise<User> => {
    const response = await api.post(`/api/users/${id}/activate`)
    return response.data
  },

  deactivate: async (id: string): Promise<User> => {
    const response = await api.post(`/api/users/${id}/deactivate`)
    return response.data
  },
}

// Application Types (replaces AllowedOrigin)
export interface Application {
  id: string
  name: string
  slug: string
  origin: string
  description?: string
  redirect_uris?: string[]
  is_active: boolean
  created_at: string
  updated_at: string
  created_by?: string
}

export interface CreateApplicationRequest {
  name: string
  slug?: string
  origin: string
  description?: string
  redirect_uris?: string[]
  is_active?: boolean
}

export interface UpdateApplicationRequest {
  name?: string
  slug?: string
  origin?: string
  description?: string
  redirect_uris?: string[]
  is_active?: boolean
}

// Applications API (replaces originsAPI)
export const applicationsAPI = {
  list: async (): Promise<Application[]> => {
    const response = await api.get('/api/applications')
    return response.data
  },

  get: async (id: string): Promise<Application> => {
    const response = await api.get(`/api/applications/${id}`)
    return response.data
  },

  create: async (data: CreateApplicationRequest): Promise<Application> => {
    const response = await api.post('/api/applications', data)
    return response.data
  },

  update: async (id: string, data: UpdateApplicationRequest): Promise<Application> => {
    const response = await api.put(`/api/applications/${id}`, data)
    return response.data
  },

  delete: async (id: string): Promise<void> => {
    await api.delete(`/api/applications/${id}`)
  },

  reloadCORS: async (): Promise<void> => {
    await api.post('/api/applications/reload-cors')
  },

  getLogins: async (id: string): Promise<any[]> => {
    const response = await api.get(`/api/applications/${id}/logins`)
    return response.data
  },
}

// Backward compatibility aliases
export const originsAPI = applicationsAPI
export type AllowedOrigin = Application
export type CreateOriginRequest = CreateApplicationRequest
export type UpdateOriginRequest = UpdateApplicationRequest

// Organization Types
export interface Organization {
  id: string
  name: string
  slug: string
  description?: string
  is_active: boolean
  created_at: string
  updated_at: string
  created_by?: string
}

export interface CreateOrganizationRequest {
  name: string
  slug?: string
  description?: string
  is_active?: boolean
}

export interface UpdateOrganizationRequest {
  name?: string
  slug?: string
  description?: string
  is_active?: boolean
}

export interface OrganizationMember {
  id: string
  user_id: string
  organization_id: string
  role: string
  email: string
  name: string
  avatar_url?: string
  created_at: string
  updated_at: string
}

export interface AddMemberRequest {
  user_id?: string
  email?: string
  role: string
}

export interface UpdateMemberRequest {
  role: string
}

// Organizations API
export const organizationsAPI = {
  list: async (): Promise<Organization[]> => {
    const response = await api.get('/api/organizations')
    return response.data
  },

  get: async (id: string): Promise<Organization> => {
    const response = await api.get(`/api/organizations/${id}`)
    return response.data
  },

  create: async (data: CreateOrganizationRequest): Promise<Organization> => {
    const response = await api.post('/api/organizations', data)
    return response.data
  },

  update: async (id: string, data: UpdateOrganizationRequest): Promise<Organization> => {
    const response = await api.put(`/api/organizations/${id}`, data)
    return response.data
  },

  delete: async (id: string): Promise<void> => {
    await api.delete(`/api/organizations/${id}`)
  },

  // Members
  listMembers: async (id: string): Promise<OrganizationMember[]> => {
    const response = await api.get(`/api/organizations/${id}/members`)
    return response.data
  },

  addMember: async (id: string, data: AddMemberRequest): Promise<OrganizationMember> => {
    const response = await api.post(`/api/organizations/${id}/members`, data)
    return response.data
  },

  updateMember: async (id: string, userId: string, data: UpdateMemberRequest): Promise<OrganizationMember> => {
    const response = await api.put(`/api/organizations/${id}/members/${userId}`, data)
    return response.data
  },

  removeMember: async (id: string, userId: string): Promise<void> => {
    await api.delete(`/api/organizations/${id}/members/${userId}`)
  },

  getLogins: async (id: string): Promise<any[]> => {
    const response = await api.get(`/api/organizations/${id}/logins`)
    return response.data
  },
}

// User Organizations
export const userOrganizationsAPI = {
  getUserOrganizations: async (userId: string): Promise<any[]> => {
    const response = await api.get(`/api/users/${userId}/organizations`)
    return response.data
  },

  getUserLogins: async (userId: string): Promise<any[]> => {
    const response = await api.get(`/api/users/${userId}/logins`)
    return response.data
  },
}

// Login Tracking
export const trackingAPI = {
  trackLogin: async (data: { application_slug?: string; application_id?: string; organization_id?: string }): Promise<void> => {
    await api.post('/api/track-login', data)
  },
}

// Statistics Types
export interface LoginStats {
  total_logins: number
  unique_users: number
  unique_applications: number
  logins_last_24_hours: number
  logins_last_7_days: number
  logins_last_30_days: number
}

export interface UserLoginStats {
  user_id: string
  email: string
  name: string
  login_count: number
  unique_apps: number
  last_login: string | null
}

export interface AppLoginStats {
  app_id: string
  name: string
  slug: string
  login_count: number
  unique_users: number
  last_login: string | null
}

export interface MyLoginStats {
  total_logins: number
  unique_applications: number
  logins_last_7_days: number
  logins_last_30_days: number
}

export interface MyAppLoginStats {
  app_id: string
  name: string
  slug: string
  login_count: number
  last_login: string
}

// Statistics API
export const statsAPI = {
  // Admin stats
  getOverallStats: async (): Promise<LoginStats> => {
    const response = await api.get('/api/stats/logins')
    return response.data
  },

  getUserStats: async (): Promise<UserLoginStats[]> => {
    const response = await api.get('/api/stats/users')
    return response.data
  },

  getApplicationStats: async (): Promise<AppLoginStats[]> => {
    const response = await api.get('/api/stats/applications')
    return response.data
  },

  // Personal stats
  getMyStats: async (): Promise<MyLoginStats> => {
    const response = await api.get('/api/me/stats')
    return response.data
  },

  getMyLoginsByApp: async (): Promise<MyAppLoginStats[]> => {
    const response = await api.get('/api/me/logins-by-app')
    return response.data
  },
}
