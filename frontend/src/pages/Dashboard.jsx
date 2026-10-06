import { Banknote, Bell, CalendarDays, CalendarRange, CheckCircle2, Clock3, FileText, LogOut, Receipt, UserRound, UsersRound, Wrench } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import Logo from '../components/ui/Logo'
import { useAuth } from '../context/AuthContext'
import apiClient from '../services/api/client'

export default function Dashboard() {
  const { user, logout } = useAuth()
  const isAdmin = user.role === 'ADMIN'
  const navigate = useNavigate()
  const [organization, setOrganization] = useState(null)
  const [organizationError, setOrganizationError] = useState('')
  const [analytics, setAnalytics] = useState(null)
  const [analyticsLoading, setAnalyticsLoading] = useState(true)
  const [analyticsError, setAnalyticsError] = useState('')
  const [analyticsRequest, setAnalyticsRequest] = useState(0)

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

  useEffect(() => {
    let isActive = true
    if (!isAdmin) {
      setAnalyticsLoading(false)
      return undefined
    }
    setAnalyticsLoading(true)
    setAnalyticsError('')
    apiClient.get('/analytics/summary')
      .then(({ data }) => {
        if (isActive) setAnalytics(data)
      })
      .catch((error) => {
        if (error.response?.status === 401) {
          logout()
          navigate('/login', { replace: true })
          return
        }
        if (isActive) setAnalyticsError(error.response?.data?.error?.message || 'Unable to load workspace analytics. Please try again.')
      })
      .finally(() => {
        if (isActive) setAnalyticsLoading(false)
      })

    return () => { isActive = false }
  }, [analyticsRequest])

  function handleLogout() {
    logout()
    navigate('/login', { replace: true })
  }

  return (
    <main className="dashboard-page">
      <header className="dashboard-header">
        <Logo />
        <div className="dashboard-header__actions">
          {isAdmin && <Link className="button button--outline" to="/services"><Wrench size={16} /> Services</Link>}
          {isAdmin && <Link className="button button--outline" to="/customers"><UsersRound size={16} /> Customers</Link>}
          {isAdmin && <Link className="button button--outline" to="/bookings"><CalendarDays size={16} /> Bookings</Link>}
          <Link className="button button--outline" to="/appointments"><CalendarRange size={16} /> Appointments</Link>
          {isAdmin && <Link className="button button--outline" to="/technicians"><UserRound size={16} /> Technicians</Link>}
          {isAdmin && <Link className="button button--outline" to="/invoices"><Receipt size={16} /> Invoices</Link>}
          {user.role === 'CUSTOMER' && <Link className="button button--outline" to="/invoices"><Receipt size={16} /> My invoices</Link>}
          <Link className="button button--outline" to="/notifications"><Bell size={16} /> Notifications</Link>
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
        {isAdmin && <section aria-label="Workspace analytics" className="dashboard-analytics">
          {analyticsLoading ? (
            <p className="dashboard-state" role="status">Loading workspace analytics...</p>
          ) : analyticsError ? (
            <div className="dashboard-analytics-error" role="alert">
              <p>{analyticsError}</p>
              <button className="button button--outline button--small" onClick={() => setAnalyticsRequest((current) => current + 1)} type="button">Try again</button>
            </div>
          ) : analytics && (
            <>
              <div aria-label="Analytics summary" className="dashboard-metrics">
                <MetricCard icon={UsersRound} label="Customers" value={formatCount(analytics.metrics.customers)} />
                <MetricCard icon={Wrench} label="Services" value={formatCount(analytics.metrics.services)} />
                <MetricCard icon={CalendarRange} label="Appointments" value={formatCount(analytics.metrics.appointments)} detail={`${formatCount(analytics.metrics.completed_appointments)} completed`} />
                <MetricCard icon={Clock3} label="Upcoming appointments" value={formatCount(analytics.metrics.upcoming_appointments)} />
                <MetricCard icon={UserRound} label="Active technicians" value={formatCount(analytics.metrics.active_technicians)} />
                <MetricCard icon={Banknote} label="Revenue received" value={formatCurrency(analytics.metrics.total_revenue)} detail="Recorded payments" />
                <MetricCard icon={FileText} label="Pending invoices" value={formatCount(analytics.metrics.pending_invoices)} detail={`${formatCurrency(analytics.metrics.outstanding_balance)} outstanding`} />
                <MetricCard icon={CheckCircle2} label="Paid invoices" value={formatCount(analytics.metrics.paid_invoices)} detail={`${formatCount(analytics.metrics.total_invoices)} total invoices`} />
              </div>
              <div className="dashboard-charts">
                <TrendChart
                  data={analytics.trends.months}
                  emptyMessage="No payments in this period."
                  formatValue={formatChartCurrency}
                  title="Revenue trend"
                  valueKey="revenue"
                  range={analytics.trends}
                />
                <TrendChart
                  data={analytics.trends.months}
                  emptyMessage="No appointments in this period."
                  formatValue={formatCount}
                  title="Appointment trend"
                  valueKey="appointments"
                  range={analytics.trends}
                />
              </div>
            </>
          )}
        </section>}
        {organizationError && <p className="auth-notice auth-notice--error" role="alert">{organizationError}</p>}
      </section>
    </main>
  )
}

function MetricCard({ icon: Icon, label, value, detail }) {
  return (
    <article className="dashboard-metric">
      <div className="dashboard-metric__label"><Icon aria-hidden="true" size={17} /><span>{label}</span></div>
      <strong className="dashboard-metric__value">{value}</strong>
      {detail && <span className="dashboard-metric__detail">{detail}</span>}
    </article>
  )
}

function TrendChart({ data, emptyMessage, formatValue, title, valueKey, range }) {
  const values = data.map((item) => Number(item[valueKey]) || 0)
  const maximum = Math.max(...values, 0)
  const rangeLabel = `${formatMonth(range.start_month)} - ${formatMonth(range.end_month)}`

  return (
    <section aria-label={title} className="dashboard-chart">
      <header className="dashboard-chart__header">
        <h2>{title}</h2>
        <span>{rangeLabel}</span>
      </header>
      {maximum === 0 ? (
        <p className="dashboard-chart__empty" role="status">{emptyMessage}</p>
      ) : (
        <ol aria-label={title} className="dashboard-chart__bars">
          {data.map((item) => {
            const value = Number(item[valueKey]) || 0
            const height = value === 0 ? 0 : Math.max(5, value / maximum * 100)
            return (
              <li aria-label={`${formatMonth(item.month)}: ${formatValue(value)}`} className="dashboard-chart__month" key={item.month}>
                <span className="dashboard-chart__value">{value === 0 ? '' : formatValue(value)}</span>
                <div aria-hidden="true" className="dashboard-chart__track">
                  <span className="dashboard-chart__bar" style={{ height: `${height}%` }} />
                </div>
                <span className="dashboard-chart__label">{formatMonth(item.month)}</span>
              </li>
            )
          })}
        </ol>
      )}
    </section>
  )
}

function formatCount(value) {
  return new Intl.NumberFormat().format(Number(value) || 0)
}

function formatCurrency(value) {
  return new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', maximumFractionDigits: 2 }).format(Number(value) || 0)
}

function formatChartCurrency(value) {
  return new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', notation: 'compact', maximumFractionDigits: 1 }).format(Number(value) || 0)
}

function formatMonth(value) {
  const [year, month] = value.split('-').map(Number)
  return new Intl.DateTimeFormat(undefined, { month: 'short', year: 'numeric' }).format(new Date(year, month - 1, 1))
}