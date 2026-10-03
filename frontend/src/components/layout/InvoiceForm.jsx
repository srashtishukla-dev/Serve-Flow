import { useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import Button from '../ui/Button'
import Input from '../ui/Input'

const maxMoneyCents = 999999999999n
const moneyPattern = /^(\d+)(?:\.(\d{1,2}))?$/

export default function InvoiceForm({ initialValues, customers, appointments, error, saving, onCancel, onSubmit }) {
  const [values, setValues] = useState({
    customerId: initialValues.customer_id || '',
    appointmentId: initialValues.appointment_id || '',
    issueDate: initialValues.issue_date || localDate(),
    dueDate: initialValues.due_date || localDate(14),
    tax: String(initialValues.tax ?? '0.00'),
    notes: initialValues.notes || '',
  })
  const [items, setItems] = useState(initialValues.items?.length
    ? initialValues.items.map((item) => ({
      description: item.description || '',
      quantity: String(item.quantity || 1),
      unitPrice: String(item.unit_price ?? ''),
    }))
    : [{ description: '', quantity: '1', unitPrice: '' }])
  const [errors, setErrors] = useState({})

  const customerAppointments = appointments.filter((appointment) => appointment.customer_id === values.customerId)
  const subtotalCents = calculateSubtotal(items)
  const taxCents = parseCents(values.tax.trim() || '0')
  const totalCents = subtotalCents === null || taxCents === null || subtotalCents + taxCents > maxMoneyCents
    ? null
    : subtotalCents + taxCents

  function updateField(event) {
    const { name, value } = event.target
    setValues((current) => ({ ...current, [name]: value }))
    setErrors((current) => ({ ...current, [name]: '' }))
  }

  function updateItem(index, field, value) {
    setItems((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, [field]: value } : item))
    setErrors((current) => ({ ...current, [`item-${index}-${field}`]: '' }))
  }

  function submit(event) {
    event.preventDefault()
    const nextErrors = {}
    if (!customers.some((customer) => customer.id === values.customerId)) nextErrors.customerId = 'Select a customer.'
    if (values.appointmentId && !customerAppointments.some((appointment) => appointment.id === values.appointmentId)) {
      nextErrors.appointmentId = 'Select an appointment for this customer.'
    }
    if (!isDate(values.issueDate)) nextErrors.issueDate = 'Select a valid issue date.'
    if (!isDate(values.dueDate)) nextErrors.dueDate = 'Select a valid due date.'
    else if (values.issueDate && values.dueDate < values.issueDate) nextErrors.dueDate = 'Due date cannot be before issue date.'
    if (items.length === 0) nextErrors.items = 'Add at least one invoice item.'
    if (items.length > 100) nextErrors.items = 'An invoice may contain at most 100 items.'

    items.forEach((item, index) => {
      if (!item.description.trim()) nextErrors[`item-${index}-description`] = 'Enter a description.'
      else if (item.description.trim().length > 200) nextErrors[`item-${index}-description`] = 'Use 200 characters or fewer.'
      if (!/^\d+$/.test(item.quantity) || !Number.isSafeInteger(Number(item.quantity)) || Number(item.quantity) < 1 || Number(item.quantity) > 1000000) {
        nextErrors[`item-${index}-quantity`] = 'Enter a whole quantity from 1 to 1,000,000.'
      }
      if (parseCents(item.unitPrice.trim()) === null) nextErrors[`item-${index}-unitPrice`] = 'Enter a non-negative amount with up to two decimals.'
    })
    if (values.tax.trim() && parseCents(values.tax.trim()) === null) nextErrors.tax = 'Enter a non-negative amount with up to two decimals.'
    if (values.notes.length > 2000) nextErrors.notes = 'Use 2000 characters or fewer.'
    if (subtotalCents === null || taxCents === null || subtotalCents + taxCents > maxMoneyCents) {
      nextErrors.items = 'Invoice total exceeds the supported amount.'
    }

    setErrors(nextErrors)
    if (Object.keys(nextErrors).length !== 0) return

    onSubmit({
      customer_id: values.customerId,
      ...(values.appointmentId ? { appointment_id: values.appointmentId } : {}),
      issue_date: values.issueDate,
      due_date: values.dueDate,
      items: items.map((item) => ({
        description: item.description.trim(),
        quantity: Number(item.quantity),
        unit_price: item.unitPrice.trim(),
      })),
      tax: values.tax.trim() || '0.00',
      notes: values.notes.trim(),
    })
  }

  return (
    <form className="service-form invoice-form" noValidate onSubmit={submit}>
      <SelectField error={errors.customerId} id="invoice-customer" label="Customer" name="customerId" onChange={(event) => {
        updateField(event)
        setValues((current) => ({ ...current, appointmentId: '' }))
      }} value={values.customerId}>
        <option value="">Select a customer</option>
        {customers.map((customer) => <option key={customer.id} value={customer.id}>{customer.name}{customer.email ? ` (${customer.email})` : ''}</option>)}
      </SelectField>
      <SelectField error={errors.appointmentId} id="invoice-appointment" label="Appointment (optional)" name="appointmentId" onChange={updateField} value={values.appointmentId}>
        <option value="">No appointment linked</option>
        {customerAppointments.map((appointment) => (
          <option key={appointment.id} value={appointment.id}>{appointment.appointment_date} · {appointment.start_time} · {appointment.status}</option>
        ))}
      </SelectField>
      <div className="invoice-form__dates">
        <Input error={errors.issueDate} id="invoice-issue-date" label="Issue date" name="issueDate" onChange={updateField} type="date" value={values.issueDate} />
        <Input error={errors.dueDate} id="invoice-due-date" label="Due date" name="dueDate" onChange={updateField} type="date" value={values.dueDate} />
      </div>

      <section aria-labelledby="invoice-items-title" className="invoice-form__items">
        <div className="invoice-form__items-heading">
          <h3 id="invoice-items-title">Line items</h3>
          <button className="button button--outline button--small" disabled={items.length >= 100} onClick={() => setItems((current) => [...current, { description: '', quantity: '1', unitPrice: '' }])} type="button"><Plus size={15} /> Add item</button>
        </div>
        {errors.items && <p className="form-field__error" role="alert">{errors.items}</p>}
        {items.map((item, index) => (
          <div className="invoice-line-item" key={index}>
            <Input error={errors[`item-${index}-description`]} id={`invoice-item-description-${index}`} label="Description" maxLength={200} onChange={(event) => updateItem(index, 'description', event.target.value)} value={item.description} />
            <Input error={errors[`item-${index}-quantity`]} id={`invoice-item-quantity-${index}`} inputMode="numeric" label="Qty" max="1000000" min="1" onChange={(event) => updateItem(index, 'quantity', event.target.value)} step="1" type="number" value={item.quantity} />
            <Input error={errors[`item-${index}-unitPrice`]} id={`invoice-item-price-${index}`} inputMode="decimal" label="Unit price" onChange={(event) => updateItem(index, 'unitPrice', event.target.value)} placeholder="0.00" type="text" value={item.unitPrice} />
            <div className="invoice-line-item__amount"><span>Amount</span><strong>{lineAmount(item)}</strong></div>
            <button aria-label={`Remove item ${index + 1}`} className="service-icon-button service-icon-button--delete" disabled={items.length === 1} onClick={() => setItems((current) => current.filter((_, itemIndex) => itemIndex !== index))} title="Remove item" type="button"><Trash2 size={16} /></button>
          </div>
        ))}
      </section>

      <Input error={errors.tax} id="invoice-tax" inputMode="decimal" label="Tax amount" name="tax" onChange={updateField} placeholder="0.00" type="text" value={values.tax} />
      <div className="invoice-totals" aria-live="polite">
        <div><span>Subtotal</span><strong>{formatCents(subtotalCents)}</strong></div>
        <div><span>Tax</span><strong>{formatCents(taxCents)}</strong></div>
        <div className="invoice-totals__total"><span>Total</span><strong>{formatCents(totalCents)}</strong></div>
      </div>
      <div className="form-field">
        <label htmlFor="invoice-notes">Notes (optional)</label>
        <textarea aria-describedby={errors.notes ? 'invoice-notes-error' : undefined} aria-invalid={Boolean(errors.notes)} className={`form-input services-description${errors.notes ? ' form-input--error' : ''}`} id="invoice-notes" maxLength={2000} name="notes" onChange={updateField} rows={3} value={values.notes} />
        {errors.notes && <span className="form-field__error" id="invoice-notes-error">{errors.notes}</span>}
      </div>
      {error && <p className="auth-notice auth-notice--error" role="alert">{error}</p>}
      <footer className="services-modal__actions">
        <button className="button button--outline" onClick={onCancel} type="button">Cancel</button>
        <Button disabled={saving} type="submit">{saving ? 'Saving...' : initialValues.id ? 'Update invoice' : 'Create invoice'}</Button>
      </footer>
    </form>
  )
}

