export const statusLabels = {
  BOOKED: 'Scheduled',
  ASSIGNED: 'Assigned',
  IN_PROGRESS: 'In Progress',
  COMPLETED: 'Completed',
  CANCELLED: 'Cancelled',
  NO_SHOW: 'No-show',
}

export const statusOptions = Object.entries(statusLabels)

export function statusLabel(status) {
  return statusLabels[status] || status
}

export function statusClass(status) {
  return `booking-status booking-status--${String(status).toLowerCase().replaceAll('_', '-')}`
}

// Mirrors the backend transition rules; the backend remains the authority.
export function availableActions(role, appointment) {
  const { status } = appointment
  const actions = []
  const open = status === 'BOOKED' || status === 'ASSIGNED'
  if (role === 'ADMIN') {
    if (open) actions.push({ status: 'IN_PROGRESS', label: 'Start' })
    if (open || status === 'IN_PROGRESS') actions.push({ status: 'COMPLETED', label: 'Complete' })
    if (open) actions.push({ status: 'NO_SHOW', label: 'No-show' })
    if (open || status === 'IN_PROGRESS') actions.push({ status: 'CANCELLED', label: 'Cancel', danger: true })
  } else if (role === 'TECHNICIAN') {
    if (status === 'ASSIGNED') actions.push({ status: 'IN_PROGRESS', label: 'Start' })
    if (status === 'ASSIGNED' || status === 'IN_PROGRESS') actions.push({ status: 'COMPLETED', label: 'Complete' })
  } else if (role === 'CUSTOMER') {
    if (open) actions.push({ status: 'CANCELLED', label: 'Cancel', danger: true })
  }
  return actions
}

export function canEdit(role, appointment) {
  return (role === 'ADMIN' || role === 'CUSTOMER') && (appointment.status === 'BOOKED' || appointment.status === 'ASSIGNED')
}
