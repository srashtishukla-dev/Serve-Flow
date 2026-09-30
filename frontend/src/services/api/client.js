import axios from 'axios'

export const accessTokenKey = 'serveflow_access_token'

const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
  headers: {
    'Content-Type': 'application/json',
  },
  timeout: 10000,
})

apiClient.interceptors.request.use((request) => {
  const token = localStorage.getItem(accessTokenKey)
  if (token) request.headers.Authorization = `Bearer ${token}`
  return request
})

export default apiClient