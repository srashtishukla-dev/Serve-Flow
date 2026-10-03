package organizations

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("organization not found")

type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (repository *Repository) Create(ctx context.Context, tx pgx.Tx, name, slug string) (Organization, error) {
	for attempt := 0; attempt < 5; attempt++ {
		candidate := slug
		if attempt > 0 {
			suffix, err := randomSuffix()
			if err != nil {
				return Organization{}, err
			}
			candidate = fmt.Sprintf("%s-%s", slug, suffix)
		}

		var organization Organization
		err := tx.QueryRow(ctx, `
			INSERT INTO organizations (name, slug)
			VALUES ($1, $2)
			ON CONFLICT (slug) DO NOTHING
			RETURNING id::text, name, slug
		`, name, candidate).Scan(&organization.ID, &organization.Name, &organization.Slug)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return Organization{}, err
		}
		return organization, nil
	}
	return Organization{}, errors.New("unable to allocate a unique organization slug")
}

func (repository *Repository) FindByID(ctx context.Context, id string) (Organization, error) {
	var organization Organization
	err := repository.pool.QueryRow(ctx, `
		SELECT id::text, name, slug
		FROM organizations
		WHERE id::text = $1
	`, id).Scan(&organization.ID, &organization.Name, &organization.Slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return Organization{}, ErrNotFound
	}
	if err != nil {
		return Organization{}, err
	}
	return organization, nil
}

func WorkspaceSlug(name string) string {
	var slug strings.Builder
	lastWasSeparator := true
	for _, character := range strings.ToLower(strings.TrimSpace(name)) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			slug.WriteRune(character)
			lastWasSeparator = false
		} else if !lastWasSeparator {
			slug.WriteByte('-')
			lastWasSeparator = true
		}
	}
	base := strings.Trim(slug.String(), "-")
	if base == "" {
		base = "workspace"
	}
	return base + "-workspace"
}

func randomSuffix() (string, error) {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
