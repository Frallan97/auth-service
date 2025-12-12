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
  is_active: boolean
  created_at: string
  updated_at: string
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
  list: async (page = 1, pageSize = 20): Promise<ListUsersResponse> => {
    const response = await api.get('/api/users', {
      params: { page, page_size: pageSize },
    })
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
