import { useEffect, useState } from 'react'
import { ArrowLeft, CalendarDays, Pencil, Plus, X } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import BookingForm from '../components/layout/BookingForm'
import Button from '../components/ui/Button'
import Logo from '../components/ui/Logo'
import { useAuth } from '../context/AuthContext'
import apiClient from '../services/api/client'
import { availableActions, canEdit, statusClass, statusLabel, statusOptions } from '../utils/appointmentStatus'

export default function Bookings({ appointments = false }) {
  const { logout, user } = useAuth()
  const role = user?.role || 'ADMIN'
  const isAdmin = role === 'ADMIN'
  const isCustomer = role === 'CUSTOMER'
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

  const [filters, setFilters] = useState({ view: '', assignment: '', status: '', search: '' })
  const [searchInput, setSearchInput] = useState('')
  const [page, setPage] = useState(1)
  const [pagination, setPagination] = useState(null)
  const [reloadKey, setReloadKey] = useState(0)

  function handleSessionError(error) {
    if (error.response?.status === 401) {
      logout()
      navigate('/login', { replace: true })
      return true
    }
    return false
  }

  useEffect(() => {
    let isActive = true
    Promise.all([
      isAdmin ? apiClient.get('/customers') : Promise.resolve(null),
      isAdmin || isCustomer ? apiClient.get('/services') : Promise.resolve(null),
      appointments && isAdmin ? apiClient.get('/technicians') : Promise.resolve(null),
    ])
      .then(([customerResponse, serviceResponse, technicianResponse]) => {
        if (!isActive) return
        setCustomers(isCustomer ? [{ id: user.customer_id, name: user.name }] : customerResponse?.data.customers || [])
        setServices(serviceResponse?.data.services || [])
        setTechnicians(technicianResponse?.data.technicians || [])
      })
      .catch((error) => {
        if (handleSessionError(error)) return
        if (isActive) setPageError(error.response?.data?.error?.message || 'Unable to load workspace data. Check your connection and try again.')
      })
    return () => { isActive = false }
  }, [appointments, isAdmin, isCustomer])

  useEffect(() => {
    let isActive = true
    setLoading(true)
    const params = { page, limit: 20, sort: 'booking_date', order: 'asc' }
    if (appointments) {
      Object.entries(filters).forEach(([key, value]) => { if (value) params[key] = value })
    } else {
      if (filters.status) params.status = filters.status
      if (filters.search) params.search = filters.search
    }
    apiClient.get(appointments ? '/appointments' : '/bookings', { params })
      .then((response) => {
        if (!isActive) return
        const items = appointments ? response.data.appointments || [] : response.data.bookings || []
        setBookings(items.map((item) => appointments ? { ...item, booking_date: item.appointment_date } : item))
        setPagination(response.data.pagination || null)
        setPageError('')
      })
      .catch((error) => {
        if (handleSessionError(error)) return
        if (isActive) setPageError(error.response?.data?.error?.message || `Unable to load ${appointments ? 'appointments' : 'bookings'}. Check your connection and try again.`)
      })
      .finally(() => {
        if (isActive) setLoading(false)
      })

    return () => { isActive = false }
  }, [appointments, filters, page, reloadKey])

  function updateFilter(name, value) {
    setPage(1)
    setFilters((current) => ({ ...current, [name]: value }))
  }

  function submitSearch(event) {
    event.preventDefault()
    updateFilter('search', searchInput.trim())
  }
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
      if (appointments) setReloadKey((key) => key + 1)
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

  async function cancelBooking(booking, nextStatus = 'CANCELLED') {
    if (nextStatus === 'CANCELLED' && !window.confirm(`Cancel this ${appointments ? 'appointment' : 'booking'}?`)) return
    setPageError('')
    setNotice('')
    try {
      if (appointments) {
        const { data } = await apiClient.post(`/appointments/${booking.id}/status`, { status: nextStatus })
        const saved = { ...data.appointment, booking_date: data.appointment.appointment_date }
        setBookings((current) => current.map((item) => item.id === saved.id ? saved : item))
        setNotice(`Appointment marked ${statusLabel(nextStatus)}.`)
        return
      }
      await apiClient.delete(`/bookings/${booking.id}`)
      setBookings((current) => current.map((item) => item.id === booking.id ? { ...item, status: 'CANCELLED', is_overdue: false } : item))
      setNotice('Booking cancelled.')
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
  const canCreate = appointments ? isAdmin || isCustomer : isAdmin
  const needsSetup = canCreate && (customers.length === 0 || services.length === 0)

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
          {canCreate && <Button disabled={needsSetup} onClick={openCreateDialog} type="button"><Plus size={17} /> Add {appointments ? 'appointment' : 'booking'}</Button>}
        </div>

        {notice && <p className="auth-notice auth-notice--success" role="status">{notice}</p>}
        {pageError && <p className="auth-notice auth-notice--error" role="alert">{pageError}</p>}

        {appointments ? (
          <div className="appointment-filters">
            <div className="appointment-filters__tabs" role="group" aria-label="Appointment views">
              {[['', 'All'], ['upcoming', 'Upcoming'], ['today', 'Today'], ['overdue', 'Overdue']].map(([value, label]) => (
                <button aria-pressed={filters.view === value} className={`appointment-filters__tab${filters.view === value ? ' is-active' : ''}`} key={label} onClick={() => updateFilter('view', value)} type="button">{label}</button>
              ))}
            </div>
            <select aria-label="Filter by assignment" onChange={(event) => updateFilter('assignment', event.target.value)} value={filters.assignment}>
              <option value="">Any technician</option><option value="assigned">Assigned</option><option value="unassigned">Unassigned</option>
            </select>
            <select aria-label="Filter by status" onChange={(event) => updateFilter('status', event.target.value)} value={filters.status}>
              <option value="">Any status</option>{statusOptions.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select>
            <form className="appointment-filters__search" onSubmit={submitSearch}>
              <input aria-label="Search appointments" onChange={(event) => setSearchInput(event.target.value)} placeholder="Search customer, service or notes" value={searchInput} />
              <Button type="submit">Search</Button>
            </form>
          </div>
        ) : (
          <div className="appointment-filters">
            <select aria-label="Filter by status" onChange={(event) => updateFilter('status', event.target.value)} value={filters.status}>
              <option value="">Any status</option>{statusOptions.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select>
            <form className="appointment-filters__search" onSubmit={submitSearch}>
              <input aria-label="Search bookings" onChange={(event) => setSearchInput(event.target.value)} placeholder="Search customer or notes" value={searchInput} />
              <Button type="submit">Search</Button>
            </form>
          </div>
        )}
        {loading ? (
          <p className="services-state" role="status">Loading {appointments ? 'appointments' : 'bookings'}...</p>
        ) : needsSetup ? (
          <div className="services-empty">
            <h2>Set up your workspace first</h2>
            <p>Add at least one customer and one service before scheduling.</p>
            <div className="booking-prerequisite-links"><Link className="text-link" to="/customers">Open Customers</Link><Link className="text-link" to="/services">Open Services</Link></div>
          </div>
        ) : bookings.length === 0 && (filters.view || filters.assignment || filters.status || filters.search) ? (
          <div className="services-empty"><h2>No matching {appointments ? 'appointments' : 'bookings'}</h2><p>Try changing or clearing the filters.</p></div>
        ) : bookings.length === 0 ? (
          <div className="services-empty">
            <span className="booking-empty-icon"><CalendarDays size={22} /></span>
            <h2>No {appointments ? 'appointments' : 'bookings'} yet</h2>
            <p>{canCreate ? `Create the first ${appointments ? 'appointment' : 'booking'} for this workspace.` : 'Appointments assigned to you will appear here.'}</p>
            {canCreate && <Button onClick={openCreateDialog} type="button"><Plus size={16} /> Add {appointments ? 'appointment' : 'booking'}</Button>}
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
                    <td className="services-table__name">{booking.customer_name || customerNames.get(booking.customer_id) || 'Unavailable'}</td>
                    <td>{booking.service_name || serviceNames.get(booking.service_id) || 'Unavailable'}</td>
                    {appointments && <td>{booking.technician_name || technicianNames.get(booking.technician_id) || 'Unassigned'}</td>}
                    <td><span className={statusClass(booking.status)}>{statusLabel(booking.status)}</span>{booking.is_overdue && <span className="booking-status booking-status--overdue">OVERDUE</span>}</td>
                    <td className="services-table__description">{booking.notes || 'No notes'}</td>
                    <td>
                      <div className="service-actions">
                        {appointments && <Link className="text-link" to={`/appointments/${booking.id}`}>Details</Link>}
                        {appointments && availableActions(role, booking).map((action) => (
                          <button className={`button button--outline button--small${action.danger ? ' button--danger' : ''}`} key={action.status} onClick={() => cancelBooking(booking, action.status)} type="button">{action.label}</button>
                        ))}
                        {appointments && canEdit(role, booking) && (
                          <button aria-label="Edit appointment" className="service-icon-button" onClick={() => openEditDialog(booking)} title="Edit appointment" type="button"><Pencil size={16} /></button>
                        )}
                        {!appointments && isAdmin && booking.status === 'BOOKED' && (<>
                          <button aria-label="Edit booking" className="service-icon-button" onClick={() => openEditDialog(booking)} title="Edit booking" type="button"><Pencil size={16} /></button>
                          <button aria-label="Cancel booking" className="service-icon-button service-icon-button--delete" onClick={() => cancelBooking(booking)} title="Cancel booking" type="button"><X size={17} /></button>
                        </>)}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {pagination && pagination.total_pages > 1 && (
          <nav aria-label={`${appointments ? 'Appointment' : 'Booking'} pages`} className="appointment-pagination">
            <Button disabled={page <= 1 || loading} onClick={() => setPage((current) => current - 1)} type="button">Previous</Button>
            <span>Page {pagination.page} of {pagination.total_pages}</span>
            <Button disabled={page >= pagination.total_pages || loading} onClick={() => setPage((current) => current + 1)} type="button">Next</Button>
          </nav>
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
              lockCustomer={isCustomer}
              hideTechnician={isCustomer}
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