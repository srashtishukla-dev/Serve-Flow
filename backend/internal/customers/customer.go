package customers

import (
	"context"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/serveflow/serveflow/backend/internal/pagination"
)

var (
	ErrNotFound     = errors.New("customer not found")
	ErrNameRequired = errors.New("name is required")
	ErrNameTooLong  = errors.New("name must be 120 characters or fewer")
	ErrEmailInvalid = errors.New("email must be a valid email address")
	ErrEmailTooLong = errors.New("email must be 254 characters or fewer")
	ErrPhoneTooLong = errors.New("phone must be 50 characters or fewer")
	ErrNotesTooLong = errors.New("notes must be 2000 characters or fewer")
	customerIDRegex = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

type Input struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	Notes string `json:"notes"`
}

type Customer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
	Notes string `json:"notes,omitempty"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (input *Input) Validate() error {
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.TrimSpace(input.Email)
	input.Phone = strings.TrimSpace(input.Phone)
	input.Notes = strings.TrimSpace(input.Notes)
	if input.Name == "" {
		return ErrNameRequired
	}
	if utf8.RuneCountInString(input.Name) > 120 {
		return ErrNameTooLong
	}
	if utf8.RuneCountInString(input.Email) > 254 {
		return ErrEmailTooLong
	}
	if input.Email != "" {
		parsed, err := mail.ParseAddress(input.Email)
		if err != nil || parsed.Address != input.Email {
			return ErrEmailInvalid
		}
	}
	if utf8.RuneCountInString(input.Phone) > 50 {
		return ErrPhoneTooLong
	}
	if utf8.RuneCountInString(input.Notes) > 2000 {
		return ErrNotesTooLong
	}
	return nil
}

func (repository *Repository) Create(ctx context.Context, organizationID string, input Input) (Customer, error) {
	return scanCustomer(repository.pool.QueryRow(ctx, `
		INSERT INTO customers (organization_id, name, email, phone, notes)
		VALUES ($1::uuid, $2, NULLIF($3, ''), NULLIF($4, ''), $5)
		RETURNING id::text, name, COALESCE(email, ''), COALESCE(phone, ''), notes
	`, organizationID, input.Name, input.Email, input.Phone, input.Notes))
}

func (repository *Repository) List(ctx context.Context, organizationID string) ([]Customer, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text, name, COALESCE(email, ''), COALESCE(phone, ''), notes
		FROM customers
		WHERE organization_id = $1::uuid
		ORDER BY created_at DESC, id
	`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Customer, 0)
	for rows.Next() {
		item, err := scanCustomer(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *Repository) ListPage(ctx context.Context, organizationID string, query pagination.Query) ([]Customer, pagination.Metadata, error) {
	var total int
	if err := repository.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM customers
		WHERE organization_id = $1::uuid
		  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR COALESCE(email, '') ILIKE '%' || $2 || '%' OR COALESCE(phone, '') ILIKE '%' || $2 || '%')
	`, organizationID, query.Search).Scan(&total); err != nil {
		return nil, pagination.Metadata{}, err
	}
	orderBy := pagination.OrderBy(query, map[string]string{
		"created_at": "created_at", "updated_at": "updated_at", "name": "name",
	})
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text, name, COALESCE(email, ''), COALESCE(phone, ''), notes
		FROM customers
		WHERE organization_id = $1::uuid
		  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR COALESCE(email, '') ILIKE '%' || $2 || '%' OR COALESCE(phone, '') ILIKE '%' || $2 || '%')
		ORDER BY `+orderBy+`, id ASC
		LIMIT $3 OFFSET $4
	`, organizationID, query.Search, query.Limit, query.Offset)
	if err != nil {
		return nil, pagination.Metadata{}, err
	}
	defer rows.Close()
	items := make([]Customer, 0)
	for rows.Next() {
		item, err := scanCustomer(rows)
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

func (repository *Repository) FindByID(ctx context.Context, organizationID, customerID string) (Customer, error) {
	item, err := scanCustomer(repository.pool.QueryRow(ctx, `
		SELECT id::text, name, COALESCE(email, ''), COALESCE(phone, ''), notes
		FROM customers
		WHERE id = $1::uuid AND organization_id = $2::uuid
	`, customerID, organizationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Customer{}, ErrNotFound
	}
	return item, err
}

func (repository *Repository) Update(ctx context.Context, organizationID, customerID string, input Input) (Customer, error) {
	item, err := scanCustomer(repository.pool.QueryRow(ctx, `
		UPDATE customers
		SET name = $3, email = NULLIF($4, ''), phone = NULLIF($5, ''), notes = $6, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1::uuid AND organization_id = $2::uuid
		RETURNING id::text, name, COALESCE(email, ''), COALESCE(phone, ''), notes
	`, customerID, organizationID, input.Name, input.Email, input.Phone, input.Notes))
	if errors.Is(err, pgx.ErrNoRows) {
		return Customer{}, ErrNotFound
	}
	return item, err
}

func (repository *Repository) Delete(ctx context.Context, organizationID, customerID string) error {
	tag, err := repository.pool.Exec(ctx, `
		DELETE FROM customers
		WHERE id = $1::uuid AND organization_id = $2::uuid
	`, customerID, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func ValidID(id string) bool {
	return customerIDRegex.MatchString(id)
}

func scanCustomer(row pgx.Row) (Customer, error) {
	var customer Customer
	err := row.Scan(&customer.ID, &customer.Name, &customer.Email, &customer.Phone, &customer.Notes)
	if err != nil {
		return Customer{}, err
	}
	return customer, nil
}
