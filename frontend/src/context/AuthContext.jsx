import { createContext, useContext, useEffect, useState } from 'react'
import apiClient, { accessTokenKey } from '../services/api/client'

const AuthContext = createContext(null)

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null)
  const [status, setStatus] = useState('loading')
  const [sessionMessage, setSessionMessage] = useState('')

  useEffect(() => {
    const token = localStorage.getItem(accessTokenKey)
    if (!token) {
      setStatus('anonymous')
      return
    }

    apiClient.get('/me')
      .then(({ data }) => {
        setUser(data)
        setStatus('authenticated')
      })
      .catch((error) => {
        localStorage.removeItem(accessTokenKey)
        setUser(null)
        setStatus('anonymous')
        setSessionMessage(error.response?.status === 401
          ? 'Your session expired. Please log in again.'
          : 'ServeFlow could not verify your session. Check your connection and log in again.')
      })
  }, [])

  async function login(email, password) {
    const { data } = await apiClient.post('/auth/login', { email, password })
    if (!data.token) throw new Error('The login response did not include an access token.')

    localStorage.setItem(accessTokenKey, data.token)
    try {
      const response = await apiClient.get('/me')
      setUser(response.data)
      setStatus('authenticated')
      setSessionMessage('')
      return response.data
    } catch {
      localStorage.removeItem(accessTokenKey)
      setUser(null)
      setStatus('anonymous')
      throw new Error('Login succeeded, but the session could not be verified. Please try again.')
    }
  }

  function logout() {
    localStorage.removeItem(accessTokenKey)
    setUser(null)
    setStatus('anonymous')
    setSessionMessage('')
  }

  function clearSessionMessage() {
    setSessionMessage('')
  }

  return (
    <AuthContext.Provider value={{ user, status, sessionMessage, login, logout, clearSessionMessage }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) throw new Error('useAuth must be used inside AuthProvider')
  return context
}