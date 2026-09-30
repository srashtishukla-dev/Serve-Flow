import { CalendarDays, LogOut, UsersRound, Wrench } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import Logo from '../components/ui/Logo'
import { useAuth } from '../context/AuthContext'
import apiClient from '../services/api/client'

export default function Dashboard() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [organization, setOrganization] = useState(null)
  const [organizationError, setOrganizationError] = useState('')

  useEffect(() => {
    let isActive = true
    apiClient.get('/organization')
      .then(({ data }) => {
        if (isActive) setOrganization(data)
      })
      .catch((error) => {
        if (error.response?.status === 401) {
          logout()
          navigate('/login', { replace: true })
          return
        }
        if (isActive) setOrganizationError('Unable to load workspace information. Please try again.')
      })

    return () => { isActive = false }
  }, [])

  function handleLogout() {
    logout()
    navigate('/login', { replace: true })
  }

  return (
    <main className="dashboard-page">
      <header className="dashboard-header">
        <Logo />
        <div className="dashboard-header__actions">
          <Link className="button button--outline" to="/services"><Wrench size={16} /> Services</Link>
          <Link className="button button--outline" to="/customers"><UsersRound size={16} /> Customers</Link>
          <Link className="button button--outline" to="/bookings"><CalendarDays size={16} /> Bookings</Link>
          <button className="button button--outline dashboard-logout" onClick={handleLogout} type="button">
            <LogOut size={16} /> Log out
          </button>
        </div>
      </header>
      <section className="dashboard-content" aria-labelledby="dashboard-title">
        <span className="section-kicker">YOUR SERVEFLOW ACCOUNT</span>
        <h1 id="dashboard-title">Welcome, {user.name}</h1>
        <p className="dashboard-content__intro">You are signed in to your ServeFlow account.</p>
        <div className="dashboard-profile">
          <div>
            <span>Name</span>
            <strong>{user.name}</strong>
          </div>
          <div>
            <span>Email</span>
            <strong>{user.email}</strong>
          </div>
          {organization && (
            <>
              <div>
                <span>Organization</span>
                <strong>{organization.name}</strong>
              </div>
              <div>
                <span>Workspace</span>
                <strong>{organization.slug}</strong>
              </div>
            </>
          )}
        </div>
        {organizationError && <p className="auth-notice auth-notice--error" role="alert">{organizationError}</p>}
      </section>
    </main>
  )
}