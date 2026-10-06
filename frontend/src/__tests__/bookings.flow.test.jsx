import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Bookings from '../pages/Bookings'

const auth = vi.hoisted(() => ({ value: {} }))
const api = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() }))
vi.mock('../context/AuthContext', () => ({ useAuth: () => auth.value }))
vi.mock('../services/api/client', () => ({ default: api }))

const booking = {
  id: 'booking-1',
  customer_id: 'customer-1',
  service_id: 'service-1',
  booking_date: '2030-01-15',
  start_time: '10:00',
  end_time: '11:00',
  status: 'BOOKED',
  notes: 'Front door repair',
}

describe('booking list flow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    auth.value = { logout: vi.fn(), user: { role: 'ADMIN', name: 'Workspace admin' } }
    api.get.mockImplementation((path) => Promise.resolve({
      data: path === '/customers'
        ? { customers: [{ id: 'customer-1', name: 'Cara Customer' }] }
        : path === '/services'
          ? { services: [{ id: 'service-1', name: 'Repair' }] }
          : { bookings: [booking], pagination: { page: 1, limit: 20, total: 21, total_pages: 2 } },
    }))
  })

  it('requests booking pages, filters by status, and displays results', async () => {
    render(<MemoryRouter><Bookings /></MemoryRouter>)
    expect(await screen.findByText('Cara Customer')).toBeInTheDocument()
    expect(screen.getByText('Repair')).toBeInTheDocument()
    expect(api.get).toHaveBeenCalledWith('/bookings', { params: { page: 1, limit: 20, sort: 'booking_date', order: 'asc' } })

    await userEvent.click(screen.getByRole('button', { name: 'Next' }))
    await waitFor(() => expect(api.get).toHaveBeenCalledWith('/bookings', { params: { page: 2, limit: 20, sort: 'booking_date', order: 'asc' } }))

    await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Filter by status' }), 'ASSIGNED')
    await waitFor(() => expect(api.get).toHaveBeenCalledWith('/bookings', { params: { page: 1, limit: 20, sort: 'booking_date', order: 'asc', status: 'ASSIGNED' } }))
  })
})
