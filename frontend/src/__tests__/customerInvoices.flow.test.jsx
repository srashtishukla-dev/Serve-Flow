import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Invoices from '../pages/Invoices'

const auth = vi.hoisted(() => ({ value: {} }))
const api = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('../context/AuthContext', () => ({ useAuth: () => auth.value }))
vi.mock('../services/api/client', () => ({ default: api }))

const invoice = {
  id: 'invoice-1',
  invoice_number: 'INV-000001',
  issue_date: '2030-01-01',
  due_date: '2030-01-31',
  status: 'PARTIALLY_PAID',
  total: '1430.00',
  amount_paid: '500.00',
  balance_due: '930.00',
}
const invoiceDetails = {
  invoice: { ...invoice, subtotal: '1300.00', tax: '130.00' },
  items: [{ id: 'item-1', description: 'Installation', quantity: 2, unit_price: '650.00', amount: '1300.00' }],
  payments: [{ id: 'payment-1', payment_date: '2030-01-10', payment_method: 'UPI', reference: 'receipt-1', amount: '500.00' }],
}

describe('customer invoice flow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    auth.value = { logout: vi.fn(), user: { role: 'CUSTOMER', customer_id: 'customer-1' } }
  })

  it("lists only the authenticated customer's invoices and loads invoice/payment details", async () => {
    api.get.mockImplementation((path) => Promise.resolve({
      data: path === '/invoices'
        ? { invoices: [invoice], pagination: { page: 1, total_pages: 1 } }
        : invoiceDetails,
    }))

    render(<MemoryRouter><Invoices /></MemoryRouter>)
    expect(await screen.findByText('INV-000001')).toBeInTheDocument()
    expect(screen.getByText('PARTIALLY PAID')).toBeInTheDocument()
    expect(screen.getByText(/1,430\.00/)).toBeInTheDocument()
    expect(api.get).toHaveBeenCalledWith('/invoices', { params: { page: 1, limit: 20, sort: 'issue_date', order: 'desc' } })
    expect(api.get).not.toHaveBeenCalledWith('/customers')
    expect(api.get).not.toHaveBeenCalledWith('/appointments')

    await userEvent.click(screen.getByRole('button', { name: 'View invoice INV-000001' }))
    expect(await screen.findByText('Installation')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Payment history' })).toBeInTheDocument()
    expect(screen.getByText('UPI')).toBeInTheDocument()
    expect(screen.getByText('receipt-1')).toBeInTheDocument()
    expect(screen.getAllByText('Balance due').length).toBeGreaterThan(0)
    expect(api.get).toHaveBeenCalledWith('/invoices/invoice-1')
  })

  it('shows API permission errors rather than an empty-state success', async () => {
    api.get.mockRejectedValue({ response: { status: 403, data: { error: { message: 'Invoice access denied' } } } })
    render(<MemoryRouter><Invoices /></MemoryRouter>)
    expect(await screen.findByRole('alert')).toHaveTextContent('Invoice access denied')
  })
})
