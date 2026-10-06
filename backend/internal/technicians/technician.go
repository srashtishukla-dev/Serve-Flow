package technicians

import (
	"context"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/serveflow/serveflow/backend/internal/pagination"
)

const (
	StatusActive   = "ACTIVE"
	StatusInactive = "INACTIVE"
)

var (
	ErrNotFound       = errors.New("technician not found")
	ErrNameRequired   = errors.New("name is required")
	ErrNameTooLong    = errors.New("name must be 120 characters or fewer")
	ErrEmailInvalid   = errors.New("email must be a valid email address")
	ErrEmailTooLong   = errors.New("email must be 254 characters or fewer")
	ErrPhoneTooLong   = errors.New("phone must be 50 characters or fewer")
	technicianIDRegex = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

type Input struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

type Technician struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Name           string    `json:"name"`
	Email          string    `json:"email,omitempty"`
	Phone          string    `json:"phone,omitempty"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
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
	return nil
}

func (repository *Repository) Create(ctx context.Context, organizationID string, input Input) (Technician, error) {
	return scanTechnician(repository.pool.QueryRow(ctx, `
		INSERT INTO technicians (organization_id, name, email, phone)
		VALUES ($1::uuid, $2, NULLIF($3, ''), NULLIF($4, ''))
		RETURNING id::text, organization_id::text, name, COALESCE(email, ''), COALESCE(phone, ''), status, created_at, updated_at
	`, organizationID, input.Name, input.Email, input.Phone))
}

func (repository *Repository) List(ctx context.Context, organizationID string) ([]Technician, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text, organization_id::text, name, COALESCE(email, ''), COALESCE(phone, ''), status, created_at, updated_at
		FROM technicians
		WHERE organization_id = $1::uuid
		ORDER BY created_at DESC, id
	`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Technician, 0)
	for rows.Next() {
		item, err := scanTechnician(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *Repository) ListPage(ctx context.Context, organizationID string, query pagination.Query) ([]Technician, pagination.Metadata, error) {
	if query.Status != "" && query.Status != StatusActive && query.Status != StatusInactive {
		return nil, pagination.Metadata{}, errors.New("status must be ACTIVE or INACTIVE")
	}
	var total int
	if err := repository.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM technicians
		WHERE organization_id = $1::uuid
		  AND ($2 = '' OR status = $2)
		  AND ($3 = '' OR name ILIKE '%' || $3 || '%' OR COALESCE(email, '') ILIKE '%' || $3 || '%' OR COALESCE(phone, '') ILIKE '%' || $3 || '%')
	`, organizationID, query.Status, query.Search).Scan(&total); err != nil {
		return nil, pagination.Metadata{}, err
	}
	orderBy := pagination.OrderBy(query, map[string]string{
		"created_at": "created_at", "updated_at": "updated_at", "name": "name", "status": "status",
	})
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text, organization_id::text, name, COALESCE(email, ''), COALESCE(phone, ''), status, created_at, updated_at
		FROM technicians
		WHERE organization_id = $1::uuid
		  AND ($2 = '' OR status = $2)
		  AND ($3 = '' OR name ILIKE '%' || $3 || '%' OR COALESCE(email, '') ILIKE '%' || $3 || '%' OR COALESCE(phone, '') ILIKE '%' || $3 || '%')
		ORDER BY `+orderBy+`, id ASC
		LIMIT $4 OFFSET $5
	`, organizationID, query.Status, query.Search, query.Limit, query.Offset)
	if err != nil {
		return nil, pagination.Metadata{}, err
	}
	defer rows.Close()
	items := make([]Technician, 0)
	for rows.Next() {
		item, err := scanTechnician(rows)
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

func (repository *Repository) FindByID(ctx context.Context, organizationID, technicianID string) (Technician, error) {
	item, err := scanTechnician(repository.pool.QueryRow(ctx, `
		SELECT id::text, organization_id::text, name, COALESCE(email, ''), COALESCE(phone, ''), status, created_at, updated_at
		FROM technicians
		WHERE id = $1::uuid AND organization_id = $2::uuid
	`, technicianID, organizationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Technician{}, ErrNotFound
	}
	return item, err
}

func (repository *Repository) Update(ctx context.Context, organizationID, technicianID string, input Input) (Technician, error) {
	item, err := scanTechnician(repository.pool.QueryRow(ctx, `
		UPDATE technicians
		SET name = $3, email = NULLIF($4, ''), phone = NULLIF($5, ''), updated_at = CURRENT_TIMESTAMP
		WHERE id = $1::uuid AND organization_id = $2::uuid
		RETURNING id::text, organization_id::text, name, COALESCE(email, ''), COALESCE(phone, ''), status, created_at, updated_at
	`, technicianID, organizationID, input.Name, input.Email, input.Phone))
	if errors.Is(err, pgx.ErrNoRows) {
		return Technician{}, ErrNotFound
	}
	return item, err
}

func (repository *Repository) Deactivate(ctx context.Context, organizationID, technicianID string) error {
	tag, err := repository.pool.Exec(ctx, `
		UPDATE technicians
		SET status = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1::uuid AND organization_id = $2::uuid
	`, technicianID, organizationID, StatusInactive)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func ValidID(id string) bool {
	return technicianIDRegex.MatchString(id)
}

func scanTechnician(row pgx.Row) (Technician, error) {
	var item Technician
	err := row.Scan(&item.ID, &item.OrganizationID, &item.Name, &item.Email, &item.Phone, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Technician{}, err
	}
	return item, nil
}
