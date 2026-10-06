BEGIN;

CREATE TABLE IF NOT EXISTS customers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(120) NOT NULL CHECK (length(btrim(name)) > 0),
    email VARCHAR(254),
    phone VARCHAR(50),
    notes TEXT NOT NULL DEFAULT '' CHECK (length(notes) <= 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (email IS NULL OR length(btrim(email)) > 0),
    CHECK (phone IS NULL OR length(phone) <= 50)
);

CREATE INDEX IF NOT EXISTS customers_organization_id_created_at_idx
    ON customers (organization_id, created_at DESC, id);

COMMIT;