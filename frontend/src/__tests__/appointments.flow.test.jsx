import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Bookings from '../pages/Bookings'
import RequireAuth from '../components/layout/RequireAuth'

const auth = vi.hoisted(() => ({ value: {} }))
const api = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() }))
vi.mock('../context/AuthContext', () => ({ useAuth: () => auth.value }))
vi.mock('../services/api/client', () => ({ default: api }))

const appt = (over = {}) => ({
  id: 7, customer_id: 1, service_id: 2, customer_name: 'Cara Customer', service_name: 'Install',
  technician_name: 'Tom Tech', status: 'ASSIGNED', appointment_date: '2030-01-01', start_time: '10:00', end_time: '11:00', ...over,
})
const page = (items) => ({ data: { appointments: items, pagination: { page: 1, limit: 10, total: items.length, total_pages: 1 } } })
const services = { data: { services: [{ id: 2, name: 'Install', price: 10, duration_minutes: 60 }] } }
const renderPage = () => render(<MemoryRouter><Bookings appointments /></MemoryRouter>)

describe('appointment flows', () => {
  beforeEach(() => { vi.clearAllMocks(); auth.value = { logout: vi.fn(), user: { role: 'TECHNICIAN', name: 'Tom' } } })

  it('technician sees assigned appointment and can start it', async () => {
    api.get.mockResolvedValue(page([appt()]))
    api.post.mockResolvedValue({ data: { appointment: appt({ status: 'IN_PROGRESS' }) } })
    renderPage()
    await screen.findByText('Cara Customer')
    expect(api.get).not.toHaveBeenCalledWith('/customers')
    await userEvent.click(screen.getByRole('button', { name: /start/i }))
    await waitFor(() => expect(api.post).toHaveBeenCalledWith('/appointments/7/status', { status: 'IN_PROGRESS' }))
  })

  it('customer can cancel own appointment', async () => {
    auth.value = { logout: vi.fn(), user: { role: 'CUSTOMER', name: 'Cara', customer_id: 1 } }
    api.get.mockImplementation((url) => Promise.resolve(url === '/appointments' ? page([appt({ status: 'BOOKED' })]) : services))
    api.post.mockResolvedValue({ data: { appointment: appt({ status: 'CANCELLED' }) } })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderPage()
    await screen.findByText('Cara Customer')
    await userEvent.click(screen.getByRole('button', { name: /cancel/i }))
    await waitFor(() => expect(api.post).toHaveBeenCalledWith('/appointments/7/status', { status: 'CANCELLED' }))
  })

  it('shows backend error on forbidden', async () => {
    api.get.mockRejectedValue({ response: { status: 403, data: { error: { message: 'Forbidden' } } } })
    renderPage()
    expect(await screen.findByText(/forbidden/i)).toBeInTheDocument()
  })

  it('RequireAuth blocks wrong role', () => {
    auth.value = { status: 'ready', user: { role: 'CUSTOMER' } }
    render(<MemoryRouter><RequireAuth roles={['ADMIN']}><p>secret</p></RequireAuth></MemoryRouter>)
    expect(screen.queryByText('secret')).toBeNull()
    expect(screen.getByRole('alert')).toHaveTextContent(/access denied/i)
  })
})
