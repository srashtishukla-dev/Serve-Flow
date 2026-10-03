CREATE TABLE technicians (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    name VARCHAR(150) NOT NULL,
    email VARCHAR(255),
    phone VARCHAR(30),
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_technicians_organization
        FOREIGN KEY (organization_id)
        REFERENCES organizations(id),

    CONSTRAINT technicians_status_check
        CHECK (status IN ('ACTIVE', 'INACTIVE'))
);

CREATE INDEX idx_technicians_organization_id
    ON technicians(organization_id);