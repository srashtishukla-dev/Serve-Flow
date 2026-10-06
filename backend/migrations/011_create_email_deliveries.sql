BEGIN;

CREATE UNIQUE INDEX IF NOT EXISTS users_id_organization_id_uidx
    ON users (id, organization_id);

CREATE TABLE IF NOT EXISTS email_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient TEXT NOT NULL CHECK (length(btrim(recipient)) BETWEEN 3 AND 254),
    event_type VARCHAR(50) NOT NULL CHECK (length(btrim(event_type)) BETWEEN 1 AND 50),
    title VARCHAR(200) NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 200),
    content TEXT NOT NULL CHECK (length(btrim(content)) BETWEEN 1 AND 2000),
    status VARCHAR(16) NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'processing', 'sent', 'failed')),
    attempts SMALLINT NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 3),
    last_error VARCHAR(120),
    available_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sent_at TIMESTAMPTZ,
    CONSTRAINT email_deliveries_user_org_fk
        FOREIGN KEY (user_id, organization_id)
        REFERENCES users(id, organization_id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS email_deliveries_queued_idx
    ON email_deliveries (available_at, created_at, id)
    WHERE status = 'queued';

CREATE INDEX IF NOT EXISTS email_deliveries_processing_idx
    ON email_deliveries (updated_at)
    WHERE status = 'processing';

COMMIT;
