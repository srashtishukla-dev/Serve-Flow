BEGIN;

-- Existing users each own their own workspace, so they are administrators of it.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS role VARCHAR(20) NOT NULL DEFAULT 'ADMIN';

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_role_check;

ALTER TABLE users
    ADD CONSTRAINT users_role_check CHECK (role IN ('ADMIN', 'TECHNICIAN', 'CUSTOMER'));

COMMIT;
