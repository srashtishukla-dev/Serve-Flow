import { Navigate, useLocation } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'

export default function RequireAuth({ children }) {
  const { user, status } = useAuth()
  const location = useLocation()

  if (status === 'loading') {
    return <main className="dashboard-page__loading">Checking your session...</main>
  }
  if (!user) return <Navigate to="/login" replace state={{ from: location }} />
  return children
}