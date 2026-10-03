package invoices

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/serveflow/serveflow/backend/internal/pagination"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (repository *Repository) Create(ctx context.Context, organizationID string, input Input, totals Totals) (Invoice, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Invoice{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockOrganization(ctx, tx, organizationID); err != nil {
		return Invoice{}, err
	}
	if err := relatedRecordsBelongToOrganization(ctx, tx, organizationID, input.CustomerID, input.AppointmentID); err != nil {
		return Invoice{}, err
	}
	var invoiceCount int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM invoices WHERE organization_id = $1::uuid`, organizationID).Scan(&invoiceCount); err != nil {
		return Invoice{}, err
	}
	invoiceNumber := fmt.Sprintf("INV-%06d", invoiceCount+1)

	var item Invoice
	item.CustomerID = input.CustomerID
	item.AppointmentID = input.AppointmentID
	item.InvoiceNumber = invoiceNumber
	item.Status = StatusIssued
	item.IssueDate = input.IssueDate
	item.DueDate = input.DueDate
	item.Subtotal = jsonMoney(totals.Subtotal)
	item.Tax = jsonMoney(totals.Tax)
	item.Total = jsonMoney(totals.Total)
	item.AmountPaid = jsonMoney("0.00")
	item.BalanceDue = item.Total
	item.Notes = input.Notes
	err = tx.QueryRow(ctx, `
		INSERT INTO invoices (organization_id, customer_id, appointment_id, invoice_number, status, issue_date, due_date, subtotal, tax, total, notes)
		VALUES ($1::uuid, $2::uuid, NULLIF($3, '')::uuid, $4, $5, $6::date, $7::date, $8::numeric, $9::numeric, $10::numeric, $11)
		RETURNING id::text, created_at, updated_at
	`, organizationID, input.CustomerID, input.AppointmentID, invoiceNumber, item.Status, input.IssueDate, input.DueDate, totals.Subtotal, totals.Tax, totals.Total, input.Notes).Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Invoice{}, err
	}
	for index, line := range input.Items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO invoice_items (invoice_id, description, quantity, unit_price, amount)
			VALUES ($1::uuid, $2, $3, $4::numeric, $5::numeric)
		`, item.ID, line.Description, line.Quantity, line.UnitPrice, totals.Items[index].Amount); err != nil {
			return Invoice{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Invoice{}, err
	}
	return item, nil
}

