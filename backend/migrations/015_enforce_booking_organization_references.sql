BEGIN;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'services_organization_id_id_unique'
          AND conrelid = 'services'::regclass
    ) THEN
        ALTER TABLE services
            ADD CONSTRAINT services_organization_id_id_unique UNIQUE (organization_id, id);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'bookings_customer_same_organization_fkey'
          AND conrelid = 'bookings'::regclass
    ) THEN
        ALTER TABLE bookings
            ADD CONSTRAINT bookings_customer_same_organization_fkey
            FOREIGN KEY (organization_id, customer_id)
            REFERENCES customers (organization_id, id);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'bookings_service_same_organization_fkey'
          AND conrelid = 'bookings'::regclass
    ) THEN
        ALTER TABLE bookings
            ADD CONSTRAINT bookings_service_same_organization_fkey
            FOREIGN KEY (organization_id, service_id)
            REFERENCES services (organization_id, id);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS bookings_organization_customer_idx
    ON bookings (organization_id, customer_id);

CREATE INDEX IF NOT EXISTS bookings_organization_service_idx
    ON bookings (organization_id, service_id);

COMMIT;
