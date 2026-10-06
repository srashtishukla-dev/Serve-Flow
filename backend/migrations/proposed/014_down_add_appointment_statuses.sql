-- PROPOSED ROLLBACK, NOT APPLIED. Lossy: ASSIGNED and IN_PROGRESS become BOOKED,
-- NO_SHOW becomes CANCELLED.
BEGIN;
UPDATE bookings SET status = 'BOOKED' WHERE status IN ('ASSIGNED', 'IN_PROGRESS');
UPDATE bookings SET status = 'CANCELLED' WHERE status = 'NO_SHOW';
ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_status_check;
ALTER TABLE bookings ADD CONSTRAINT bookings_status_check CHECK (status IN ('BOOKED', 'CANCELLED', 'COMPLETED'));
DROP INDEX IF EXISTS bookings_technician_schedule_idx;
CREATE INDEX bookings_technician_schedule_idx ON bookings (organization_id, technician_id, booking_date, start_time)
    WHERE technician_id IS NOT NULL AND status = 'BOOKED';
DROP INDEX IF EXISTS bookings_organization_id_date_start_idx;
CREATE INDEX bookings_organization_id_date_start_idx ON bookings (organization_id, booking_date, start_time)
    WHERE status = 'BOOKED';
COMMIT;
