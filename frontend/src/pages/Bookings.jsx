import { useEffect, useState } from 'react'
import { ArrowLeft, CalendarDays, Pencil, Plus, X } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import BookingForm from '../components/layout/BookingForm'
import Button from '../components/ui/Button'
import Logo from '../components/ui/Logo'
import { useAuth } from '../context/AuthContext'
import apiClient from '../services/api/client'

export default function Bookings({ appointments = false }) {
  const { logout } = useAuth()
  const navigate = useNavigate()
  const [bookings, setBookings] = useState([])
  const [customers, setCustomers] = useState([])
  const [services, setServices] = useState([])
  const [technicians, setTechnicians] = useState([])
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
      apiClient.get(appointments ? '/appointments' : '/bookings'),
      apiClient.get('/customers'),
      apiClient.get('/services'),
      ...(appointments ? [apiClient.get('/technicians')] : []),
    ])
      .then(([bookingResponse, customerResponse, serviceResponse, technicianResponse]) => {
        if (!isActive) return
        const items = appointments ? bookingResponse.data.appointments || [] : bookingResponse.data.bookings || []
        setBookings(items.map((item) => appointments ? { ...item, booking_date: item.appointment_date } : item))
        setCustomers(customerResponse.data.customers || [])
        setServices(serviceResponse.data.services || [])
        if (appointments) setTechnicians(technicianResponse.data.technicians || [])
      })
      .catch((error) => {
        if (error.response?.status === 401) {
          logout()
          navigate('/login', { replace: true })
          return
        }
        if (isActive) setPageError(error.response?.data?.error?.message || `Unable to load ${appointments ? 'appointments' : 'bookings'}. Check your connection and try again.`)
      })
      .finally(() => {
        if (isActive) setLoading(false)
      })

    return () => { isActive = false }
  }, [appointments])

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
    const collectionPath = appointments ? '/appointments' : '/bookings'
    try {
      if (editingBooking) {
        const { data } = await apiClient.put(`${collectionPath}/${editingBooking.id}`, values)
        const saved = appointments ? { ...data.appointment, booking_date: data.appointment.appointment_date } : data.booking
        setBookings((current) => current.map((booking) => booking.id === saved.id ? saved : booking).sort(compareBookings))
        setNotice(appointments ? 'Appointment updated.' : 'Booking updated.')
      } else {
        const { data } = await apiClient.post(collectionPath, values)
        const saved = appointments ? { ...data.appointment, booking_date: data.appointment.appointment_date } : data.booking
        setBookings((current) => [...current, saved].sort(compareBookings))
        setNotice(appointments ? 'Appointment created.' : 'Booking created.')
      }
      setDialogOpen(false)
    } catch (error) {
      if (error.response?.status === 401) {
        logout()
        navigate('/login', { replace: true })
        return
      }
      setFormError(error.response?.data?.error?.message || `Unable to save this ${appointments ? 'appointment' : 'booking'}. Please try again.`)
    } finally {
      setSaving(false)
    }
  }

  async function cancelBooking(booking) {
    if (!window.confirm(`Cancel this ${appointments ? 'appointment' : 'booking'}?`)) return
    setPageError('')
    setNotice('')
    try {
      await apiClient.delete(`${appointments ? '/appointments' : '/bookings'}/${booking.id}`)
      setBookings((current) => current.map((item) => item.id === booking.id ? { ...item, status: 'CANCELLED' } : item))
      setNotice(appointments ? 'Appointment cancelled.' : 'Booking cancelled.')
    } catch (error) {
      if (error.response?.status === 401) {
        logout()
        navigate('/login', { replace: true })
        return
      }
      setPageError(error.response?.data?.error?.message || `Unable to cancel this ${appointments ? 'appointment' : 'booking'}. Please try again.`)
    }
  }

  const customerNames = new Map(customers.map((customer) => [customer.id, customer.name]))
  const serviceNames = new Map(services.map((service) => [service.id, service.name]))
  const technicianNames = new Map(technicians.map((technician) => [technician.id, technician.name]))

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
            <h1 id="bookings-title">{appointments ? 'Appointments' : 'Bookings'}</h1>
            <p>{appointments ? "Schedule customers with your organization's services and technicians." : "Schedule a customer with one of your organization's services."}</p>
          </div>
          <Button disabled={customers.length === 0 || services.length === 0} onClick={openCreateDialog} type="button"><Plus size={17} /> Add {appointments ? 'appointment' : 'booking'}</Button>
        </div>

        {notice && <p className="auth-notice auth-notice--success" role="status">{notice}</p>}
        {pageError && <p className="auth-notice auth-notice--error" role="alert">{pageError}</p>}

        {loading ? (
          <p className="services-state" role="status">Loading {appointments ? 'appointments' : 'bookings'}...</p>
        ) : customers.length === 0 || services.length === 0 ? (
          <div className="services-empty">
            <h2>Set up your workspace first</h2>
            <p>Add at least one customer and one service before scheduling.</p>
            <div className="booking-prerequisite-links"><Link className="text-link" to="/customers">Open Customers</Link><Link className="text-link" to="/services">Open Services</Link></div>
          </div>
        ) : bookings.length === 0 ? (
          <div className="services-empty">
            <span className="booking-empty-icon"><CalendarDays size={22} /></span>
            <h2>No {appointments ? 'appointments' : 'bookings'} yet</h2>
            <p>Create the first {appointments ? 'appointment' : 'booking'} for this workspace.</p>
            <Button onClick={openCreateDialog} type="button"><Plus size={16} /> Add {appointments ? 'appointment' : 'booking'}</Button>
          </div>
        ) : (
          <div className="services-table-wrap">
            <table className={`services-table bookings-table${appointments ? ' bookings-table--appointments' : ''}`}>
              <thead>
                <tr><th scope="col">Date</th><th scope="col">Time</th><th scope="col">Customer</th><th scope="col">Service</th>{appointments && <th scope="col">Technician</th>}<th scope="col">Status</th><th scope="col">Notes</th><th scope="col"><span className="sr-only">Actions</span></th></tr>
              </thead>
              <tbody>
                {bookings.map((booking) => (
                  <tr key={booking.id}>
                    <td>{booking.booking_date}</td>
                    <td>{booking.start_time} to {booking.end_time}</td>
                    <td className="services-table__name">{customerNames.get(booking.customer_id) || 'Unavailable'}</td>
                    <td>{serviceNames.get(booking.service_id) || 'Unavailable'}</td>
                    {appointments && <td>{technicianNames.get(booking.technician_id) || 'Unassigned'}</td>}
                    <td><span className={`booking-status booking-status--${booking.status.toLowerCase()}`}>{booking.status}</span></td>
                    <td className="services-table__description">{booking.notes || 'No notes'}</td>
                    <td>
                      {booking.status === 'BOOKED' && (
                        <div className="service-actions">
                          <button aria-label={`Edit ${appointments ? 'appointment' : 'booking'}`} className="service-icon-button" onClick={() => openEditDialog(booking)} title={`Edit ${appointments ? 'appointment' : 'booking'}`} type="button"><Pencil size={16} /></button>
                          <button aria-label={`Cancel ${appointments ? 'appointment' : 'booking'}`} className="service-icon-button service-icon-button--delete" onClick={() => cancelBooking(booking)} title={`Cancel ${appointments ? 'appointment' : 'booking'}`} type="button"><X size={17} /></button>
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
                <span className="section-kicker">{appointments ? 'APPOINTMENT DETAILS' : 'BOOKING DETAILS'}</span>
                <h2 id="booking-dialog-title">{editingBooking ? `Edit ${appointments ? 'appointment' : 'booking'}` : `Add a ${appointments ? 'appointment' : 'booking'}`}</h2>
              </div>
              <button aria-label="Close form" className="service-icon-button" onClick={() => setDialogOpen(false)} type="button"><X size={18} /></button>
            </header>
            <BookingForm
              key={editingBooking?.id || 'new-booking'}
              customers={customers}
              technicians={technicians}
              appointmentMode={appointments}
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