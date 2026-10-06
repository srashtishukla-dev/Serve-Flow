BEGIN;

CREATE TABLE IF NOT EXISTS services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(120) NOT NULL CHECK (length(btrim(name)) > 0),
    description TEXT NOT NULL DEFAULT '' CHECK (length(description) <= 2000),
    duration_minutes INTEGER NOT NULL CHECK (duration_minutes > 0),
    price NUMERIC(12, 2) NOT NULL CHECK (price >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS services_organization_id_created_at_idx
    ON services (organization_id, created_at DESC, id);

COMMIT;