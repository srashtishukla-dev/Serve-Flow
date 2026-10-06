BEGIN;

CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL CHECK (length(btrim(type)) > 0),
    title VARCHAR(200) NOT NULL CHECK (length(btrim(title)) > 0),
    message TEXT NOT NULL CHECK (length(btrim(message)) > 0 AND length(message) <= 2000),
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS notifications_organization_created_idx
    ON notifications (organization_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS notifications_user_created_idx
    ON notifications (user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS notifications_unread_user_created_idx
    ON notifications (organization_id, user_id, created_at DESC, id DESC)
    WHERE is_read = FALSE;

COMMIT;