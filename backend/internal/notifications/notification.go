package notifications

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/serveflow/serveflow/backend/internal/pagination"
)

var (
	ErrNotFound       = errors.New("notification not found")
	ErrUserNotFound   = errors.New("notification user not found in organization")
	ErrInvalidContent = errors.New("notification type, title, and message are required and must fit their limits")
	notificationIDRE  = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

type Input struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

type Notification struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"-"`
	UserID         string    `json:"-"`
	Type           string    `json:"type"`
	Title          string    `json:"title"`
	Message        string    `json:"message"`
	IsRead         bool      `json:"is_read"`
	CreatedAt      time.Time `json:"created_at"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (input *Input) Validate() error {
	input.Type = strings.TrimSpace(input.Type)
	input.Title = strings.TrimSpace(input.Title)
	input.Message = strings.TrimSpace(input.Message)
	if input.Type == "" || utf8.RuneCountInString(input.Type) > 50 ||
		input.Title == "" || utf8.RuneCountInString(input.Title) > 200 ||
		input.Message == "" || utf8.RuneCountInString(input.Message) > 2000 {
		return ErrInvalidContent
	}
	return nil
}

func ValidID(id string) bool {
	return notificationIDRE.MatchString(id)
}

func (repository *Repository) Create(ctx context.Context, organizationID, userID string, input Input) (Notification, error) {
	if err := input.Validate(); err != nil {
		return Notification{}, err
	}
	item, err := scanNotification(repository.pool.QueryRow(ctx, `
		INSERT INTO notifications (organization_id, user_id, type, title, message)
		SELECT $1::uuid, $2::uuid, $3, $4, $5
		WHERE EXISTS (
			SELECT 1 FROM users
			WHERE id = $2::uuid AND organization_id = $1::uuid
		)
		RETURNING id::text, organization_id::text, user_id::text, type, title, message, is_read, created_at
	`, organizationID, userID, input.Type, input.Title, input.Message))
	if errors.Is(err, pgx.ErrNoRows) {
		return Notification{}, ErrUserNotFound
	}
	return item, err
}

func (repository *Repository) List(ctx context.Context, organizationID, userID string) ([]Notification, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text, organization_id::text, user_id::text, type, title, message, is_read, created_at
		FROM notifications
		WHERE organization_id = $1::uuid AND user_id = $2::uuid
		ORDER BY created_at DESC, id DESC
		LIMIT 100
	`, organizationID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Notification, 0)
	for rows.Next() {
		item, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *Repository) ListPage(ctx context.Context, organizationID, userID string, query pagination.Query) ([]Notification, pagination.Metadata, error) {
	var total int
	if err := repository.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM notifications
		WHERE organization_id = $1::uuid AND user_id = $2::uuid
		  AND ($3::boolean IS NULL OR is_read = $3)
		  AND ($4 = '' OR title ILIKE '%' || $4 || '%' OR message ILIKE '%' || $4 || '%')
	`, organizationID, userID, query.IsRead, query.Search).Scan(&total); err != nil {
		return nil, pagination.Metadata{}, err
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text, organization_id::text, user_id::text, type, title, message, is_read, created_at
		FROM notifications
		WHERE organization_id = $1::uuid AND user_id = $2::uuid
		  AND ($3::boolean IS NULL OR is_read = $3)
		  AND ($4 = '' OR title ILIKE '%' || $4 || '%' OR message ILIKE '%' || $4 || '%')
		ORDER BY `+pagination.OrderBy(query, map[string]string{"created_at": "created_at"})+`, id DESC
		LIMIT $5 OFFSET $6
	`, organizationID, userID, query.IsRead, query.Search, query.Limit, query.Offset)
	if err != nil {
		return nil, pagination.Metadata{}, err
	}
	defer rows.Close()
	items := make([]Notification, 0)
	for rows.Next() {
		item, err := scanNotification(rows)
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

func (repository *Repository) MarkRead(ctx context.Context, organizationID, userID, notificationID string) (Notification, error) {
	if !ValidID(notificationID) {
		return Notification{}, ErrNotFound
	}
	item, err := scanNotification(repository.pool.QueryRow(ctx, `
		UPDATE notifications
		SET is_read = TRUE
		WHERE id = $1::uuid AND organization_id = $2::uuid AND user_id = $3::uuid
		RETURNING id::text, organization_id::text, user_id::text, type, title, message, is_read, created_at
	`, notificationID, organizationID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Notification{}, ErrNotFound
	}
	return item, err
}

func scanNotification(row pgx.Row) (Notification, error) {
	var item Notification
	err := row.Scan(&item.ID, &item.OrganizationID, &item.UserID, &item.Type, &item.Title, &item.Message, &item.IsRead, &item.CreatedAt)
	return item, err
}
