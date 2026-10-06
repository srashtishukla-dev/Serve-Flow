import { describe, expect, it } from 'vitest'
import { availableActions, statusLabel, statusClass } from '../utils/appointmentStatus'

const labels = (role, status) => availableActions(role, { status }).map((a) => a.status)

describe('appointment status helpers', () => {
  it('uses backend status values for labels', () => {
    expect(statusLabel('BOOKED')).toBe('Scheduled')
    expect(statusLabel('IN_PROGRESS')).toBe('In Progress')
    expect(statusLabel('NO_SHOW')).toBe('No-show')
    expect(statusClass('IN_PROGRESS')).toContain('in-progress')
  })
  it('limits actions per role', () => {
    expect(labels('CUSTOMER', 'BOOKED')).toEqual(['CANCELLED'])
    expect(labels('TECHNICIAN', 'ASSIGNED')).toEqual(['IN_PROGRESS', 'COMPLETED'])
    expect(labels('ADMIN', 'ASSIGNED')).toContain('NO_SHOW')
  })
  it('offers nothing for terminal statuses', () => {
    for (const role of ['ADMIN', 'CUSTOMER', 'TECHNICIAN']) {
      expect(labels(role, 'COMPLETED')).toEqual([])
      expect(labels(role, 'CANCELLED')).toEqual([])
    }
  })
})
