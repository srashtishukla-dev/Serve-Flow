BEGIN;

CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    slug TEXT NOT NULL UNIQUE CHECK (length(btrim(slug)) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE users ADD COLUMN IF NOT EXISTS organization_id UUID;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'users_organization_id_fkey'
          AND conrelid = 'users'::regclass
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT users_organization_id_fkey
            FOREIGN KEY (organization_id) REFERENCES organizations(id);
    END IF;
END $$;

INSERT INTO organizations (name, slug)
SELECT COALESCE(NULLIF(btrim(name), ''), 'Personal') || ' Workspace',
       'legacy-' || replace(id::text, '-', '')
FROM users
WHERE organization_id IS NULL
ON CONFLICT (slug) DO NOTHING;

UPDATE users AS existing_user
SET organization_id = organization.id
FROM organizations AS organization
WHERE existing_user.organization_id IS NULL
  AND organization.slug = 'legacy-' || replace(existing_user.id::text, '-', '');

ALTER TABLE users ALTER COLUMN organization_id SET NOT NULL;
CREATE INDEX IF NOT EXISTS users_organization_id_idx ON users(organization_id);

COMMIT;