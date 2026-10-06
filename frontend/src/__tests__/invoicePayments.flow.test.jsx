import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Invoices from '../pages/Invoices'

const auth = vi.hoisted(() => ({ value: {} }))
const api = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() }))
vi.mock('../context/AuthContext', () => ({ useAuth: () => auth.value }))
vi.mock('../services/api/client', () => ({ default: api }))

describe('admin invoice balances', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    auth.value = { logout: vi.fn(), user: { role: 'ADMIN' } }
    api.get.mockImplementation((url) => {
      if (url === '/invoices') return Promise.resolve({ data: { invoices: [{
        id: 'invoice-1',
        invoice_number: 'INV-000001',
        customer_name: 'QA Customer',
        issue_date: '2026-10-04',
        due_date: '2026-10-18',
        total: '275.00',
        balance_due: '275.00',
        status: 'ISSUED',
      }] } })
      if (url === '/customers') return Promise.resolve({ data: { customers: [] } })
      return Promise.resolve({ data: { appointments: [] } })
    })
  })

  it('formats the aggregate outstanding balance in currency units', async () => {
    render(<MemoryRouter><Invoices /></MemoryRouter>)
    expect(await screen.findByRole('complementary', { name: 'Outstanding invoice summary' })).toHaveTextContent('Balance due: INR 275.00')
  })
})
