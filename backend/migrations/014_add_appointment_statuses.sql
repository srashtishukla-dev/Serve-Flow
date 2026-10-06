-- PROPOSED, NOT APPLIED. Adds ASSIGNED, IN_PROGRESS and NO_SHOW to bookings.status
-- (upper-case, underscore style like PARTIALLY_PAID). Existing rows are untouched.
BEGIN;

ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_status_check;
ALTER TABLE bookings ADD CONSTRAINT bookings_status_check
    CHECK (status IN ('BOOKED', 'ASSIGNED', 'IN_PROGRESS', 'COMPLETED', 'CANCELLED', 'NO_SHOW'));

-- Active statuses keep occupying the technician's time slot; terminal ones do not.
DROP INDEX IF EXISTS bookings_technician_schedule_idx;
CREATE INDEX bookings_technician_schedule_idx
    ON bookings (organization_id, technician_id, booking_date, start_time)
    WHERE technician_id IS NOT NULL AND status IN ('BOOKED', 'ASSIGNED', 'IN_PROGRESS');

DROP INDEX IF EXISTS bookings_organization_id_date_start_idx;
CREATE INDEX bookings_organization_id_date_start_idx
    ON bookings (organization_id, booking_date, start_time)
    WHERE status IN ('BOOKED', 'ASSIGNED', 'IN_PROGRESS');

COMMIT;
