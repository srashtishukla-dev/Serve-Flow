import { useEffect, useState } from 'react'
import { ArrowLeft, Eye, Receipt, X } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import Logo from '../components/ui/Logo'
import Button from '../components/ui/Button'
import { useAuth } from '../context/AuthContext'
import apiClient from '../services/api/client'

const pageLimit = 20

export default function CustomerInvoices() {
  const { logout } = useAuth()
  const navigate = useNavigate()
  const [invoices, setInvoices] = useState([])
  const [pagination, setPagination] = useState(null)
  const [page, setPage] = useState(1)
  const [reloadKey, setReloadKey] = useState(0)
  const [loading, setLoading] = useState(true)
  const [pageError, setPageError] = useState('')
  const [details, setDetails] = useState(null)
  const [detailsLoading, setDetailsLoading] = useState(false)
  const [detailsError, setDetailsError] = useState('')

  useEffect(() => {
    let active = true
    setLoading(true)
    apiClient.get('/invoices', { params: { page, limit: pageLimit, sort: 'issue_date', order: 'desc' } })
      .then(({ data }) => {
        if (!active) return
        setInvoices(data.invoices || [])
        setPagination(data.pagination || null)
        setPageError('')
      })
      .catch((error) => {
        if (!active) return
        if (error.response?.status === 401) {
          logout()
          navigate('/login', { replace: true })
          return
        }
        setPageError(error.response?.data?.error?.message || 'Unable to load your invoices. Please try again.')
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => { active = false }
  }, [page, reloadKey])

  async function openDetails(invoice) {
    setDetails(null)
    setDetailsLoading(true)
    setDetailsError('')
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

  return (
    <main className="services-page invoices-page">
      <header className="services-header">
        <Logo />
        <Link className="text-link" to="/dashboard"><ArrowLeft size={15} /> Dashboard</Link>
      </header>
      <section aria-labelledby="customer-invoices-title" className="services-content">
        <div className="services-heading">
          <div>
            <span className="section-kicker">YOUR BILLING</span>
            <h1 id="customer-invoices-title">My invoices</h1>
            <p>View your invoices, balances, and recorded payment history.</p>
          </div>
        </div>
        {pageError && <p className="auth-notice auth-notice--error" role="alert">{pageError}</p>}
        {loading ? (
          <p className="services-state" role="status">Loading your invoices...</p>
        ) : pageError ? (
          <div className="services-empty">
            <Button onClick={() => setReloadKey((current) => current + 1)} type="button">Try again</Button>
          </div>
        ) : invoices.length === 0 ? (
          <div className="services-empty">
            <span className="booking-empty-icon"><Receipt size={22} /></span>
            <h2>No invoices yet</h2>
            <p>Invoices issued to you will appear here.</p>
          </div>
        ) : (
          <div className="services-table-wrap">
            <table className="services-table invoices-table">
              <thead>
                <tr><th scope="col">Invoice</th><th scope="col">Issue date</th><th scope="col">Due date</th><th scope="col">Total</th><th scope="col">Paid</th><th scope="col">Balance due</th><th scope="col">Status</th><th scope="col"><span className="sr-only">Details</span></th></tr>
              </thead>
              <tbody>
                {invoices.map((invoice) => (
                  <tr key={invoice.id}>
                    <td className="services-table__name">{invoice.invoice_number}</td>
                    <td>{invoice.issue_date}</td>
                    <td>{invoice.due_date}</td>
                    <td>{formatMoney(invoice.total)}</td>
                    <td>{formatMoney(invoice.amount_paid)}</td>
                    <td>{formatMoney(invoice.balance_due)}</td>
                    <td><span className={`invoice-status invoice-status--${invoice.status.toLowerCase().replaceAll('_', '-')}`}>{invoice.status.replaceAll('_', ' ')}</span></td>
                    <td><button aria-label={`View invoice ${invoice.invoice_number}`} className="service-icon-button" onClick={() => openDetails(invoice)} title="View invoice" type="button"><Eye size={16} /></button></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {pagination?.total_pages > 1 && (
          <nav aria-label="Invoice pages" className="appointment-pagination">
            <Button disabled={page <= 1 || loading} onClick={() => setPage((current) => current - 1)} type="button">Previous</Button>
            <span>Page {pagination.page} of {pagination.total_pages}</span>
            <Button disabled={page >= pagination.total_pages || loading} onClick={() => setPage((current) => current + 1)} type="button">Next</Button>
          </nav>
        )}
      </section>

      {(details || detailsLoading || detailsError) && (
        <div className="services-modal-backdrop">
          <section aria-labelledby="customer-invoice-details-title" aria-modal="true" className="services-modal invoice-details-modal" role="dialog">
            <header className="services-modal__header">
              <div>
                <span className="section-kicker">INVOICE DETAILS</span>
                <h2 id="customer-invoice-details-title">{details?.invoice.invoice_number || 'Invoice details'}</h2>
              </div>
              <button aria-label="Close details" className="service-icon-button" onClick={() => { setDetails(null); setDetailsError('') }} type="button"><X size={18} /></button>
            </header>
            {detailsLoading ? <p className="services-state" role="status">Loading invoice details...</p> : detailsError ? <p className="auth-notice auth-notice--error" role="alert">{detailsError}</p> : details && (
              <CustomerInvoiceDetails details={details} />
            )}
          </section>
        </div>
      )}
    </main>
  )
}

function CustomerInvoiceDetails({ details }) {
  const { invoice, items, payments } = details
  return (
    <div className="invoice-details">
      <div className="invoice-details__facts">
        <div><span>Issue date</span><strong>{invoice.issue_date}</strong></div>
        <div><span>Due date</span><strong>{invoice.due_date}</strong></div>
        <div><span>Status</span><strong><span className={`invoice-status invoice-status--${invoice.status.toLowerCase().replaceAll('_', '-')}`}>{invoice.status.replaceAll('_', ' ')}</span></strong></div>
      </div>
      <h3>Line items</h3>
      {items.length === 0 ? <p className="services-state">No line items are available.</p> : (
        <div className="services-table-wrap invoice-items-table-wrap">
          <table className="services-table invoice-items-table">
            <thead><tr><th scope="col">Description</th><th scope="col">Qty</th><th scope="col">Unit price</th><th scope="col">Amount</th></tr></thead>
            <tbody>{items.map((item) => <tr key={item.id}><td>{item.description}</td><td>{item.quantity}</td><td>{formatMoney(item.unit_price)}</td><td>{formatMoney(item.amount)}</td></tr>)}</tbody>
          </table>
        </div>
      )}
      <dl className="invoice-details__totals">
        <div><dt>Subtotal</dt><dd>{formatMoney(invoice.subtotal)}</dd></div>
        <div><dt>Tax</dt><dd>{formatMoney(invoice.tax)}</dd></div>
        <div><dt>Total</dt><dd>{formatMoney(invoice.total)}</dd></div>
        <div><dt>Paid</dt><dd>{formatMoney(invoice.amount_paid)}</dd></div>
        <div><dt>Balance due</dt><dd>{formatMoney(invoice.balance_due)}</dd></div>
      </dl>
      <h3>Payment history</h3>
      {payments.length === 0 ? <p className="services-state">No payments recorded.</p> : (
        <div className="services-table-wrap invoice-items-table-wrap">
          <table className="services-table invoice-items-table">
            <thead><tr><th scope="col">Date</th><th scope="col">Method</th><th scope="col">Reference</th><th scope="col">Amount</th></tr></thead>
            <tbody>{payments.map((payment) => <tr key={payment.id}><td>{payment.payment_date}</td><td>{payment.payment_method.replaceAll('_', ' ')}</td><td>{payment.reference || '—'}</td><td>{formatMoney(payment.amount)}</td></tr>)}</tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function formatMoney(value) {
  const amount = Number(value)
  if (!Number.isFinite(amount)) return 'INR --'
  return new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(amount)
}
