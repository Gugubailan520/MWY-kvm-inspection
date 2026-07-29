import axios from 'axios'

const http = axios.create({ baseURL: '', timeout: 15000 })

http.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

http.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('token')
      window.location.href = '/login'
    }
    return Promise.reject(err)
  },
)

export default http

// ---- Auth ----
export const login = (username: string, password: string) =>
  http.post('/api/login', { username, password })

// ---- Nodes ----
export const listNodes = () => http.get('/api/nodes')
export const createNode = (data: { name: string; ip: string; os: string; virt: string }) =>
  http.post('/api/nodes', data)
export const deleteNode = (id: string) => http.delete(`/api/nodes/${id}`)

// ---- Events ----
export const listEvents = (params: Record<string, any>) =>
  http.get('/api/events', { params })

// ---- Rules ----
export const listRules = () => http.get('/api/rules')
export const createRule = (data: any) => http.post('/api/rules', data)
export const updateRule = (id: number, data: any) => http.put(`/api/rules/${id}`, data)
export const deleteRule = (id: number) => http.delete(`/api/rules/${id}`)

// ---- Blacklist ----
export const listBlacklist = () => http.get('/api/blacklist')
export const addBlacklist = (data: any) => http.post('/api/blacklist', data)
export const deleteBlacklist = (id: number) => http.delete(`/api/blacklist/${id}`)

// ---- Dashboard ----
export const dashboard = () => http.get('/api/dashboard')
