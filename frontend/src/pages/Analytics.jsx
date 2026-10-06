import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import Logo from '../components/ui/Logo'
import { useAuth } from '../context/AuthContext'
import apiClient from '../services/api/client'

const ranges = [
  { value: 'today', label: 'Today' },
  { value: 'last_7_days', label: 'Last 7 days' },
  { value: 'last_30_days', label: 'Last 30 days' },
  { value: 'current_month', label: 'Current month' },
]

export default function Analytics() {
  const { logout } = useAuth()
  const navigate = useNavigate()
  const [summary, setSummary] = useState(null)
  const [activity, setActivity] = useState(null)
  const [range, setRange] = useState('last_30_days')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [refresh, setRefresh] = useState(0)

  useEffect(() => {
    let active = true
    setLoading(true)
    setError('')
    Promise.all([
      apiClient.get('/analytics/summary'),
      apiClient.get('/analytics/activity', { params: { range } }),
    ])
      .then(([summaryResponse, activityResponse]) => {
        if (!active) return
        setSummary(summaryResponse.data)
        setActivity(activityResponse.data)
      })
      .catch((requestError) => {
        if (requestError.response?.status === 401) {
          logout()
          navigate('/login', { replace: true })
          return
        }
        if (active) setError(requestError.response?.data?.error?.message || 'Unable to load analytics. Please try again.')
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => { active = false }
  }, [range, refresh])

  return (
    <main className="dashboard-page">
      <header className="dashboard-header">
        <Logo />
        <div className="dashboard-header__actions">
          <Link className="button button--outline" to="/dashboard">Dashboard</Link>
        </div>
      </header>
      <section aria-labelledby="analytics-title" className="dashboard-content analytics-content">
        <span className="section-kicker">WORKSPACE PERFORMANCE</span>
        <h1 id="analytics-title">Analytics</h1>
        <p className="dashboard-content__intro">Bookings, customer activity, technician workload, and invoicing for your organization.</p>
        {loading ? (
          <p className="dashboard-state" role="status">Loading analytics...</p>
        ) : error ? (
          <div className="dashboard-analytics-error" role="alert">
            <p>{error}</p>
            <button className="button button--outline button--small" onClick={() => setRefresh((value) => value + 1)} type="button">Try again</button>
          </div>
        ) : summary && activity && (
          <>
            <div aria-label="Analytics summary" className="dashboard-metrics">
              <Metric label="Appointments" value={formatCount(summary.metrics.appointments)} detail={`${formatCount(summary.metrics.completed_appointments)} completed`} />
              <Metric label="Cancelled" value={formatCount(summary.metrics.cancelled_appointments)} detail={`${formatCount(summary.metrics.no_show_appointments)} no-shows`} />
              <Metric label="Technicians" value={formatCount(summary.metrics.total_technicians)} detail={`${formatCount(summary.metrics.active_technicians)} active`} />
              <Metric label="Services" value={formatCount(summary.metrics.services)} />
              <Metric label="Total invoiced" value={formatCurrency(summary.metrics.total_invoiced_amount)} detail={`${formatCount(summary.metrics.total_invoices)} invoices`} />
              <Metric label="Revenue received" value={formatCurrency(summary.metrics.total_revenue)} detail={`${formatCount(summary.metrics.total_payments)} recorded payments`} />
              <Metric label="Outstanding balance" value={formatCurrency(summary.metrics.outstanding_balance)} detail={`${formatCount(summary.metrics.pending_invoices)} pending invoices`} />
              <Metric label="Paid invoices" value={formatCount(summary.metrics.paid_invoices)} />
            </div>

            <section aria-labelledby="activity-title" className="dashboard-chart analytics-section">
              <header className="dashboard-chart__header">
                <div>
                  <h2 id="activity-title">Daily activity</h2>
                  <span>{activity.start_date} – {activity.end_date}</span>
                </div>
                <label className="analytics-range">
                  <span className="sr-only">Date range</span>
                  <select aria-label="Date range" onChange={(event) => setRange(event.target.value)} value={range}>
                    {ranges.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                  </select>
                </label>
              </header>
              <div className="analytics-table-wrap">
                <table className="analytics-table">
                  <thead>
                    <tr><th scope="col">Date</th><th scope="col">Appointments</th><th scope="col">Completed</th><th scope="col">Cancelled</th><th scope="col">No-shows</th><th scope="col">Revenue</th></tr>
                  </thead>
                  <tbody>
                    {activity.days.map((day) => (
                      <tr key={day.date}>
                        <th scope="row">{formatDate(day.date)}</th>
                        <td>{formatCount(day.appointments)}</td>
                        <td>{formatCount(day.completed_appointments)}</td>
                        <td>{formatCount(day.cancelled_appointments)}</td>
                        <td>{formatCount(day.no_show_appointments)}</td>
                        <td>{formatCurrency(day.revenue)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>

            <div className="analytics-insights">
              <InsightList title="Popular services" empty="No service bookings yet." items={summary.popular_services} render={(item) => <><strong>{item.name}</strong><span>{formatCount(item.bookings)} bookings</span></>} />
              <InsightList title="Customer activity" empty="No customer bookings yet." items={summary.top_customers} render={(item) => <><strong>{item.name}</strong><span>{formatCount(item.bookings)} bookings · last {formatDate(item.last_booking_date)}</span></>} />
              <InsightList title="Technician workload" empty="No technicians yet." items={summary.technician_workload} render={(item) => <><strong>{item.name}</strong><span>{formatCount(item.active_bookings)} active · {formatCount(item.completed_bookings)} completed</span></>} />
            </div>
          </>
        )}
      </section>
    </main>
  )
}

function Metric({ label, value, detail }) {
  return (
    <article className="dashboard-metric">
      <span className="dashboard-metric__label">{label}</span>
      <strong className="dashboard-metric__value">{value}</strong>
      {detail && <span className="dashboard-metric__detail">{detail}</span>}
    </article>
  )
}

function InsightList({ title, empty, items, render }) {
  return (
    <section aria-label={title} className="dashboard-chart analytics-insight">
      <header className="dashboard-chart__header"><h2>{title}</h2></header>
      {items.length === 0 ? <p className="analytics-empty">{empty}</p> : (
        <ol className="analytics-list">
          {items.map((item) => <li key={item.id}>{render(item)}</li>)}
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

function formatDate(value) {
  if (!value) return '—'
  const [year, month, day] = value.split('-').map(Number)
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', year: 'numeric' }).format(new Date(year, month - 1, day))
}
