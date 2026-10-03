import { useEffect, useState } from 'react'
import { ArrowLeft, Pencil, Plus, UserRoundMinus, X } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import TechnicianForm from '../components/layout/TechnicianForm'
import Button from '../components/ui/Button'
import Logo from '../components/ui/Logo'
import { useAuth } from '../context/AuthContext'
import apiClient from '../services/api/client'

const emptyTechnician = { name: '', email: '', phone: '' }

export default function Technicians() {
  const { logout } = useAuth()
  const navigate = useNavigate()
  const [technicians, setTechnicians] = useState([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [pageError, setPageError] = useState('')
  const [formError, setFormError] = useState('')
  const [notice, setNotice] = useState('')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingTechnician, setEditingTechnician] = useState(null)

  useEffect(() => {
    let isActive = true
    apiClient.get('/technicians')
      .then(({ data }) => {
        if (isActive) setTechnicians(data.technicians || [])
      })
      .catch((error) => {
        if (error.response?.status === 401) {
          logout()
          navigate('/login', { replace: true })
          return
        }
        if (isActive) setPageError(error.response?.data?.error?.message || 'Unable to load technicians. Check your connection and try again.')
      })
      .finally(() => {
        if (isActive) setLoading(false)
      })

    return () => { isActive = false }
  }, [])

  function openCreateDialog() {
    setEditingTechnician(null)
    setFormError('')
    setNotice('')
    setDialogOpen(true)
  }

  function openEditDialog(technician) {
    setEditingTechnician(technician)
    setFormError('')
    setNotice('')
    setDialogOpen(true)
  }

  async function saveTechnician(values) {
    setSaving(true)
    setFormError('')
    try {
      if (editingTechnician) {
        const { data } = await apiClient.put(`/technicians/${editingTechnician.id}`, values)
        setTechnicians((current) => current.map((item) => item.id === data.technician.id ? data.technician : item))
        setNotice('Technician updated.')
      } else {
        const { data } = await apiClient.post('/technicians', values)
        setTechnicians((current) => [data.technician, ...current])
        setNotice('Technician added.')
      }
      setDialogOpen(false)
    } catch (error) {
      if (error.response?.status === 401) {
        logout()
        navigate('/login', { replace: true })
        return
      }
      setFormError(error.response?.data?.error?.message || 'Unable to save this technician. Check your connection and try again.')
    } finally {
      setSaving(false)
    }
  }

  async function deactivateTechnician(technician) {
    if (!window.confirm(`Deactivate ${technician.name}?`)) return
    setPageError('')
    setNotice('')
    try {
      await apiClient.delete(`/technicians/${technician.id}`)
      setTechnicians((current) => current.map((item) => item.id === technician.id ? { ...item, status: 'INACTIVE' } : item))
      setNotice('Technician deactivated.')
    } catch (error) {
      if (error.response?.status === 401) {
        logout()
        navigate('/login', { replace: true })
        return
      }
      setPageError(error.response?.data?.error?.message || 'Unable to deactivate this technician. Please try again.')
    }
  }

  return (
    <main className="services-page technicians-page">
      <header className="services-header">
        <Logo />
        <Link className="text-link" to="/dashboard"><ArrowLeft size={15} /> Dashboard</Link>
      </header>
      <section className="services-content" aria-labelledby="technicians-title">
        <div className="services-heading">
          <div>
            <span className="section-kicker">YOUR WORKSPACE TEAM</span>
            <h1 id="technicians-title">Technicians</h1>
            <p>Manage technicians for this organization.</p>
          </div>
          <Button onClick={openCreateDialog} type="button"><Plus size={17} /> Add Technician</Button>
        </div>

        {notice && <p className="auth-notice auth-notice--success" role="status">{notice}</p>}
        {pageError && <p className="auth-notice auth-notice--error" role="alert">{pageError}</p>}

        {loading ? (
          <p className="services-state" role="status">Loading technicians...</p>
        ) : technicians.length === 0 ? (
          <div className="services-empty">
            <h2>No technicians yet</h2>
            <p>Add a technician to this organization.</p>
            <Button onClick={openCreateDialog} type="button"><Plus size={16} /> Add Technician</Button>
          </div>
        ) : (
          <div className="services-table-wrap">
            <table className="services-table technicians-table">
              <thead>
                <tr><th scope="col">Name</th><th scope="col">Email</th><th scope="col">Phone</th><th scope="col">Status</th><th scope="col"><span className="sr-only">Actions</span></th></tr>
              </thead>
              <tbody>
                {technicians.map((technician) => (
                  <tr key={technician.id}>
                    <td className="services-table__name">{technician.name}</td>
                    <td>{technician.email || 'Not provided'}</td>
                    <td>{technician.phone || 'Not provided'}</td>
                    <td><span className={`technician-status technician-status--${technician.status.toLowerCase()}`}>{technician.status}</span></td>
                    <td>
                      <div className="service-actions">
                        <button aria-label={`Edit ${technician.name}`} className="service-icon-button" onClick={() => openEditDialog(technician)} title="Edit technician" type="button"><Pencil size={16} /></button>
                        {technician.status === 'ACTIVE' && (
                          <button aria-label={`Deactivate ${technician.name}`} className="service-icon-button service-icon-button--delete" onClick={() => deactivateTechnician(technician)} title="Deactivate technician" type="button"><UserRoundMinus size={16} /></button>
                        )}
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
          <section aria-labelledby="technician-dialog-title" aria-modal="true" className="services-modal" role="dialog">
            <header className="services-modal__header">
              <div>
                <span className="section-kicker">TECHNICIAN DETAILS</span>
                <h2 id="technician-dialog-title">{editingTechnician ? 'Edit technician' : 'Add a technician'}</h2>
              </div>
              <button aria-label="Close form" className="service-icon-button" onClick={() => setDialogOpen(false)} type="button"><X size={18} /></button>
            </header>
            <TechnicianForm
              key={editingTechnician?.id || 'new-technician'}
              error={formError}
              initialValues={editingTechnician || emptyTechnician}
              onCancel={() => setDialogOpen(false)}
              onSubmit={saveTechnician}
              saving={saving}
            />
          </section>
        </div>
      )}
    </main>
  )
}