function SelectField({ id, label, error, children, ...props }) {
  const errorId = error ? `${id}-error` : undefined
  return (
    <div className="form-field">
      <label htmlFor={id}>{label}</label>
      <select aria-describedby={errorId} aria-invalid={Boolean(error)} className={`form-input${error ? ' form-input--error' : ''}`} id={id} {...props}>
        {children}
      </select>
      {error && <span className="form-field__error" id={errorId}>{error}</span>}
    </div>
  )
}

function localDate(daysFromNow = 0) {
  const date = new Date()
  date.setDate(date.getDate() + daysFromNow)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function isDate(value) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
  const [year, month, day] = value.split('-').map(Number)
  const date = new Date(year, month - 1, day)
  return date.getFullYear() === year && date.getMonth() === month - 1 && date.getDate() === day
}

function parseCents(value) {
  const match = moneyPattern.exec(value)
  if (!match) return null
  const cents = BigInt(match[1]) * 100n + BigInt((match[2] || '').padEnd(2, '0') || '0')
  return cents <= maxMoneyCents ? cents : null
}

function calculateSubtotal(items) {
  let subtotal = 0n
  for (const item of items) {
    const unitPrice = parseCents(item.unitPrice.trim())
    const quantity = Number(item.quantity)
    if (unitPrice === null || !Number.isSafeInteger(quantity) || quantity < 1 || quantity > 1000000) return null
    subtotal += unitPrice * BigInt(quantity)
    if (subtotal > maxMoneyCents) return null
  }
  return subtotal
}

function lineAmount(item) {
  const unitPrice = parseCents(item.unitPrice.trim())
  const quantity = Number(item.quantity)
  if (unitPrice === null || !Number.isSafeInteger(quantity) || quantity < 1 || quantity > 1000000) return 'INR --'
  const amount = unitPrice * BigInt(quantity)
  return amount <= maxMoneyCents ? formatCents(amount) : 'INR --'
}

function formatCents(cents) {
  if (cents === null) return 'INR --'
  return `INR ${cents / 100n}.${String(cents % 100n).padStart(2, '0')}`
}