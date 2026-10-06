import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Analytics from '../pages/Analytics'

const auth = vi.hoisted(() => ({ value: {} }))
const api = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('../context/AuthContext', () => ({ useAuth: () => auth.value }))
vi.mock('../services/api/client', () => ({ default: api }))

const summary = {
  metrics: {
    appointments: 4, completed_appointments: 2, cancelled_appointments: 1, no_show_appointments: 1,
    total_technicians: 2, active_technicians: 1, services: 3, total_invoiced_amount: '1500.00',
    total_invoices: 2, total_revenue: '500.00', total_payments: 1, outstanding_balance: '1000.00',
    pending_invoices: 1, paid_invoices: 1,
  },
  popular_services: [{ id: 'service-1', name: 'Deep clean', bookings: 5 }],
  top_customers: [{ id: 'customer-1', name: 'Sam Customer', bookings: 3, last_booking_date: '2026-05-09' }],
  technician_workload: [{ id: 'tech-1', name: 'Taylor Technician', active_bookings: 2, completed_bookings: 4 }],
}
const activity = {
  range: 'last_30_days', start_date: '2026-04-10', end_date: '2026-05-09',
  days: [{ date: '2026-05-09', appointments: 2, completed_appointments: 1, cancelled_appointments: 0, no_show_appointments: 0, revenue: '250.00' }],
}

describe('analytics flows', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    auth.value = { logout: vi.fn() }
    api.get.mockImplementation((url) => Promise.resolve({ data: url.endsWith('/summary') ? summary : activity }))
  })

  it('shows organization insights and fetches activity for the selected range', async () => {
    render(<MemoryRouter><Analytics /></MemoryRouter>)
    await screen.findByText('Deep clean')
    expect(screen.getByText('Sam Customer')).toBeInTheDocument()
    expect(screen.getByText('Taylor Technician')).toBeInTheDocument()
    expect(screen.getByRole('table')).toHaveTextContent('2')
    expect(api.get).toHaveBeenCalledWith('/analytics/activity', { params: { range: 'last_30_days' } })

    await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Date range' }), 'last_7_days')
    await waitFor(() => expect(api.get).toHaveBeenCalledWith('/analytics/activity', { params: { range: 'last_7_days' } }))
  })

  it('reports an analytics API failure and offers retry', async () => {
    api.get.mockRejectedValue({ response: { status: 403, data: { error: { message: 'Forbidden' } } } })
    render(<MemoryRouter><Analytics /></MemoryRouter>)
    expect(await screen.findByRole('alert')).toHaveTextContent('Forbidden')
  })
})
