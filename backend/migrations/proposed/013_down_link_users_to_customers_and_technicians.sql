-- PROPOSED ROLLBACK, NOT APPLIED. Fails if any CUSTOMER/TECHNICIAN users exist
-- (they would lose their identity link); remove or convert them first.
BEGIN;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM users WHERE role <> 'ADMIN') THEN
        RAISE EXCEPTION 'Rollback blocked: non-ADMIN users exist';
    END IF;
END $$;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_identity_check;
DROP INDEX IF EXISTS users_customer_id_unique_idx;
DROP INDEX IF EXISTS users_technician_id_unique_idx;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_customer_same_organization_fkey;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_technician_same_organization_fkey;
ALTER TABLE users DROP COLUMN IF EXISTS customer_id, DROP COLUMN IF EXISTS technician_id;
COMMIT;
