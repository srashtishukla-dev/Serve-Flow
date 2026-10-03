import { Bell, Check, RefreshCw } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import Logo from '../components/ui/Logo'
import { useAuth } from '../context/AuthContext'
import apiClient from '../services/api/client'

export default function Notifications() {
  const { logout } = useAuth()
  const navigate = useNavigate()
  const [notifications, setNotifications] = useState([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [savingID, setSavingID] = useState('')
  const [pageError, setPageError] = useState('')

  useEffect(() => {
    let isActive = true
    apiClient.get('/notifications')
      .then(({ data }) => {
        if (isActive) setNotifications(data.notifications || [])
      })
      .catch((error) => {
        if (!isActive) return
        if (error.response?.status === 401) {
          logout()
          navigate('/login', { replace: true })
          return
        }
        setPageError(error.response?.data?.error?.message || 'Unable to load notifications. Check your connection and try again.')
      })
      .finally(() => {
        if (isActive) setLoading(false)
      })

    return () => { isActive = false }
  }, [])

  async function refreshNotifications() {
    setRefreshing(true)
    setPageError('')
    try {
      const { data } = await apiClient.get('/notifications')
      setNotifications(data.notifications || [])
    } catch (error) {
      if (error.response?.status === 401) {
        logout()
        navigate('/login', { replace: true })
        return
      }
      setPageError(error.response?.data?.error?.message || 'Unable to load notifications. Check your connection and try again.')
    } finally {
      setRefreshing(false)
    }
  }

  async function markAsRead(notification) {
    setSavingID(notification.id)
    setPageError('')
    try {
      const { data } = await apiClient.put(`/notifications/${notification.id}/read`)
      setNotifications((current) => current.map((item) => item.id === data.notification.id ? data.notification : item))
    } catch (error) {
      if (error.response?.status === 401) {
        logout()
        navigate('/login', { replace: true })
        return
      }
      setPageError(error.response?.data?.error?.message || 'Unable to update this notification. Please try again.')
    } finally {
      setSavingID('')
    }
  }

  return (
    <main className="services-page notifications-page">
      <header className="services-header">
        <Logo />
        <Link className="text-link" to="/dashboard">Dashboard</Link>
      </header>
      <section className="services-content" aria-labelledby="notifications-title">
        <div className="services-heading">
          <div>
            <span className="section-kicker">YOUR WORKSPACE</span>
            <h1 id="notifications-title">Notifications</h1>
            <p>Updates about appointments, invoices, and payments.</p>
          </div>
          <button className="button button--outline" disabled={refreshing || loading} onClick={refreshNotifications} type="button">
            <RefreshCw aria-hidden="true" size={16} /> {refreshing ? 'Refreshing...' : 'Refresh'}
          </button>
        </div>

        {pageError && <p className="auth-notice auth-notice--error" role="alert">{pageError}</p>}
        {loading ? (
          <p className="services-state" role="status">Loading notifications...</p>
        ) : notifications.length === 0 ? (
          <div className="services-empty notifications-empty">
            <Bell aria-hidden="true" size={22} />
            <h2>You’re all caught up</h2>
            <p>New appointment, invoice, and payment updates will appear here.</p>
          </div>
        ) : (
          <ul aria-label="Notifications" className="notification-list">
            {notifications.map((notification) => (
              <li className={`notification-item${notification.is_read ? '' : ' notification-item--unread'}`} key={notification.id}>
                <div className="notification-item__body">
                  <div className="notification-item__heading">
                    <h2>{notification.title}</h2>
                    <span className={`notification-state${notification.is_read ? ' notification-state--read' : ''}`}>
                      {notification.is_read ? 'Read' : 'Unread'}
                    </span>
                  </div>
                  <p>{notification.message}</p>
                  <time dateTime={notification.created_at}>{new Date(notification.created_at).toLocaleString()}</time>
                </div>
                {!notification.is_read && (
                  <button
                    aria-label={`Mark ${notification.title} as read`}
                    className="button button--outline button--small notification-item__action"
                    disabled={savingID === notification.id}
                    onClick={() => markAsRead(notification)}
                    type="button"
                  >
                    <Check aria-hidden="true" size={15} /> {savingID === notification.id ? 'Saving...' : 'Mark read'}
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>
    </main>
  )
}