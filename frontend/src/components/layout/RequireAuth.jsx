import { Link, Navigate, useLocation } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'

export default function RequireAuth({ children, roles }) {
  const { user, status } = useAuth()
  const location = useLocation()

  if (status === 'loading') {
    return <main className="dashboard-page__loading">Checking your session...</main>
  }
  if (!user) return <Navigate to="/login" replace state={{ from: location }} />
  if (roles && !roles.includes(user.role)) {
    return (
      <main className="dashboard-page__loading" role="alert">
        <div>
          <h1>Access denied</h1>
          <p>Your account does not have permission to view this page.</p>
          <Link className="text-link" to="/dashboard">Back to dashboard</Link>
        </div>
      </main>
    )
  }
  return children
}
