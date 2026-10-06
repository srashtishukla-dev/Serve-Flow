-- PROPOSED, NOT APPLIED. Links a login user to exactly one customer or technician record
-- inside the same organization. Idempotent and additive.
BEGIN;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS customer_id UUID,
    ADD COLUMN IF NOT EXISTS technician_id UUID;

-- Composite FKs reuse customers_organization_id_id_unique (009) and
-- technicians_organization_id_id_unique (008), so a user can only link to a record
-- of its own organization. They are not enforced while the column is NULL.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_customer_same_organization_fkey' AND conrelid = 'users'::regclass) THEN
        ALTER TABLE users ADD CONSTRAINT users_customer_same_organization_fkey
            FOREIGN KEY (organization_id, customer_id) REFERENCES customers (organization_id, id) ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_technician_same_organization_fkey' AND conrelid = 'users'::regclass) THEN
        ALTER TABLE users ADD CONSTRAINT users_technician_same_organization_fkey
            FOREIGN KEY (organization_id, technician_id) REFERENCES technicians (organization_id, id) ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_role_identity_check' AND conrelid = 'users'::regclass) THEN
        ALTER TABLE users ADD CONSTRAINT users_role_identity_check CHECK (
            (role = 'ADMIN' AND customer_id IS NULL AND technician_id IS NULL)
            OR (role = 'CUSTOMER' AND customer_id IS NOT NULL AND technician_id IS NULL)
            OR (role = 'TECHNICIAN' AND technician_id IS NOT NULL AND customer_id IS NULL)
        );
    END IF;
END $$;

-- One login per identity record.
CREATE UNIQUE INDEX IF NOT EXISTS users_customer_id_unique_idx ON users (customer_id) WHERE customer_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS users_technician_id_unique_idx ON users (technician_id) WHERE technician_id IS NOT NULL;

COMMIT;