func (repository *Repository) List(ctx context.Context, organizationID string) ([]Invoice, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT i.id::text, i.invoice_number, i.customer_id::text, c.name, COALESCE(i.appointment_id::text, ''),
		       i.status, i.issue_date::text, i.due_date::text, i.subtotal::text, i.tax::text, i.total::text,
		       COALESCE(SUM(p.amount), 0)::text, i.notes, i.created_at, i.updated_at
		FROM invoices AS i
		JOIN customers AS c ON c.id = i.customer_id AND c.organization_id = i.organization_id
		LEFT JOIN payments AS p ON p.invoice_id = i.id AND p.organization_id = i.organization_id
		WHERE i.organization_id = $1::uuid
		GROUP BY i.id, c.name
		ORDER BY i.issue_date DESC, i.invoice_number DESC
	`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Invoice, 0)
	for rows.Next() {
		item, err := scanInvoice(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *Repository) ListPage(ctx context.Context, organizationID string, query pagination.Query) ([]Invoice, pagination.Metadata, error) {
	var total int
	if err := repository.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM invoices AS i
		JOIN customers AS c ON c.id = i.customer_id AND c.organization_id = i.organization_id
		WHERE i.organization_id = $1::uuid
		  AND ($2 = '' OR i.status = $2)
		  AND ($3 = '' OR i.invoice_number ILIKE '%' || $3 || '%' OR c.name ILIKE '%' || $3 || '%')
	`, organizationID, query.Status, query.Search).Scan(&total); err != nil {
		return nil, pagination.Metadata{}, err
	}
	orderBy := pagination.OrderBy(query, map[string]string{
		"issue_date": "i.issue_date", "due_date": "i.due_date", "total": "i.total",
		"status": "i.status", "created_at": "i.created_at",
	})
	rows, err := repository.pool.Query(ctx, `
		SELECT i.id::text, i.invoice_number, i.customer_id::text, c.name, COALESCE(i.appointment_id::text, ''),
		       i.status, i.issue_date::text, i.due_date::text, i.subtotal::text, i.tax::text, i.total::text,
		       COALESCE(SUM(p.amount), 0)::text, i.notes, i.created_at, i.updated_at
		FROM invoices AS i
		JOIN customers AS c ON c.id = i.customer_id AND c.organization_id = i.organization_id
		LEFT JOIN payments AS p ON p.invoice_id = i.id AND p.organization_id = i.organization_id
		WHERE i.organization_id = $1::uuid
		  AND ($2 = '' OR i.status = $2)
		  AND ($3 = '' OR i.invoice_number ILIKE '%' || $3 || '%' OR c.name ILIKE '%' || $3 || '%')
		GROUP BY i.id, c.name
		ORDER BY `+orderBy+`, i.invoice_number ASC
		LIMIT $4 OFFSET $5
	`, organizationID, query.Status, query.Search, query.Limit, query.Offset)
	if err != nil {
		return nil, pagination.Metadata{}, err
	}
	defer rows.Close()
	items := make([]Invoice, 0)
	for rows.Next() {
		item, err := scanInvoice(rows)
		if err != nil {
			return nil, pagination.Metadata{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, pagination.Metadata{}, err
	}
	return items, pagination.NewMetadata(query, total), nil
}

func (repository *Repository) Get(ctx context.Context, organizationID, invoiceID string) (Details, error) {
	item, err := repository.findInvoice(ctx, repository.pool, organizationID, invoiceID)
	if err != nil {
		return Details{}, err
	}

	details := Details{Invoice: item, Items: make([]Item, 0), Payments: make([]Payment, 0)}
	err = repository.pool.QueryRow(ctx, `
		SELECT id::text, name, COALESCE(email, ''), COALESCE(phone, ''), notes
		FROM customers
		WHERE id = $1::uuid AND organization_id = $2::uuid
	`, item.CustomerID, organizationID).Scan(&details.Customer.ID, &details.Customer.Name, &details.Customer.Email, &details.Customer.Phone, &details.Customer.Notes)
	if err != nil {
		return Details{}, err
	}

	itemRows, err := repository.pool.Query(ctx, `
		SELECT line.id::text, line.description, line.quantity, line.unit_price::text, line.amount::text
		FROM invoice_items AS line
		JOIN invoices AS invoice ON invoice.id = line.invoice_id
		WHERE line.invoice_id = $1::uuid AND invoice.organization_id = $2::uuid
		ORDER BY line.created_at, line.id
	`, invoiceID, organizationID)
	if err != nil {
		return Details{}, err
	}
	for itemRows.Next() {
		var line Item
		var unitPrice, amount string
		if err := itemRows.Scan(&line.ID, &line.Description, &line.Quantity, &unitPrice, &amount); err != nil {
			itemRows.Close()
			return Details{}, err
		}
		line.UnitPrice = jsonMoney(unitPrice)
		line.Amount = jsonMoney(amount)
		details.Items = append(details.Items, line)
	}
	if err := itemRows.Err(); err != nil {
		itemRows.Close()
		return Details{}, err
	}
	itemRows.Close()

	paymentRows, err := repository.pool.Query(ctx, `
		SELECT id::text, invoice_id::text, amount::text, payment_date::text, payment_method, reference, notes, created_at
		FROM payments
		WHERE organization_id = $1::uuid AND invoice_id = $2::uuid
		ORDER BY payment_date, id
	`, organizationID, invoiceID)
	if err != nil {
		return Details{}, err
	}
	defer paymentRows.Close()
	for paymentRows.Next() {
		var payment Payment
		var amount string
		if err := paymentRows.Scan(&payment.ID, &payment.InvoiceID, &amount, &payment.PaymentDate, &payment.PaymentMethod, &payment.Reference, &payment.Notes, &payment.CreatedAt); err != nil {
			return Details{}, err
		}
		payment.Amount = jsonMoney(amount)
		details.Payments = append(details.Payments, payment)
	}
	return details, paymentRows.Err()
}

func (repository *Repository) Update(ctx context.Context, organizationID, invoiceID string, input Input, totals Totals) (Invoice, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Invoice{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockOrganization(ctx, tx, organizationID); err != nil {
		return Invoice{}, err
	}
	status, err := lockInvoice(ctx, tx, organizationID, invoiceID)
	if err != nil {
		return Invoice{}, err
	}
	paidCents, err := paidAmount(ctx, tx, organizationID, invoiceID)
	if err != nil {
		return Invoice{}, err
	}
	if status != StatusIssued || paidCents != 0 {
		return Invoice{}, ErrConflict
	}
	if err := relatedRecordsBelongToOrganization(ctx, tx, organizationID, input.CustomerID, input.AppointmentID); err != nil {
		return Invoice{}, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM invoice_items WHERE invoice_id = $1::uuid`, invoiceID); err != nil {
		return Invoice{}, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE invoices
		SET customer_id = $3::uuid, appointment_id = NULLIF($4, '')::uuid, issue_date = $5::date, due_date = $6::date,
		    subtotal = $7::numeric, tax = $8::numeric, total = $9::numeric, notes = $10, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1::uuid AND organization_id = $2::uuid
	`, invoiceID, organizationID, input.CustomerID, input.AppointmentID, input.IssueDate, input.DueDate, totals.Subtotal, totals.Tax, totals.Total, input.Notes)
	if err != nil {
		return Invoice{}, err
	}
	for index, line := range input.Items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO invoice_items (invoice_id, description, quantity, unit_price, amount)
			VALUES ($1::uuid, $2, $3, $4::numeric, $5::numeric)
		`, invoiceID, line.Description, line.Quantity, line.UnitPrice, totals.Items[index].Amount); err != nil {
			return Invoice{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Invoice{}, err
	}
	details, err := repository.Get(ctx, organizationID, invoiceID)
	if err != nil {
		return Invoice{}, err
	}
	return details.Invoice, nil
}

func (repository *Repository) Cancel(ctx context.Context, organizationID, invoiceID string) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockOrganization(ctx, tx, organizationID); err != nil {
		return err
	}
	status, err := lockInvoice(ctx, tx, organizationID, invoiceID)
	if err != nil {
		return err
	}
	paidCents, err := paidAmount(ctx, tx, organizationID, invoiceID)
	if err != nil {
		return err
	}
	if status != StatusIssued || paidCents != 0 {
		return ErrConflict
	}
	if _, err := tx.Exec(ctx, `
		UPDATE invoices SET status = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1::uuid AND organization_id = $2::uuid
	`, invoiceID, organizationID, StatusCancelled); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (repository *Repository) RecordPayment(ctx context.Context, organizationID, invoiceID string, input PaymentInput, amountCents int64) (Payment, Invoice, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Payment{}, Invoice{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockOrganization(ctx, tx, organizationID); err != nil {
		return Payment{}, Invoice{}, err
	}
	status, err := lockInvoice(ctx, tx, organizationID, invoiceID)
	if err != nil {
		return Payment{}, Invoice{}, err
	}
	if status != StatusIssued && status != StatusPartiallyPaid {
		return Payment{}, Invoice{}, ErrConflict
	}
	var totalText string
	if err := tx.QueryRow(ctx, `
		SELECT total::text FROM invoices WHERE id = $1::uuid AND organization_id = $2::uuid
	`, invoiceID, organizationID).Scan(&totalText); err != nil {
		return Payment{}, Invoice{}, err
	}
	totalCents, err := parseMoney(Money(totalText), false)
	if err != nil {
		return Payment{}, Invoice{}, err
	}
	paidCents, err := paidAmount(ctx, tx, organizationID, invoiceID)
	if err != nil {
		return Payment{}, Invoice{}, err
	}
	if amountCents > totalCents-paidCents {
		return Payment{}, Invoice{}, ErrOverpayment
	}
	newStatus := StatusAfterPayment(totalCents, paidCents+amountCents)
	var payment Payment
	var amountText string
	err = tx.QueryRow(ctx, `
		INSERT INTO payments (organization_id, invoice_id, amount, payment_date, payment_method, reference, notes)
		VALUES ($1::uuid, $2::uuid, $3::numeric, $4::date, $5, $6, $7)
		RETURNING id::text, invoice_id::text, amount::text, payment_date::text, payment_method, reference, notes, created_at
	`, organizationID, invoiceID, input.Amount, input.PaymentDate, input.PaymentMethod, input.Reference, input.Notes).Scan(
		&payment.ID, &payment.InvoiceID, &amountText, &payment.PaymentDate, &payment.PaymentMethod, &payment.Reference, &payment.Notes, &payment.CreatedAt,
	)
	if err != nil {
		return Payment{}, Invoice{}, err
	}
	payment.Amount = jsonMoney(amountText)
	if _, err := tx.Exec(ctx, `
		UPDATE invoices SET status = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1::uuid AND organization_id = $2::uuid
	`, invoiceID, organizationID, newStatus); err != nil {
		return Payment{}, Invoice{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Payment{}, Invoice{}, err
	}
	details, err := repository.Get(ctx, organizationID, invoiceID)
	if err != nil {
		return Payment{}, Invoice{}, err
	}
	return payment, details.Invoice, nil
}

func (repository *Repository) findInvoice(ctx context.Context, queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, organizationID, invoiceID string) (Invoice, error) {
	item, err := scanInvoice(queryer.QueryRow(ctx, `
		SELECT i.id::text, i.invoice_number, i.customer_id::text, c.name, COALESCE(i.appointment_id::text, ''),
		       i.status, i.issue_date::text, i.due_date::text, i.subtotal::text, i.tax::text, i.total::text,
		       COALESCE(SUM(p.amount), 0)::text, i.notes, i.created_at, i.updated_at
		FROM invoices AS i
		JOIN customers AS c ON c.id = i.customer_id AND c.organization_id = i.organization_id
		LEFT JOIN payments AS p ON p.invoice_id = i.id AND p.organization_id = i.organization_id
		WHERE i.id = $1::uuid AND i.organization_id = $2::uuid
		GROUP BY i.id, c.name
	`, invoiceID, organizationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, ErrNotFound
	}
	return item, err
}

func scanInvoice(row pgx.Row) (Invoice, error) {
	var item Invoice
	var appointmentID, subtotal, tax, total, paid string
	err := row.Scan(&item.ID, &item.InvoiceNumber, &item.CustomerID, &item.CustomerName, &appointmentID,
		&item.Status, &item.IssueDate, &item.DueDate, &subtotal, &tax, &total, &paid, &item.Notes, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Invoice{}, err
	}
	item.AppointmentID = appointmentID
	totalCents, err := parseMoney(Money(total), false)
	if err != nil {
		return Invoice{}, err
	}
	paidCents, err := parseMoney(Money(paid), false)
	if err != nil || paidCents > totalCents {
		return Invoice{}, ErrMoneyInvalid
	}
	item.Subtotal = jsonMoney(subtotal)
	item.Tax = jsonMoney(tax)
	item.Total = jsonMoney(total)
	item.AmountPaid = jsonMoney(formatMoney(paidCents))
	item.BalanceDue = jsonMoney(formatMoney(totalCents - paidCents))
	return item, nil
}

func scanItem(row pgx.Row) (Item, error) {
	var item Item
	var unitPrice, amount string
	err := row.Scan(&item.ID, &item.Description, &item.Quantity, &unitPrice, &amount)
	if err != nil {
		return Item{}, err
	}
	item.UnitPrice = jsonMoney(unitPrice)
	item.Amount = jsonMoney(amount)
	return item, nil
}

func lockOrganization(ctx context.Context, tx pgx.Tx, organizationID string) error {
	var lockedID string
	err := tx.QueryRow(ctx, `SELECT id::text FROM organizations WHERE id = $1::uuid FOR UPDATE`, organizationID).Scan(&lockedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func lockInvoice(ctx context.Context, tx pgx.Tx, organizationID, invoiceID string) (string, error) {
	var status string
	err := tx.QueryRow(ctx, `
		SELECT status FROM invoices WHERE id = $1::uuid AND organization_id = $2::uuid FOR UPDATE
	`, invoiceID, organizationID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return status, err
}

func paidAmount(ctx context.Context, queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, organizationID, invoiceID string) (int64, error) {
	var paid string
	err := queryer.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0)::text FROM payments
		WHERE organization_id = $1::uuid AND invoice_id = $2::uuid
	`, organizationID, invoiceID).Scan(&paid)
	if err != nil {
		return 0, err
	}
	return parseMoney(Money(paid), false)
}

func relatedRecordsBelongToOrganization(ctx context.Context, tx pgx.Tx, organizationID, customerID, appointmentID string) error {
	var customerExists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM customers WHERE id = $1::uuid AND organization_id = $2::uuid)
	`, customerID, organizationID).Scan(&customerExists); err != nil {
		return err
	}
	if !customerExists {
		return ErrRelatedRecordNotFound
	}
	if appointmentID == "" {
		return nil
	}
	var appointmentCustomerID string
	err := tx.QueryRow(ctx, `
		SELECT customer_id::text FROM bookings WHERE id = $1::uuid AND organization_id = $2::uuid
	`, appointmentID, organizationID).Scan(&appointmentCustomerID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && appointmentCustomerID != customerID) {
		return ErrRelatedRecordNotFound
	}
	return err
}
