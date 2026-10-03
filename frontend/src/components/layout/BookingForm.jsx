import { useState } from 'react'
import Button from '../ui/Button'

export default function BookingForm({ initialValues, customers, services, technicians = [], appointmentMode = false, error, saving, onCancel, onSubmit }) {
  const [values, setValues] = useState({
    customerId: initialValues.customer_id || '',
    serviceId: initialValues.service_id || '',
    technicianId: initialValues.technician_id || '',
    bookingDate: initialValues.booking_date || '',
    startTime: initialValues.start_time || '',
    endTime: initialValues.end_time || '',
    notes: initialValues.notes || '',
    status: initialValues.status || 'BOOKED',
  })
  const [errors, setErrors] = useState({})

  function updateField(event) {
    const { name, value } = event.target
    setValues((current) => ({ ...current, [name]: value }))
    setErrors((current) => ({ ...current, [name]: '' }))
  }

  function submit(event) {
    event.preventDefault()
    const nextErrors = {}
    if (!customers.some((customer) => customer.id === values.customerId)) nextErrors.customerId = 'Select a customer.'
    if (!services.some((service) => service.id === values.serviceId)) nextErrors.serviceId = 'Select a service.'
    if (appointmentMode && values.technicianId && !technicians.some((technician) => technician.id === values.technicianId)) nextErrors.technicianId = 'Select an available technician.'
    if (!/^\d{4}-\d{2}-\d{2}$/.test(values.bookingDate)) nextErrors.bookingDate = 'Select a valid booking date.'
    if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(values.startTime)) nextErrors.startTime = 'Select a valid start time.'
    if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(values.endTime)) nextErrors.endTime = 'Select a valid end time.'
    else if (values.startTime && values.endTime <= values.startTime) nextErrors.endTime = 'End time must be after start time.'
    if (values.notes.length > 2000) nextErrors.notes = 'Use 2000 characters or fewer.'

    setErrors(nextErrors)
    if (Object.keys(nextErrors).length === 0) {
      const payload = {
        customer_id: values.customerId,
        service_id: values.serviceId,
        technician_id: appointmentMode ? values.technicianId : undefined,
        [appointmentMode ? 'appointment_date' : 'booking_date']: values.bookingDate,
        start_time: values.startTime,
        end_time: values.endTime,
        notes: values.notes.trim(),
      }
      if (appointmentMode && initialValues.id) payload.status = values.status
      onSubmit(payload)
    }
  }

  return (
    <form className="service-form" noValidate onSubmit={submit}>
      <SelectField error={errors.customerId} id="booking-customer" label="Customer" name="customerId" onChange={updateField} value={values.customerId}>
        <option value="">Select a customer</option>
        {customers.map((customer) => <option key={customer.id} value={customer.id}>{customer.name}{customer.email ? ` (${customer.email})` : ''}</option>)}
      </SelectField>
      <SelectField error={errors.serviceId} id="booking-service" label="Service" name="serviceId" onChange={updateField} value={values.serviceId}>
        <option value="">Select a service</option>
        {services.map((service) => <option key={service.id} value={service.id}>{service.name}</option>)}
      </SelectField>
      {appointmentMode && (
        <SelectField error={errors.technicianId} id="appointment-technician" label="Technician (optional)" name="technicianId" onChange={updateField} value={values.technicianId}>
          <option value="">No technician assigned</option>
          {technicians.filter((technician) => technician.status === 'ACTIVE' || technician.id === values.technicianId).map((technician) => (
            <option key={technician.id} value={technician.id}>{technician.name}{technician.status === 'INACTIVE' ? ' (inactive)' : ''}</option>
          ))}
        </SelectField>
      )}
      <div className="booking-form__row">
        <InputField error={errors.bookingDate} id="booking-date" label={appointmentMode ? 'Appointment date' : 'Date'} name="bookingDate" onChange={updateField} type="date" value={values.bookingDate} />
        <InputField error={errors.startTime} id="booking-start-time" label="Start time" name="startTime" onChange={updateField} type="time" value={values.startTime} />
        <InputField error={errors.endTime} id="booking-end-time" label="End time" name="endTime" onChange={updateField} type="time" value={values.endTime} />
      </div>
      <div className="form-field">
        <label htmlFor="booking-notes">Notes (optional)</label>
        <textarea
          aria-describedby={errors.notes ? 'booking-notes-error' : undefined}
          aria-invalid={Boolean(errors.notes)}
          className={`form-input services-description${errors.notes ? ' form-input--error' : ''}`}
          id="booking-notes"
          maxLength={2000}
          name="notes"
          onChange={updateField}
          rows={3}
          value={values.notes}
        />
        {errors.notes && <span className="form-field__error" id="booking-notes-error">{errors.notes}</span>}
      </div>
      {appointmentMode && initialValues.id && (
        <SelectField id="appointment-status" label="Status" name="status" onChange={updateField} value={values.status}>
          <option value="BOOKED">BOOKED</option>
          <option value="COMPLETED">COMPLETED</option>
          <option value="CANCELLED">CANCELLED</option>
        </SelectField>
      )}
      {error && <p className="auth-notice auth-notice--error" role="alert">{error}</p>}
      <footer className="services-modal__actions">
        <button className="button button--outline" onClick={onCancel} type="button">Cancel</button>
        <Button disabled={saving} type="submit">{saving ? 'Saving...' : initialValues.id ? appointmentMode ? 'Update appointment' : 'Save booking' : appointmentMode ? 'Create appointment' : 'Save booking'}</Button>
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

function InputField({ id, label, error, ...props }) {
  const errorId = error ? `${id}-error` : undefined
  return (
    <div className="form-field">
      <label htmlFor={id}>{label}</label>
      <input aria-describedby={errorId} aria-invalid={Boolean(error)} className={`form-input${error ? ' form-input--error' : ''}`} id={id} {...props} />
      {error && <span className="form-field__error" id={errorId}>{error}</span>}
    </div>
  )
}