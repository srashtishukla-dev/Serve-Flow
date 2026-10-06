package email

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxAttempts = 3

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (repository *Repository) Queue(ctx context.Context, organizationID, userID, eventType, title, content string) (Delivery, error) {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return Delivery{}, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	var recipient string
	if err := transaction.QueryRow(ctx, `
		SELECT email
		FROM users
		WHERE id = $1::uuid AND organization_id = $2::uuid
	`, userID, organizationID).Scan(&recipient); err != nil {
		return Delivery{}, err
	}
	if _, err := BuildMessage(recipient, eventType, title, content); err != nil {
		return Delivery{}, err
	}
	_, err = transaction.Exec(ctx, `
		INSERT INTO notifications (organization_id, user_id, type, title, message)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5)
	`, organizationID, userID, eventType, title, content)
	if err != nil {
		return Delivery{}, err
	}
	delivery, err := scanDelivery(transaction.QueryRow(ctx, `
		INSERT INTO email_deliveries (organization_id, user_id, recipient, event_type, title, content)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)
		RETURNING id::text, organization_id::text, user_id::text, recipient, event_type, title, content, attempts
	`, organizationID, userID, recipient, eventType, title, content))
	if err != nil {
		return Delivery{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return Delivery{}, err
	}
	return delivery, nil
}

func (repository *Repository) RecoverStale(ctx context.Context) error {
	_, err := repository.pool.Exec(ctx, `
		UPDATE email_deliveries
		SET status = 'queued', available_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP,
		    last_error = 'delivery interrupted before completion'
		WHERE status = 'processing'
		  AND updated_at < CURRENT_TIMESTAMP - INTERVAL '5 minutes'
		  AND attempts < $1
	`, maxAttempts)
	if err != nil {
		return err
	}
	_, err = repository.pool.Exec(ctx, `
		UPDATE email_deliveries
		SET status = 'failed', updated_at = CURRENT_TIMESTAMP,
		    last_error = 'maximum delivery attempts reached'
		WHERE status = 'processing'
		  AND updated_at < CURRENT_TIMESTAMP - INTERVAL '5 minutes'
		  AND attempts >= $1
	`, maxAttempts)
	return err
}

func (repository *Repository) ClaimNext(ctx context.Context) (Delivery, bool, error) {
	delivery, err := scanDelivery(repository.pool.QueryRow(ctx, `
		WITH next_delivery AS (
			SELECT id
			FROM email_deliveries
			WHERE status = 'queued'
			  AND available_at <= CURRENT_TIMESTAMP
			  AND attempts < $1
			ORDER BY created_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE email_deliveries AS delivery
		SET status = 'processing', attempts = delivery.attempts + 1, updated_at = CURRENT_TIMESTAMP
		FROM next_delivery
		WHERE delivery.id = next_delivery.id
		RETURNING delivery.id::text, delivery.organization_id::text, delivery.user_id::text,
		          delivery.recipient, delivery.event_type, delivery.title, delivery.content, delivery.attempts
	`, maxAttempts))
	if errors.Is(err, pgx.ErrNoRows) {
		return Delivery{}, false, nil
	}
	if err != nil {
		return Delivery{}, false, err
	}
	return delivery, true, nil
}

func (repository *Repository) MarkSent(ctx context.Context, id string) error {
	commandTag, err := repository.pool.Exec(ctx, `
		UPDATE email_deliveries
		SET status = 'sent', sent_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP, last_error = NULL
		WHERE id = $1::uuid AND status = 'processing'
	`, id)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() != 1 {
		return errors.New("email delivery state changed before marking sent")
	}
	return nil
}

func (repository *Repository) MarkFailed(ctx context.Context, id string, attempts int, retry bool, safeError string) error {
	delay := 0 * time.Second
	if attempts == 1 {
		delay = 30 * time.Second
	} else if attempts == 2 {
		delay = 2 * time.Minute
	}
	status := "failed"
	if retry && attempts < maxAttempts {
		status = "queued"
	}
	commandTag, err := repository.pool.Exec(ctx, `
		UPDATE email_deliveries
		SET status = $2, available_at = CURRENT_TIMESTAMP + $3::interval,
		    updated_at = CURRENT_TIMESTAMP, last_error = $4
		WHERE id = $1::uuid AND status = 'processing'
	`, id, status, intervalString(delay), safeError)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() != 1 {
		return errors.New("email delivery state changed before marking failed")
	}
	return nil
}

func intervalString(delay time.Duration) string {
	return delay.String()
}

type rowScanner interface {
	Scan(...any) error
}

func scanDelivery(row rowScanner) (Delivery, error) {
	var delivery Delivery
	err := row.Scan(
		&delivery.ID, &delivery.OrganizationID, &delivery.UserID, &delivery.Recipient,
		&delivery.Type, &delivery.Title, &delivery.Content, &delivery.Attempts,
	)
	return delivery, err
}
