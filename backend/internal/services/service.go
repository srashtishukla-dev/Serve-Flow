package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/serveflow/serveflow/backend/internal/cache"
)

const serviceListCacheTTL = 2 * time.Minute

var (
	ErrNotFound           = errors.New("service not found")
	ErrNameRequired       = errors.New("name is required")
	ErrNameTooLong        = errors.New("name must be 120 characters or fewer")
	ErrDescriptionTooLong = errors.New("description must be 2000 characters or fewer")
	ErrDurationInvalid    = errors.New("duration_minutes must be greater than zero")
	ErrPriceInvalid       = errors.New("price must be a non-negative amount with at most two decimal places")
	priceRegex            = regexp.MustCompile(`^\d+(\.\d{1,2})?$`)
	serviceIDRegex        = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

type Decimal string

func (decimal *Decimal) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return errors.New("price is required")
	}
	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*decimal = Decimal(value)
		return nil
	}
	*decimal = Decimal(string(data))
	return nil
}

type Input struct {
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	DurationMinutes int     `json:"duration_minutes"`
	Price           Decimal `json:"price"`
}

type Service struct {
	ID              string      `json:"id"`
	Name            string      `json:"name"`
	Description     string      `json:"description"`
	DurationMinutes int         `json:"duration_minutes"`
	Price           json.Number `json:"price"`
}

type Repository struct {
	pool  *pgxpool.Pool
	cache *cache.Client
}

func NewRepository(pool *pgxpool.Pool, cacheClients ...*cache.Client) *Repository {
	repository := &Repository{pool: pool}
	if len(cacheClients) > 0 {
		repository.cache = cacheClients[0]
	}
	return repository
}

func (input *Input) Validate() (string, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if input.Name == "" {
		return "", ErrNameRequired
	}
	if utf8.RuneCountInString(input.Name) > 120 {
		return "", ErrNameTooLong
	}
	if utf8.RuneCountInString(input.Description) > 2000 {
		return "", ErrDescriptionTooLong
	}
	if input.DurationMinutes <= 0 {
		return "", ErrDurationInvalid
	}
	return decimalString(input.Price)
}

func (repository *Repository) Create(ctx context.Context, organizationID string, input Input, price string) (Service, error) {
	item, err := scanService(repository.pool.QueryRow(ctx, `
		INSERT INTO services (organization_id, name, description, duration_minutes, price)
		VALUES ($1::uuid, $2, $3, $4, $5::numeric)
		RETURNING id::text, name, description, duration_minutes, price::text
	`, organizationID, input.Name, input.Description, input.DurationMinutes, price))
	if err == nil {
		repository.invalidateList(ctx, organizationID)
	}
	return item, err
}

func (repository *Repository) List(ctx context.Context, organizationID string) ([]Service, error) {
	cacheKey := serviceListCacheKey(organizationID)
	if repository.cache != nil {
		data, found, err := repository.cache.Get(ctx, cacheKey)
		if err == nil && found {
			var items []Service
			if json.Unmarshal(data, &items) == nil {
				return items, nil
			}
			_ = repository.cache.Delete(ctx, cacheKey)
		}
	}

	rows, err := repository.pool.Query(ctx, `
		SELECT id::text, name, description, duration_minutes, price::text
		FROM services
		WHERE organization_id = $1::uuid
		ORDER BY created_at DESC, id
	`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Service, 0)
	for rows.Next() {
		item, err := scanService(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if repository.cache != nil {
		if data, err := json.Marshal(items); err == nil {
			_ = repository.cache.Set(ctx, cacheKey, data, serviceListCacheTTL)
		}
	}
	return items, nil
}

func (repository *Repository) FindByID(ctx context.Context, organizationID, serviceID string) (Service, error) {
	item, err := scanService(repository.pool.QueryRow(ctx, `
		SELECT id::text, name, description, duration_minutes, price::text
		FROM services
		WHERE id = $1::uuid AND organization_id = $2::uuid
	`, serviceID, organizationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Service{}, ErrNotFound
	}
	return item, err
}

func (repository *Repository) Update(ctx context.Context, organizationID, serviceID string, input Input, price string) (Service, error) {
	item, err := scanService(repository.pool.QueryRow(ctx, `
		UPDATE services
		SET name = $3, description = $4, duration_minutes = $5, price = $6::numeric, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1::uuid AND organization_id = $2::uuid
		RETURNING id::text, name, description, duration_minutes, price::text
	`, serviceID, organizationID, input.Name, input.Description, input.DurationMinutes, price))
	if errors.Is(err, pgx.ErrNoRows) {
		return Service{}, ErrNotFound
	}
	if err == nil {
		repository.invalidateList(ctx, organizationID)
	}
	return item, err
}

func (repository *Repository) Delete(ctx context.Context, organizationID, serviceID string) error {
	tag, err := repository.pool.Exec(ctx, `
		DELETE FROM services
		WHERE id = $1::uuid AND organization_id = $2::uuid
	`, serviceID, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	repository.invalidateList(ctx, organizationID)
	return nil
}

func (repository *Repository) invalidateList(ctx context.Context, organizationID string) {
	if repository.cache != nil {
		_ = repository.cache.Delete(ctx, serviceListCacheKey(organizationID))
	}
}

func serviceListCacheKey(organizationID string) string {
	return "serveflow:org:" + organizationID + ":services"
}

func ValidID(id string) bool {
	return serviceIDRegex.MatchString(id)
}

func decimalString(price Decimal) (string, error) {
	rawPrice := strings.TrimSpace(string(price))
	if !priceRegex.MatchString(rawPrice) {
		return "", ErrPriceInvalid
	}
	value, ok := new(big.Rat).SetString(rawPrice)
	if !ok || value.Sign() < 0 {
		return "", ErrPriceInvalid
	}
	cents := new(big.Rat).Mul(value, big.NewRat(100, 1))
	if !cents.IsInt() {
		return "", ErrPriceInvalid
	}
	centsValue := cents.Num()
	maximumCents := big.NewInt(999999999999)
	if !centsValue.IsInt64() || centsValue.Cmp(maximumCents) > 0 {
		return "", ErrPriceInvalid
	}
	amount := centsValue.Int64()
	return fmt.Sprintf("%d.%02d", amount/100, amount%100), nil
}

func scanService(row pgx.Row) (Service, error) {
	var item Service
	var price string
	err := row.Scan(&item.ID, &item.Name, &item.Description, &item.DurationMinutes, &price)
	if err != nil {
		return Service{}, err
	}
	item.Price = json.Number(price)
	return item, nil
}
