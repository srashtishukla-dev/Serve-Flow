BEGIN;

ALTER TABLE technicians
    ALTER COLUMN id SET DEFAULT gen_random_uuid();

ALTER TABLE technicians
    DROP CONSTRAINT IF EXISTS fk_technicians_organization;

ALTER TABLE technicians
    ADD CONSTRAINT fk_technicians_organization
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_technicians_organization_id;

CREATE INDEX IF NOT EXISTS technicians_organization_id_created_at_idx
    ON technicians (organization_id, created_at DESC, id);

COMMIT;