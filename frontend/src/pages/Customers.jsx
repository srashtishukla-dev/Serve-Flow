import { useEffect, useState } from 'react'
import { ArrowLeft, Pencil, Plus, Trash2, X } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import CustomerForm from '../components/layout/CustomerForm'
import Button from '../components/ui/Button'
import Logo from '../components/ui/Logo'
import { useAuth } from '../context/AuthContext'
import apiClient from '../services/api/client'

const emptyCustomer = { name: '', email: '', phone: '', notes: '' }

export default function Customers() {
  const { logout } = useAuth()
  const navigate = useNavigate()
  const [customers, setCustomers] = useState([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [pageError, setPageError] = useState('')
  const [formError, setFormError] = useState('')
  const [notice, setNotice] = useState('')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingCustomer, setEditingCustomer] = useState(null)

  useEffect(() => {
    let isActive = true
    apiClient.get('/customers')
      .then(({ data }) => {
        if (isActive) setCustomers(data.customers || [])
      })
      .catch((error) => {
        if (error.response?.status === 401) {
          logout()
          navigate('/login', { replace: true })
          return
        }
        if (isActive) setPageError(error.response?.data?.error?.message || 'Unable to load customers. Check your connection and try again.')
      })
      .finally(() => {
        if (isActive) setLoading(false)
      })

    return () => { isActive = false }
  }, [])

  function openCreateDialog() {
    setEditingCustomer(null)
    setFormError('')
    setNotice('')
    setDialogOpen(true)
  }

  function openEditDialog(customer) {
    setEditingCustomer(customer)
    setFormError('')
    setNotice('')
    setDialogOpen(true)
  }

  async function saveCustomer(values) {
    setSaving(true)
    setFormError('')
    try {
      if (editingCustomer) {
        const { data } = await apiClient.put(`/customers/${editingCustomer.id}`, values)
        setCustomers((current) => current.map((customer) => customer.id === data.customer.id ? data.customer : customer))
        setNotice('Customer updated.')
      } else {
        const { data } = await apiClient.post('/customers', values)
        setCustomers((current) => [data.customer, ...current])
        setNotice('Customer added.')
      }
      setDialogOpen(false)
    } catch (error) {
      if (error.response?.status === 401) {
        logout()
        navigate('/login', { replace: true })
        return
      }
      setFormError(error.response?.data?.error?.message || 'Unable to save this customer. Check your connection and try again.')
    } finally {
      setSaving(false)
    }
  }

  async function deleteCustomer(customer) {
    if (!window.confirm(`Delete "${customer.name}"?`)) return
    setPageError('')
    setNotice('')
    try {
      await apiClient.delete(`/customers/${customer.id}`)
      setCustomers((current) => current.filter((item) => item.id !== customer.id))
      setNotice('Customer deleted.')
    } catch (error) {
      if (error.response?.status === 401) {
        logout()
        navigate('/login', { replace: true })
        return
      }
      setPageError(error.response?.data?.error?.message || 'Unable to delete this customer. Please try again.')
    }
  }

  return (
    <main className="services-page customers-page">
      <header className="services-header">
        <Logo />
        <Link className="text-link" to="/dashboard"><ArrowLeft size={15} /> Dashboard</Link>
      </header>
      <section className="services-content" aria-labelledby="customers-title">
        <div className="services-heading">
          <div>
            <span className="section-kicker">YOUR WORKSPACE DIRECTORY</span>
            <h1 id="customers-title">Customers</h1>
            <p>Keep customer contact details for this organization.</p>
          </div>
          <Button onClick={openCreateDialog} type="button"><Plus size={17} /> Add customer</Button>
        </div>

        {notice && <p className="auth-notice auth-notice--success" role="status">{notice}</p>}
        {pageError && <p className="auth-notice auth-notice--error" role="alert">{pageError}</p>}

        {loading ? (
          <p className="services-state" role="status">Loading customers...</p>
        ) : customers.length === 0 ? (
          <div className="services-empty">
            <h2>No customers yet</h2>
            <p>Add a customer to this workspace directory.</p>
            <Button onClick={openCreateDialog} type="button"><Plus size={16} /> Add customer</Button>
          </div>
        ) : (
          <div className="services-table-wrap">
            <table className="services-table">
              <thead>
                <tr><th scope="col">Name</th><th scope="col">Email</th><th scope="col">Phone</th><th scope="col">Notes</th><th scope="col"><span className="sr-only">Actions</span></th></tr>
              </thead>
              <tbody>
                {customers.map((customer) => (
                  <tr key={customer.id}>
                    <td className="services-table__name">{customer.name}</td>
                    <td>{customer.email || 'Not provided'}</td>
                    <td>{customer.phone || 'Not provided'}</td>
                    <td className="services-table__description">{customer.notes || 'No notes'}</td>
                    <td>
                      <div className="service-actions">
                        <button aria-label={`Edit ${customer.name}`} className="service-icon-button" onClick={() => openEditDialog(customer)} title="Edit customer" type="button"><Pencil size={16} /></button>
                        <button aria-label={`Delete ${customer.name}`} className="service-icon-button service-icon-button--delete" onClick={() => deleteCustomer(customer)} title="Delete customer" type="button"><Trash2 size={16} /></button>
                      </div>
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
          <section aria-labelledby="customer-dialog-title" aria-modal="true" className="services-modal" role="dialog">
            <header className="services-modal__header">
              <div>
                <span className="section-kicker">CUSTOMER DETAILS</span>
                <h2 id="customer-dialog-title">{editingCustomer ? 'Edit customer' : 'Add a customer'}</h2>
              </div>
              <button aria-label="Close form" className="service-icon-button" onClick={() => setDialogOpen(false)} type="button"><X size={18} /></button>
            </header>
            <CustomerForm
              key={editingCustomer?.id || 'new-customer'}
              error={formError}
              initialValues={editingCustomer || emptyCustomer}
              onCancel={() => setDialogOpen(false)}
              onSubmit={saveCustomer}
              saving={saving}
            />
          </section>
        </div>
      )}
    </main>
  )
}