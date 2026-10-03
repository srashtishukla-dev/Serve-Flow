package users

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrEmailExists = errors.New("email already exists")
	ErrNotFound    = errors.New("user not found")
)

type User struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	OrganizationID string `json:"-"`
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
		RETURNING id::text, name, email, organization_id::text
	`, organizationID, name, email, passwordHash).Scan(&user.ID, &user.Name, &user.Email, &user.OrganizationID)
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
		SELECT id::text, name, email, password_hash, organization_id::text
		FROM users
		WHERE email = $1
	`, email).Scan(&user.ID, &user.Name, &user.Email, &passwordHash, &user.OrganizationID)
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
		SELECT id::text, name, email, organization_id::text
		FROM users
		WHERE id::text = $1
	`, id).Scan(&user.ID, &user.Name, &user.Email, &user.OrganizationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return user, nil
}
