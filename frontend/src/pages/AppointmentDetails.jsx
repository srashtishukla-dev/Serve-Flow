import { useEffect, useState } from 'react'
import { ArrowLeft } from 'lucide-react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import Logo from '../components/ui/Logo'
import { useAuth } from '../context/AuthContext'
import apiClient from '../services/api/client'
import { availableActions, statusClass, statusLabel } from '../utils/appointmentStatus'

export default function AppointmentDetails() {
  const { id } = useParams()
  const { logout, user } = useAuth()
  const [actionError, setActionError] = useState('')
  const navigate = useNavigate()
  const [state, setState] = useState({ loading: true, error: '', appointment: null })

  useEffect(() => {
    let isActive = true
    async function load() {
      try {
        const { data } = await apiClient.get(`/appointments/${id}`)
        const appointment = data.appointment
        if (!isActive) return
        setState({ loading: false, error: '', appointment })
      } catch (error) {
        if (error.response?.status === 401) {
          logout()
          navigate('/login', { replace: true })
          return
        }
        if (isActive) {
          const message = error.response?.status === 403 ? 'You do not have permission to view this appointment.' : error.response?.status === 404 ? 'Appointment not found.' : error.response?.data?.error?.message || 'Unable to load this appointment.'
          setState((current) => ({ ...current, loading: false, error: message }))
        }
      }
    }
    load()
    return () => { isActive = false }
  }, [id])

  const { loading, error, appointment } = state

  async function changeStatus(status) {
    setActionError('')
    try {
      const { data } = await apiClient.post(`/appointments//status`, { status })
      setState((current) => ({ ...current, appointment: data.appointment }))
    } catch (err) {
      if (err.response?.status === 401) {
        logout()
        navigate('/login', { replace: true })
        return
      }
      setActionError(err.response?.data?.error?.message || 'Unable to update this appointment.')
    }
  }

  return (
    <main className="services-page bookings-page">
      <header className="services-header">
        <Logo />
        <Link className="text-link" to="/appointments"><ArrowLeft size={15} /> Appointments</Link>
      </header>
      <section className="services-content" aria-labelledby="appointment-details-title">
        <h1 id="appointment-details-title">Appointment details</h1>
        {loading && <p className="services-state" role="status">Loading appointment...</p>}
        {error && <p className="auth-notice auth-notice--error" role="alert">{error}</p>}
        {appointment && (
          <dl className="appointment-details">
            <dt>Status</dt>
            <dd>
              <span className={statusClass(appointment.status)}>{statusLabel(appointment.status)}</span>
              {appointment.is_overdue && <span className="booking-status booking-status--overdue">OVERDUE</span>}
            </dd>
            <dt>Date</dt><dd>{appointment.appointment_date}</dd>
            <dt>Time</dt><dd>{appointment.start_time} to {appointment.end_time}</dd>
            <dt>Customer</dt><dd>{appointment.customer_name || 'Unavailable'}</dd>
            <dt>Service</dt><dd>{appointment.service_name || 'Unavailable'}</dd>
            <dt>Technician</dt><dd>{appointment.technician_name || 'Unassigned'}</dd>
            <dt>Notes</dt><dd>{appointment.notes || 'No notes'}</dd>
          </dl>
        )}
        {actionError && <p className="auth-notice auth-notice--error" role="alert">{actionError}</p>}
        {appointment && (
          <div className="service-actions">
            {availableActions(user?.role, appointment).map((action) => (
              <button className={`button button--outline button--small`} key={action.status} onClick={() => changeStatus(action.status)} type="button">{action.label}</button>
            ))}
          </div>
        )}
      </section>
    </main>
  )
}
