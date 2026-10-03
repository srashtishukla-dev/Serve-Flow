import { useState } from 'react'
import Button from '../ui/Button'
import Input from '../ui/Input'

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export default function TechnicianForm({ initialValues, error, saving, onCancel, onSubmit }) {
  const [values, setValues] = useState({
    name: initialValues.name || '',
    email: initialValues.email || '',
    phone: initialValues.phone || '',
  })
  const [errors, setErrors] = useState({})

  function updateField(event) {
    const { name, value } = event.target
    setValues((current) => ({ ...current, [name]: value }))
    setErrors((current) => ({ ...current, [name]: '' }))
  }

  function submit(event) {
    event.preventDefault()
    const name = values.name.trim()
    const email = values.email.trim()
    const phone = values.phone.trim()
    const nextErrors = {}

    if (!name) nextErrors.name = 'Enter a technician name.'
    else if (name.length > 120) nextErrors.name = 'Use 120 characters or fewer.'
    if (email && !emailPattern.test(email)) nextErrors.email = 'Enter a valid email address.'
    else if (email.length > 254) nextErrors.email = 'Use 254 characters or fewer.'
    if (phone.length > 50) nextErrors.phone = 'Use 50 characters or fewer.'

    setErrors(nextErrors)
    if (Object.keys(nextErrors).length === 0) onSubmit({ name, email, phone })
  }

  return (
    <form className="service-form" noValidate onSubmit={submit}>
      <Input autoFocus error={errors.name} id="technician-name" label="Name" maxLength={120} name="name" onChange={updateField} value={values.name} />
      <Input autoComplete="email" error={errors.email} id="technician-email" label="Email (optional)" maxLength={254} name="email" onChange={updateField} type="email" value={values.email} />
      <Input autoComplete="tel" error={errors.phone} id="technician-phone" label="Phone (optional)" maxLength={50} name="phone" onChange={updateField} type="tel" value={values.phone} />
      {error && <p className="auth-notice auth-notice--error" role="alert">{error}</p>}
      <footer className="services-modal__actions">
        <button className="button button--outline" onClick={onCancel} type="button">Cancel</button>
        <Button disabled={saving} type="submit">{saving ? 'Saving...' : initialValues.id ? 'Update Technician' : 'Add Technician'}</Button>
      </footer>
    </form>
  )
}