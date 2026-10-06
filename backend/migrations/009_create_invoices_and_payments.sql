BEGIN;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'customers_organization_id_id_unique'
          AND conrelid = 'customers'::regclass
    ) THEN
        ALTER TABLE customers
            ADD CONSTRAINT customers_organization_id_id_unique UNIQUE (organization_id, id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'bookings_organization_id_id_unique'
          AND conrelid = 'bookings'::regclass
    ) THEN
        ALTER TABLE bookings
            ADD CONSTRAINT bookings_organization_id_id_unique UNIQUE (organization_id, id);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    customer_id UUID NOT NULL,
    appointment_id UUID,
    invoice_number VARCHAR(40) NOT NULL,
    status TEXT NOT NULL DEFAULT 'ISSUED'
        CHECK (status IN ('DRAFT', 'ISSUED', 'PARTIALLY_PAID', 'PAID', 'CANCELLED')),
    issue_date DATE NOT NULL,
    due_date DATE NOT NULL,
    subtotal NUMERIC(12, 2) NOT NULL CHECK (subtotal >= 0),
    tax NUMERIC(12, 2) NOT NULL DEFAULT 0 CHECK (tax >= 0),
    total NUMERIC(12, 2) NOT NULL CHECK (total >= 0 AND total = subtotal + tax),
    notes TEXT NOT NULL DEFAULT '' CHECK (length(notes) <= 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT invoices_due_date_check CHECK (due_date >= issue_date),
    CONSTRAINT invoices_organization_number_unique UNIQUE (organization_id, invoice_number),
    CONSTRAINT invoices_organization_id_id_unique UNIQUE (organization_id, id),
    CONSTRAINT invoices_customer_same_organization_fkey
        FOREIGN KEY (organization_id, customer_id)
        REFERENCES customers (organization_id, id),
    CONSTRAINT invoices_appointment_same_organization_fkey
        FOREIGN KEY (organization_id, appointment_id)
        REFERENCES bookings (organization_id, id)
);

CREATE INDEX IF NOT EXISTS invoices_organization_id_issue_date_idx
    ON invoices (organization_id, issue_date DESC, invoice_number);

CREATE INDEX IF NOT EXISTS invoices_organization_id_customer_idx
    ON invoices (organization_id, customer_id, issue_date DESC);

CREATE TABLE IF NOT EXISTS invoice_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    description VARCHAR(200) NOT NULL CHECK (length(btrim(description)) > 0),
    quantity INTEGER NOT NULL CHECK (quantity BETWEEN 1 AND 1000000),
    unit_price NUMERIC(12, 2) NOT NULL CHECK (unit_price >= 0),
    amount NUMERIC(12, 2) NOT NULL CHECK (amount >= 0 AND amount = quantity * unit_price),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS invoice_items_invoice_id_idx
    ON invoice_items (invoice_id, created_at, id);

CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    invoice_id UUID NOT NULL,
    amount NUMERIC(12, 2) NOT NULL CHECK (amount > 0),
    payment_date DATE NOT NULL,
    payment_method TEXT NOT NULL
        CHECK (payment_method IN ('CASH', 'CARD', 'UPI', 'BANK_TRANSFER', 'OTHER')),
    reference VARCHAR(120) NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '' CHECK (length(notes) <= 1000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT payments_invoice_same_organization_fkey
        FOREIGN KEY (organization_id, invoice_id)
        REFERENCES invoices (organization_id, id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS payments_organization_invoice_date_idx
    ON payments (organization_id, invoice_id, payment_date, id);

COMMIT;