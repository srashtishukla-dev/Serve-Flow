import { useEffect, useState } from 'react'
import { ArrowLeft, Pencil, Plus, Trash2, X } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import Button from '../components/ui/Button'
import Input from '../components/ui/Input'
import Logo from '../components/ui/Logo'
import { useAuth } from '../context/AuthContext'
import apiClient from '../services/api/client'

const emptyForm = { name: '', description: '', duration: '60', price: '' }

export default function Services() {
  const { logout } = useAuth()
  const navigate = useNavigate()
  const [services, setServices] = useState([])
  const [loading, setLoading] = useState(true)
  const [pageError, setPageError] = useState('')
  const [notice, setNotice] = useState('')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingService, setEditingService] = useState(null)
  const [form, setForm] = useState(emptyForm)
  const [fieldErrors, setFieldErrors] = useState({})
  const [formError, setFormError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    let isActive = true
    apiClient.get('/services')
      .then(({ data }) => {
        if (isActive) setServices(data.services || [])
      })
      .catch((error) => {
        if (error.response?.status === 401) {
          logout()
          navigate('/login', { replace: true })
          return
        }
        if (isActive) setPageError(error.response?.data?.error?.message || 'Unable to load services. Check your connection and try again.')
      })
      .finally(() => {
        if (isActive) setLoading(false)
      })

    return () => { isActive = false }
  }, [])

  function openCreateDialog() {
    setEditingService(null)
    setForm(emptyForm)
    setFieldErrors({})
    setFormError('')
    setDialogOpen(true)
  }

  function openEditDialog(service) {
    setEditingService(service)
    setForm({
      name: service.name,
      description: service.description || '',
      duration: String(service.duration_minutes),
      price: String(service.price),
    })
    setFieldErrors({})
    setFormError('')
    setDialogOpen(true)
  }

  function updateField(event) {
    const { name, value } = event.target
    setForm((current) => ({ ...current, [name]: value }))
    setFieldErrors((current) => ({ ...current, [name]: '' }))
    setFormError('')
  }

  function validateForm() {
    const errors = {}
    const duration = Number(form.duration)
    if (!form.name.trim()) errors.name = 'Enter a service name.'
    else if (form.name.trim().length > 120) errors.name = 'Use 120 characters or fewer.'
    if (form.description.length > 2000) errors.description = 'Use 2000 characters or fewer.'
    if (!/^\d+$/.test(form.duration) || !Number.isSafeInteger(duration) || duration < 1 || duration > 2147483647) {
      errors.duration = 'Enter a whole duration greater than zero minutes.'
    }
    if (!/^\d{1,10}(\.\d{1,2})?$/.test(form.price.trim())) {
      errors.price = 'Enter a non-negative price with up to two decimal places.'
    }
    setFieldErrors(errors)
    return Object.keys(errors).length === 0
  }

  async function saveService(event) {
    event.preventDefault()
    if (!validateForm()) return

    setSubmitting(true)
    setFormError('')
    const payload = {
      name: form.name.trim(),
      description: form.description.trim(),
      duration_minutes: Number(form.duration),
      price: form.price.trim(),
    }

    try {
      if (editingService) {
        const { data } = await apiClient.put(`/services/${editingService.id}`, payload)
        setServices((current) => current.map((service) => service.id === data.service.id ? data.service : service))
        setNotice('Service updated.')
      } else {
        const { data } = await apiClient.post('/services', payload)
        setServices((current) => [data.service, ...current])
        setNotice('Service added.')
      }
      setDialogOpen(false)
    } catch (error) {
      if (error.response?.status === 401) {
        logout()
        navigate('/login', { replace: true })
        return
      }
      setFormError(error.response?.data?.error?.message || 'Unable to save this service. Check your connection and try again.')
    } finally {
      setSubmitting(false)
    }
  }

  async function deleteService(service) {
    if (!window.confirm(`Delete "${service.name}"?`)) return
    setPageError('')
    setNotice('')
    try {
      await apiClient.delete(`/services/${service.id}`)
      setServices((current) => current.filter((item) => item.id !== service.id))
      setNotice('Service deleted.')
    } catch (error) {
      if (error.response?.status === 401) {
        logout()
        navigate('/login', { replace: true })
        return
      }
      setPageError(error.response?.data?.error?.message || 'Unable to delete this service. Please try again.')
    }
  }

  return (
    <main className="services-page">
      <header className="services-header">
        <Logo />
        <Link className="text-link" to="/dashboard"><ArrowLeft size={15} /> Dashboard</Link>
      </header>
      <section className="services-content" aria-labelledby="services-title">
        <div className="services-heading">
          <div>
            <span className="section-kicker">YOUR WORKSPACE CATALOG</span>
            <h1 id="services-title">Services</h1>
            <p>Manage the work your organization offers.</p>
          </div>
          <Button onClick={openCreateDialog} type="button"><Plus size={17} /> Add service</Button>
        </div>

        {notice && <p className="auth-notice auth-notice--success" role="status">{notice}</p>}
        {pageError && <p className="auth-notice auth-notice--error" role="alert">{pageError}</p>}

        {loading ? (
          <p className="services-state" role="status">Loading services...</p>
        ) : services.length === 0 ? (
          <div className="services-empty">
            <h2>No services yet</h2>
            <p>Add the first service offered by this workspace.</p>
            <Button onClick={openCreateDialog} type="button"><Plus size={16} /> Add service</Button>
          </div>
        ) : (
          <div className="services-table-wrap">
            <table className="services-table">
              <thead>
                <tr><th scope="col">Service</th><th scope="col">Description</th><th scope="col">Duration</th><th scope="col">Price</th><th scope="col"><span className="sr-only">Actions</span></th></tr>
              </thead>
              <tbody>
                {services.map((service) => (
                  <tr key={service.id}>
                    <td className="services-table__name">{service.name}</td>
                    <td className="services-table__description">{service.description || 'No description'}</td>
                    <td>{service.duration_minutes} min</td>
                    <td>{formatPrice(service.price)}</td>
                    <td>
                      <div className="service-actions">
                        <button aria-label={`Edit ${service.name}`} className="service-icon-button" onClick={() => openEditDialog(service)} title="Edit service" type="button"><Pencil size={16} /></button>
                        <button aria-label={`Delete ${service.name}`} className="service-icon-button service-icon-button--delete" onClick={() => deleteService(service)} title="Delete service" type="button"><Trash2 size={16} /></button>
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
          <section aria-labelledby="service-dialog-title" aria-modal="true" className="services-modal" role="dialog">
            <header className="services-modal__header">
              <div>
                <span className="section-kicker">SERVICE DETAILS</span>
                <h2 id="service-dialog-title">{editingService ? 'Edit service' : 'Add a service'}</h2>
              </div>
              <button aria-label="Close form" className="service-icon-button" onClick={() => setDialogOpen(false)} type="button"><X size={18} /></button>
            </header>
            {formError && <p className="auth-notice auth-notice--error" role="alert">{formError}</p>}
            <form className="service-form" noValidate onSubmit={saveService}>
              <Input autoFocus error={fieldErrors.name} id="service-name" label="Name" maxLength={120} name="name" onChange={updateField} value={form.name} />
              <div className="form-field">
                <label htmlFor="service-description">Description</label>
                <textarea aria-describedby={fieldErrors.description ? 'service-description-error' : undefined} aria-invalid={Boolean(fieldErrors.description)} className={`form-input services-description${fieldErrors.description ? ' form-input--error' : ''}`} id="service-description" maxLength={2000} name="description" onChange={updateField} rows={3} value={form.description} />
                {fieldErrors.description && <span className="form-field__error" id="service-description-error">{fieldErrors.description}</span>}
              </div>
              <div className="service-form__row">
                <Input error={fieldErrors.duration} id="service-duration" label="Duration (minutes)" min="1" name="duration" onChange={updateField} step="1" type="number" value={form.duration} />
                <Input error={fieldErrors.price} id="service-price" inputMode="decimal" label="Price" name="price" onChange={updateField} placeholder="0.00" type="text" value={form.price} />
              </div>
              <footer className="services-modal__actions">
                <button className="button button--outline" onClick={() => setDialogOpen(false)} type="button">Cancel</button>
                <Button disabled={submitting} type="submit">{submitting ? 'Saving...' : editingService ? 'Save changes' : 'Add service'}</Button>
              </footer>
            </form>
          </section>
        </div>
      )}
    </main>
  )
}

function formatPrice(price) {
  return new Intl.NumberFormat(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(Number(price))
}