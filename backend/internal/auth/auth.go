package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/serveflow/serveflow/backend/internal/organizations"
	"github.com/serveflow/serveflow/backend/internal/users"
)

var (
	ErrInvalidInput = errors.New("name, valid email, and password are required; password must be 8 to 72 bytes")
	ErrEmailTaken   = errors.New("an account with this email already exists")
	ErrInvalidLogin = errors.New("invalid email or password")
)

type Service struct {
	pool          *pgxpool.Pool
	users         *users.Repository
	organizations *organizations.Repository
	tokens        *TokenManager
}

func NewService(pool *pgxpool.Pool, userRepository *users.Repository, organizationRepository *organizations.Repository, tokenManager *TokenManager) *Service {
	return &Service{pool: pool, users: userRepository, organizations: organizationRepository, tokens: tokenManager}
}

func (s *Service) Register(ctx context.Context, name, email, password string) (users.User, error) {
	name = strings.TrimSpace(name)
	email, validEmail := normalizeEmail(email)
	if name == "" || len(name) > 200 || !validEmail || len(password) < 8 || len(password) > 72 {
		return users.User{}, ErrInvalidInput
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return users.User{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return users.User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	organization, err := s.organizations.Create(ctx, tx, name+"'s Workspace", organizations.WorkspaceSlug(name))
	if err != nil {
		return users.User{}, err
	}
	user, err := s.users.Create(ctx, tx, organization.ID, name, email, string(passwordHash))
	if errors.Is(err, users.ErrEmailExists) {
		return users.User{}, ErrEmailTaken
	}
	if err != nil {
		return users.User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return users.User{}, err
	}
	return user, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (users.User, string, error) {
	email, validEmail := normalizeEmail(email)
	if !validEmail || password == "" || len(password) > 72 {
		return users.User{}, "", ErrInvalidInput
	}

	user, passwordHash, err := s.users.FindByEmail(ctx, email)
	if errors.Is(err, users.ErrNotFound) {
		return users.User{}, "", ErrInvalidLogin
	}
	if err != nil {
		return users.User{}, "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		return users.User{}, "", ErrInvalidLogin
	}
	token, err := s.tokens.Issue(user.ID, user.OrganizationID)
	if err != nil {
		return users.User{}, "", err
	}
	return user, token, nil
}

func normalizeEmail(email string) (string, bool) {
	email = strings.ToLower(strings.TrimSpace(email))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || len(email) > 254 {
		return "", false
	}
	return email, true
}
