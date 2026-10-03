BEGIN;

ALTER TABLE bookings
    ADD COLUMN IF NOT EXISTS technician_id UUID;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'technicians_organization_id_id_unique'
          AND conrelid = 'technicians'::regclass
    ) THEN
        ALTER TABLE technicians
            ADD CONSTRAINT technicians_organization_id_id_unique UNIQUE (organization_id, id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'bookings_technician_same_organization_fkey'
          AND conrelid = 'bookings'::regclass
    ) THEN
        ALTER TABLE bookings
            ADD CONSTRAINT bookings_technician_same_organization_fkey
            FOREIGN KEY (organization_id, technician_id)
            REFERENCES technicians (organization_id, id);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS bookings_technician_schedule_idx
    ON bookings (organization_id, technician_id, booking_date, start_time)
    WHERE technician_id IS NOT NULL AND status = 'BOOKED';

COMMIT;