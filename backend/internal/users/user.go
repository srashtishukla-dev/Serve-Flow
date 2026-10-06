package users

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrEmailExists = errors.New("email already exists")
	ErrNotFound    = errors.New("user not found")
	ErrLinkInvalid = errors.New("record not found in this organization")
	ErrLinkTaken   = errors.New("this record already has a login")
)

const (
	RoleAdmin      = "ADMIN"
	RoleTechnician = "TECHNICIAN"
	RoleCustomer   = "CUSTOMER"
)

type User struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	OrganizationID string `json:"-"`
	CustomerID     string `json:"customer_id,omitempty"`
	TechnicianID   string `json:"technician_id,omitempty"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (repository *Repository) Create(ctx context.Context, tx pgx.Tx, organizationID, name, email, passwordHash string) (User, error) {
	var user User
	err := tx.QueryRow(ctx, `
		INSERT INTO users (organization_id, name, email, password_hash)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (email) DO NOTHING
		RETURNING id::text, name, email, role, organization_id::text, COALESCE(customer_id::text, ''), COALESCE(technician_id::text, '')
	`, organizationID, name, email, passwordHash).Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.OrganizationID, &user.CustomerID, &user.TechnicianID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrEmailExists
	}
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (repository *Repository) FindByEmail(ctx context.Context, email string) (User, string, error) {
	var user User
	var passwordHash string
	err := repository.pool.QueryRow(ctx, `
		SELECT id::text, name, email, role, password_hash, organization_id::text, COALESCE(customer_id::text, ''), COALESCE(technician_id::text, '')
		FROM users
		WHERE email = $1
	`, email).Scan(&user.ID, &user.Name, &user.Email, &user.Role, &passwordHash, &user.OrganizationID, &user.CustomerID, &user.TechnicianID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	if err != nil {
		return User{}, "", err
	}
	return user, passwordHash, nil
}

func (repository *Repository) FindByID(ctx context.Context, id string) (User, error) {
	var user User
	err := repository.pool.QueryRow(ctx, `
		SELECT id::text, name, email, role, organization_id::text, COALESCE(customer_id::text, ''), COALESCE(technician_id::text, '')
		FROM users
		WHERE id::text = $1
	`, id).Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.OrganizationID, &user.CustomerID, &user.TechnicianID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return user, nil
}

type contextKey struct{}

// WithUser stores the freshly loaded identity for the request.
func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, contextKey{}, user)
}

func FromContext(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(contextKey{}).(User)
	return user, ok && user.ID != ""
}

// CreateLinked creates a CUSTOMER or TECHNICIAN login linked to an existing record
// of the same organization. The organization comes from the authenticated admin.
func (repository *Repository) CreateLinked(ctx context.Context, organizationID, role, recordID, name, email, passwordHash string) (User, error) {
	var table, column string
	switch role {
	case RoleCustomer:
		table, column = "customers", "customer_id"
	case RoleTechnician:
		table, column = "technicians", "technician_id"
	default:
		return User{}, ErrLinkInvalid
	}
	var user User
	err := repository.pool.QueryRow(ctx, `
		INSERT INTO users (organization_id, name, email, password_hash, role, `+column+`)
		SELECT record.organization_id, $3, $4, $5, $6, record.id
		FROM `+table+` AS record
		WHERE record.id = $2::uuid AND record.organization_id = $1::uuid
		RETURNING id::text, name, email, role, organization_id::text, COALESCE(customer_id::text, ''), COALESCE(technician_id::text, '')
	`, organizationID, recordID, name, email, passwordHash, role).Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.OrganizationID, &user.CustomerID, &user.TechnicianID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrLinkInvalid
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if strings.Contains(pgErr.ConstraintName, "email") {
			return User{}, ErrEmailExists
		}
		return User{}, ErrLinkTaken
	}
	return user, err
}
