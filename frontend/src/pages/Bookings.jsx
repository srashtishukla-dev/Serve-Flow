import { useEffect, useState } from 'react'
import { ArrowLeft, CalendarDays, Pencil, Plus, X } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import BookingForm from '../components/layout/BookingForm'
import Button from '../components/ui/Button'
import Logo from '../components/ui/Logo'
import { useAuth } from '../context/AuthContext'
import apiClient from '../services/api/client'

export default function Bookings() {
  const { logout } = useAuth()
  const navigate = useNavigate()
  const [bookings, setBookings] = useState([])
  const [customers, setCustomers] = useState([])
  const [services, setServices] = useState([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [pageError, setPageError] = useState('')
  const [formError, setFormError] = useState('')
  const [notice, setNotice] = useState('')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingBooking, setEditingBooking] = useState(null)

  useEffect(() => {
    let isActive = true
    Promise.all([
      apiClient.get('/bookings'),
      apiClient.get('/customers'),
      apiClient.get('/services'),
    ])
      .then(([bookingResponse, customerResponse, serviceResponse]) => {
        if (!isActive) return
        setBookings(bookingResponse.data.bookings || [])
        setCustomers(customerResponse.data.customers || [])
        setServices(serviceResponse.data.services || [])
      })
      .catch((error) => {
        if (error.response?.status === 401) {
          logout()
          navigate('/login', { replace: true })
          return
        }
        if (isActive) setPageError(error.response?.data?.error?.message || 'Unable to load bookings. Check your connection and try again.')
      })
      .finally(() => {
        if (isActive) setLoading(false)
      })

    return () => { isActive = false }
  }, [])

  function openCreateDialog() {
    setEditingBooking(null)
    setFormError('')
    setNotice('')
    setDialogOpen(true)
  }

  function openEditDialog(booking) {
    setEditingBooking(booking)
    setFormError('')
    setNotice('')
    setDialogOpen(true)
  }

  async function saveBooking(values) {
    setSaving(true)
    setFormError('')
    try {
      if (editingBooking) {
        const { data } = await apiClient.put(`/bookings/${editingBooking.id}`, values)
        setBookings((current) => current.map((booking) => booking.id === data.booking.id ? data.booking : booking))
        setNotice('Booking updated.')
      } else {
        const { data } = await apiClient.post('/bookings', values)
        setBookings((current) => [...current, data.booking].sort(compareBookings))
        setNotice('Booking created.')
      }
      setDialogOpen(false)
    } catch (error) {
      if (error.response?.status === 401) {
        logout()
        navigate('/login', { replace: true })
        return
      }
      setFormError(error.response?.data?.error?.message || 'Unable to save this booking. Please try again.')
    } finally {
      setSaving(false)
    }
  }

  async function cancelBooking(booking) {
    if (!window.confirm('Cancel this booking?')) return
    setPageError('')
    setNotice('')
    try {
      await apiClient.delete(`/bookings/${booking.id}`)
      setBookings((current) => current.map((item) => item.id === booking.id ? { ...item, status: 'CANCELLED' } : item))
      setNotice('Booking cancelled.')
    } catch (error) {
      if (error.response?.status === 401) {
        logout()
        navigate('/login', { replace: true })
        return
      }
      setPageError(error.response?.data?.error?.message || 'Unable to cancel this booking. Please try again.')
    }
  }

  const customerNames = new Map(customers.map((customer) => [customer.id, customer.name]))
  const serviceNames = new Map(services.map((service) => [service.id, service.name]))

  return (
    <main className="services-page bookings-page">
      <header className="services-header">
        <Logo />
        <Link className="text-link" to="/dashboard"><ArrowLeft size={15} /> Dashboard</Link>
      </header>
      <section className="services-content" aria-labelledby="bookings-title">
        <div className="services-heading">
          <div>
            <span className="section-kicker">YOUR WORKSPACE SCHEDULE</span>
            <h1 id="bookings-title">Bookings</h1>
            <p>Schedule a customer with one of your organization's services.</p>
          </div>
          <Button disabled={customers.length === 0 || services.length === 0} onClick={openCreateDialog} type="button"><Plus size={17} /> Add booking</Button>
        </div>

        {notice && <p className="auth-notice auth-notice--success" role="status">{notice}</p>}
        {pageError && <p className="auth-notice auth-notice--error" role="alert">{pageError}</p>}

        {loading ? (
          <p className="services-state" role="status">Loading bookings...</p>
        ) : customers.length === 0 || services.length === 0 ? (
          <div className="services-empty">
            <h2>Set up your workspace first</h2>
            <p>Add at least one customer and one service before scheduling a booking.</p>
            <div className="booking-prerequisite-links"><Link className="text-link" to="/customers">Open Customers</Link><Link className="text-link" to="/services">Open Services</Link></div>
          </div>
        ) : bookings.length === 0 ? (
          <div className="services-empty">
            <span className="booking-empty-icon"><CalendarDays size={22} /></span>
            <h2>No bookings yet</h2>
            <p>Create the first booking for this workspace.</p>
            <Button onClick={openCreateDialog} type="button"><Plus size={16} /> Add booking</Button>
          </div>
        ) : (
          <div className="services-table-wrap">
            <table className="services-table bookings-table">
              <thead>
                <tr><th scope="col">Date</th><th scope="col">Time</th><th scope="col">Customer</th><th scope="col">Service</th><th scope="col">Status</th><th scope="col">Notes</th><th scope="col"><span className="sr-only">Actions</span></th></tr>
              </thead>
              <tbody>
                {bookings.map((booking) => (
                  <tr key={booking.id}>
                    <td>{booking.booking_date}</td>
                    <td>{booking.start_time} to {booking.end_time}</td>
                    <td className="services-table__name">{customerNames.get(booking.customer_id) || 'Unavailable'}</td>
                    <td>{serviceNames.get(booking.service_id) || 'Unavailable'}</td>
                    <td><span className={`booking-status booking-status--${booking.status.toLowerCase()}`}>{booking.status}</span></td>
                    <td className="services-table__description">{booking.notes || 'No notes'}</td>
                    <td>
                      {booking.status === 'BOOKED' && (
                        <div className="service-actions">
                          <button aria-label="Edit booking" className="service-icon-button" onClick={() => openEditDialog(booking)} title="Edit booking" type="button"><Pencil size={16} /></button>
                          <button aria-label="Cancel booking" className="service-icon-button service-icon-button--delete" onClick={() => cancelBooking(booking)} title="Cancel booking" type="button"><X size={17} /></button>
                        </div>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {dialogOpen && (
        <div className="services-modal-backdrop">
          <section aria-labelledby="booking-dialog-title" aria-modal="true" className="services-modal" role="dialog">
            <header className="services-modal__header">
              <div>
                <span className="section-kicker">BOOKING DETAILS</span>
                <h2 id="booking-dialog-title">{editingBooking ? 'Edit booking' : 'Add a booking'}</h2>
              </div>
              <button aria-label="Close form" className="service-icon-button" onClick={() => setDialogOpen(false)} type="button"><X size={18} /></button>
            </header>
            <BookingForm
              key={editingBooking?.id || 'new-booking'}
              customers={customers}
              error={formError}
              initialValues={editingBooking || {}}
              onCancel={() => setDialogOpen(false)}
              onSubmit={saveBooking}
              saving={saving}
              services={services}
            />
          </section>
        </div>
      )}
    </main>
  )
}

function compareBookings(left, right) {
  return `${left.booking_date} ${left.start_time}`.localeCompare(`${right.booking_date} ${right.start_time}`)
}