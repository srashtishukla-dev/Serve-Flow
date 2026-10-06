import { useEffect, useState } from 'react'
import { ArrowLeft, Eye, FileText, Pencil, Plus, Receipt, X } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import InvoiceForm from '../components/layout/InvoiceForm'
import Button from '../components/ui/Button'
import CustomerInvoices from './CustomerInvoices'
import Input from '../components/ui/Input'
import Logo from '../components/ui/Logo'
import { useAuth } from '../context/AuthContext'
import apiClient from '../services/api/client'

const emptyPayment = { amount: '', payment_date: localDate(), payment_method: 'CASH', reference: '', notes: '' }

export default function Invoices() {
  const { user } = useAuth()
  return user?.role === 'CUSTOMER' ? <CustomerInvoices /> : <AdminInvoices />
}

function AdminInvoices() {
  const { logout } = useAuth()
  const navigate = useNavigate()
  const [invoices, setInvoices] = useState([])
  const [customers, setCustomers] = useState([])
  const [appointments, setAppointments] = useState([])
  const [loading, setLoading] = useState(true)
  const [pageError, setPageError] = useState('')
  const [notice, setNotice] = useState('')
  const [invoiceDialogOpen, setInvoiceDialogOpen] = useState(false)
  const [editingInvoice, setEditingInvoice] = useState(null)
  const [formError, setFormError] = useState('')
  const [savingInvoice, setSavingInvoice] = useState(false)
  const [details, setDetails] = useState(null)
  const [detailsLoading, setDetailsLoading] = useState(false)
  const [detailsError, setDetailsError] = useState('')
  const [payment, setPayment] = useState(emptyPayment)
  const [paymentError, setPaymentError] = useState('')
  const [paymentNotice, setPaymentNotice] = useState('')
  const [savingPayment, setSavingPayment] = useState(false)

  useEffect(() => {
    let isActive = true
    Promise.all([apiClient.get('/invoices'), apiClient.get('/customers'), apiClient.get('/appointments')])
      .then(([invoiceResponse, customerResponse, appointmentResponse]) => {
        if (!isActive) return
        setInvoices(invoiceResponse.data.invoices || [])
        setCustomers(customerResponse.data.customers || [])
        setAppointments(appointmentResponse.data.appointments || [])
      })
      .catch((error) => {
        if (error.response?.status === 401) {
          logout()
          navigate('/login', { replace: true })
          return
        }
        if (isActive) setPageError(error.response?.data?.error?.message || 'Unable to load invoices. Check your connection and try again.')
      })
      .finally(() => {
        if (isActive) setLoading(false)
      })
    return () => { isActive = false }
  }, [])

  async function refreshInvoices() {
    const { data } = await apiClient.get('/invoices')
    setInvoices(data.invoices || [])
  }

  function openCreateDialog() {
    setEditingInvoice(null)
    setFormError('')
    setNotice('')
    setInvoiceDialogOpen(true)
  }

  async function openDetails(invoice) {
    setDetailsLoading(true)
    setDetailsError('')
    setPaymentError('')
    setPaymentNotice('')
    setPayment(emptyPayment)
    setDetails(null)
    try {
      const { data } = await apiClient.get(`/invoices/${invoice.id}`)
      setDetails(data)
    } catch (error) {
      if (error.response?.status === 401) {
        logout()
        navigate('/login', { replace: true })
        return
      }
      setDetailsError(error.response?.data?.error?.message || 'Unable to load invoice details. Please try again.')
    } finally {
      setDetailsLoading(false)
    }
  }

  function startEditing() {
    if (!details) return
    const invoice = details.invoice
    setEditingInvoice({
      ...invoice,
      customer_id: invoice.customer_id,
      appointment_id: invoice.appointment_id || '',
      issue_date: invoice.issue_date,
      due_date: invoice.due_date,
      items: details.items,
    })
    setFormError('')
    setInvoiceDialogOpen(true)
    setDetails(null)
  }

  async function saveInvoice(values) {
    setSavingInvoice(true)
    setFormError('')
    try {
      if (editingInvoice) {
        await apiClient.put(`/invoices/${editingInvoice.id}`, values)
        setNotice('Invoice updated.')
      } else {
        await apiClient.post('/invoices', values)
        setNotice('Invoice created.')
      }
      setInvoiceDialogOpen(false)
      await refreshInvoices()
    } catch (error) {
      if (error.response?.status === 401) {
        logout()
        navigate('/login', { replace: true })
        return
      }
      setFormError(error.response?.data?.error?.message || 'Unable to save this invoice. Please try again.')
    } finally {
      setSavingInvoice(false)
    }
  }

  async function cancelInvoice() {
    if (!details || !window.confirm(`Cancel invoice ${details.invoice.invoice_number}?`)) return
    setPageError('')
    try {
      await apiClient.delete(`/invoices/${details.invoice.id}`)
      setDetails((current) => ({ ...current, invoice: { ...current.invoice, status: 'CANCELLED' } }))
      setNotice('Invoice cancelled.')
      await refreshInvoices()
    } catch (error) {
      if (error.response?.status === 401) {
        logout()
        navigate('/login', { replace: true })
        return
      }
      setPageError(error.response?.data?.error?.message || 'Unable to cancel this invoice. Please try again.')
    }
  }

  async function recordPayment(event) {
    event.preventDefault()
    if (!details) return
    const amountCents = parseCents(payment.amount.trim())
    const balanceCents = parseCents(String(details.invoice.balance_due))
    if (amountCents === null || amountCents === 0) {
      setPaymentError('Enter a payment amount greater than zero with up to two decimals.')
      return
    }
    if (!isDate(payment.payment_date)) {
      setPaymentError('Select a valid payment date.')
      return
    }
    if (balanceCents === null || amountCents > balanceCents) {
      setPaymentError('Payment cannot exceed the remaining balance.')
      return
    }

    setSavingPayment(true)
    setPaymentError('')
    setPaymentNotice('')
    try {
      const { data } = await apiClient.post(`/invoices/${details.invoice.id}/payments`, payment)
      setDetails((current) => ({
        ...current,
        invoice: data.invoice,
        payments: [...current.payments, data.payment],
      }))
      setPaymentNotice('Payment recorded.')
      setPayment((current) => ({ ...emptyPayment, payment_date: current.payment_date, payment_method: current.payment_method }))
      await refreshInvoices()
    } catch (error) {
      if (error.response?.status === 401) {
        logout()
        navigate('/login', { replace: true })
        return
      }
      setPaymentError(error.response?.data?.error?.message || 'Unable to record this payment. Please try again.')
    } finally {
      setSavingPayment(false)
    }
  }

  function updatePayment(event) {
    const { name, value } = event.target
    setPayment((current) => ({ ...current, [name]: value }))
    setPaymentError('')
  }

  const outstanding = invoices.filter((invoice) => Number(invoice.balance_due) > 0 && invoice.status !== 'CANCELLED')

  return (
    <main className="services-page invoices-page">
      <header className="services-header">
        <Logo />
        <Link className="text-link" to="/dashboard"><ArrowLeft size={15} /> Dashboard</Link>
      </header>
      <section className="services-content" aria-labelledby="invoices-title">
        <div className="services-heading">
          <div>
            <span className="section-kicker">YOUR WORKSPACE BILLING</span>
            <h1 id="invoices-title">Invoices</h1>
            <p>Create invoices and record payments for this organization.</p>
          </div>
          <Button disabled={customers.length === 0} onClick={openCreateDialog} type="button"><Plus size={17} /> Create invoice</Button>
        </div>

        {notice && <p className="auth-notice auth-notice--success" role="status">{notice}</p>}
        {pageError && <p className="auth-notice auth-notice--error" role="alert">{pageError}</p>}

        {loading ? (
          <p className="services-state" role="status">Loading invoices...</p>
        ) : customers.length === 0 ? (
          <div className="services-empty">
            <h2>Add a customer first</h2>
            <p>Invoices need to be associated with a customer.</p>
            <Link className="text-link" to="/customers">Open Customers</Link>
          </div>
        ) : invoices.length === 0 ? (
          <div className="services-empty">
            <span className="booking-empty-icon"><Receipt size={22} /></span>
            <h2>No invoices yet</h2>
            <p>Create the first invoice for this workspace.</p>
            <Button onClick={openCreateDialog} type="button"><Plus size={16} /> Create invoice</Button>
          </div>
        ) : (
          <div className="services-table-wrap">
            <table className="services-table invoices-table">
              <thead>
                <tr><th scope="col">Invoice</th><th scope="col">Customer</th><th scope="col">Issue date</th><th scope="col">Due date</th><th scope="col">Total</th><th scope="col">Balance</th><th scope="col">Status</th><th scope="col"><span className="sr-only">Details</span></th></tr>
              </thead>
              <tbody>
                {invoices.map((invoice) => (
                  <tr key={invoice.id}>
                    <td className="services-table__name">{invoice.invoice_number}</td>
                    <td>{invoice.customer_name || 'Customer'}</td>
                    <td>{invoice.issue_date}</td>
                    <td>{invoice.due_date}</td>
                    <td>{displayMoney(invoice.total)}</td>
                    <td>{displayMoney(invoice.balance_due)}</td>
                    <td><span className={`invoice-status invoice-status--${invoice.status.toLowerCase().replaceAll('_', '-')}`}>{invoice.status.replaceAll('_', ' ')}</span></td>
                    <td><button aria-label={`View invoice ${invoice.invoice_number}`} className="service-icon-button" onClick={() => openDetails(invoice)} title="View invoice" type="button"><Eye size={16} /></button></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {outstanding.length > 0 && (
        <aside aria-label="Outstanding invoice summary" className="invoice-outstanding-summary">
          <span>{outstanding.length} outstanding {outstanding.length === 1 ? 'invoice' : 'invoices'}</span>
          <strong>Balance due: {displayCents(outstanding.reduce((sum, invoice) => sum + parseCents(String(invoice.balance_due)), 0n))}</strong>
        </aside>
      )}

      {invoiceDialogOpen && (
        <div className="services-modal-backdrop">
          <section aria-labelledby="invoice-dialog-title" aria-modal="true" className="services-modal invoice-edit-modal" role="dialog">
            <header className="services-modal__header">
              <div>
                <span className="section-kicker">INVOICE DETAILS</span>
                <h2 id="invoice-dialog-title">{editingInvoice ? `Edit ${editingInvoice.invoice_number}` : 'Create an invoice'}</h2>
              </div>
              <button aria-label="Close form" className="service-icon-button" onClick={() => setInvoiceDialogOpen(false)} type="button"><X size={18} /></button>
            </header>
            <InvoiceForm
              key={editingInvoice?.id || 'new-invoice'}
              appointments={appointments}
              error={formError}
              initialValues={editingInvoice || {}}
              onCancel={() => setInvoiceDialogOpen(false)}
              onSubmit={saveInvoice}
              saving={savingInvoice}
              customers={customers}
            />
          </section>
        </div>
      )}

      {(details || detailsLoading || detailsError) && (
        <div className="services-modal-backdrop">
          <section aria-labelledby="invoice-details-title" aria-modal="true" className="services-modal invoice-details-modal" role="dialog">
            <header className="services-modal__header">
              <div>
                <span className="section-kicker">INVOICE</span>
                <h2 id="invoice-details-title">{details?.invoice.invoice_number || 'Invoice details'}</h2>
              </div>
              <button aria-label="Close details" className="service-icon-button" onClick={() => setDetails(null)} type="button"><X size={18} /></button>
            </header>
            {detailsLoading ? <p className="services-state" role="status">Loading invoice details...</p> : detailsError ? <p className="auth-notice auth-notice--error" role="alert">{detailsError}</p> : details && (
              <InvoiceDetails
                details={details}
                payment={payment}
                paymentError={paymentError}
                paymentNotice={paymentNotice}
                savingPayment={savingPayment}
                onEdit={startEditing}
                onCancelInvoice={cancelInvoice}
                onPaymentChange={updatePayment}
                onRecordPayment={recordPayment}
              />
            )}
          </section>
        </div>
      )}
    </main>
  )
}

function InvoiceDetails({ details, payment, paymentError, paymentNotice, savingPayment, onEdit, onCancelInvoice, onPaymentChange, onRecordPayment }) {
  const { invoice, customer, items, payments } = details
  const isUnpaidIssued = invoice.status === 'ISSUED' && Number(invoice.amount_paid) === 0
  const canPay = (invoice.status === 'ISSUED' || invoice.status === 'PARTIALLY_PAID') && Number(invoice.balance_due) > 0

  return (
    <div className="invoice-details">
      <div className="invoice-details__facts">
        <div><span>Customer</span><strong>{customer.name}</strong><small>{customer.email || customer.phone || 'No contact details'}</small></div>
        <div><span>Issue date</span><strong>{invoice.issue_date}</strong></div>
        <div><span>Due date</span><strong>{invoice.due_date}</strong></div>
        <div><span>Status</span><strong><span className={`invoice-status invoice-status--${invoice.status.toLowerCase().replaceAll('_', '-')}`}>{invoice.status.replaceAll('_', ' ')}</span></strong></div>
      </div>
      {invoice.appointment_id && <p className="invoice-details__appointment">Appointment linked: {invoice.appointment_id}</p>}
      {invoice.notes && <p className="invoice-details__notes">{invoice.notes}</p>}

      <h3>Line items</h3>
      <div className="services-table-wrap invoice-items-table-wrap">
        <table className="services-table invoice-items-table">
          <thead><tr><th scope="col">Description</th><th scope="col">Qty</th><th scope="col">Unit price</th><th scope="col">Amount</th></tr></thead>
          <tbody>{items.map((item) => <tr key={item.id}><td>{item.description}</td><td>{item.quantity}</td><td>{displayMoney(item.unit_price)}</td><td>{displayMoney(item.amount)}</td></tr>)}</tbody>
        </table>
      </div>
      <dl className="invoice-details__totals">
        <div><dt>Subtotal</dt><dd>{displayMoney(invoice.subtotal)}</dd></div>
        <div><dt>Tax</dt><dd>{displayMoney(invoice.tax)}</dd></div>
        <div><dt>Total</dt><dd>{displayMoney(invoice.total)}</dd></div>
        <div><dt>Paid</dt><dd>{displayMoney(invoice.amount_paid)}</dd></div>
        <div><dt>Balance due</dt><dd>{displayMoney(invoice.balance_due)}</dd></div>
      </dl>

      <h3>Payments</h3>
      {payments.length === 0 ? <p className="services-state">No payments recorded.</p> : (
        <div className="services-table-wrap invoice-items-table-wrap">
          <table className="services-table invoice-items-table">
            <thead><tr><th scope="col">Date</th><th scope="col">Method</th><th scope="col">Reference</th><th scope="col">Amount</th></tr></thead>
            <tbody>{payments.map((entry) => <tr key={entry.id}><td>{entry.payment_date}</td><td>{entry.payment_method.replaceAll('_', ' ')}</td><td>{entry.reference || '—'}</td><td>{displayMoney(entry.amount)}</td></tr>)}</tbody>
          </table>
        </div>
      )}

      {canPay && (
        <form className="invoice-payment-form" noValidate onSubmit={onRecordPayment}>
          <h3>Record payment</h3>
          <div className="invoice-payment-form__fields">
            <Input id="payment-amount" inputMode="decimal" label="Amount" name="amount" onChange={onPaymentChange} placeholder="0.00" type="text" value={payment.amount} />
            <Input id="payment-date" label="Payment date" name="payment_date" onChange={onPaymentChange} type="date" value={payment.payment_date} />
            <label className="form-field" htmlFor="payment-method"><span>Payment method</span>
              <select className="form-input" id="payment-method" name="payment_method" onChange={onPaymentChange} value={payment.payment_method}>
                <option value="CASH">Cash</option><option value="CARD">Card</option><option value="UPI">UPI</option><option value="BANK_TRANSFER">Bank transfer</option><option value="OTHER">Other</option>
              </select>
            </label>
            <Input id="payment-reference" label="Reference (optional)" maxLength={120} name="reference" onChange={onPaymentChange} value={payment.reference} />
          </div>
          <div className="form-field">
            <label htmlFor="payment-notes">Notes (optional)</label>
            <textarea className="form-input services-description" id="payment-notes" maxLength={1000} name="notes" onChange={onPaymentChange} rows={2} value={payment.notes} />
          </div>
          {paymentNotice && <p className="auth-notice auth-notice--success" role="status">{paymentNotice}</p>}
          {paymentError && <p className="auth-notice auth-notice--error" role="alert">{paymentError}</p>}
          <Button disabled={savingPayment} type="submit">{savingPayment ? 'Recording...' : 'Record payment'}</Button>
        </form>
      )}

      <footer className="services-modal__actions invoice-details__actions">
        {isUnpaidIssued && <>
          <button className="button button--outline" onClick={onCancelInvoice} type="button">Cancel invoice</button>
          <Button className="button--outline" onClick={onEdit} type="button"><Pencil size={15} /> Edit invoice</Button>
        </>}
      </footer>
    </div>
  )
}

function localDate() {
  const date = new Date()
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

function isDate(value) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
  const [year, month, day] = value.split('-').map(Number)
  const date = new Date(year, month - 1, day)
  return date.getFullYear() === year && date.getMonth() === month - 1 && date.getDate() === day
}

function parseCents(value) {
  const match = /^(\d+)(?:\.(\d{1,2}))?$/.exec(value)
  if (!match) return null
  return BigInt(match[1]) * 100n + BigInt((match[2] || '').padEnd(2, '0') || '0')
}

function displayMoney(value) {
  const amount = String(value ?? '0')
  const [whole, fraction = ''] = amount.split('.')
  return `INR ${whole}.${fraction.padEnd(2, '0').slice(0, 2)}`
}

function displayCents(cents) {
  return `INR ${cents / 100n}.${String(cents % 100n).padStart(2, '0')}`
}