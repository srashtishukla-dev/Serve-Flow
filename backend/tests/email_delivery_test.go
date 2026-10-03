package tests_test

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/serveflow/serveflow/backend/internal/auth"
	"github.com/serveflow/serveflow/backend/internal/email"
	"github.com/serveflow/serveflow/backend/internal/jobs"
	"github.com/serveflow/serveflow/backend/internal/routes"
	"github.com/serveflow/serveflow/backend/internal/users"
)

type integrationEmailSender struct {
	err error
}

func (sender integrationEmailSender) Send(context.Context, email.Message) error {
	return sender.err
}

func TestRegistrationQueuesAndSendsWelcomeEmailAfterCreatingAccount(t *testing.T) {
	server := newAuthTestServer(t)
	emailAddress := "welcome-integration+" + testEmail()
	t.Cleanup(func() { cleanupUser(t, server, emailAddress) })

	worker := jobs.NewWorker(server.notifications, log.New(io.Discard, "", 0), 8, jobs.EmailDependencies{
		Store:  email.NewRepository(server.pool),
		Sender: integrationEmailSender{},
	})
	worker.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = worker.Shutdown(ctx)
	})
	server.handler = routes.New(auth.NewService(server.pool, users.NewRepository(server.pool), server.organizations, server.tokens), server.tokens, users.NewRepository(server.pool), server.organizations,
		server.services, server.customers, server.bookings, server.technicians, server.invoices,
		server.notifications, worker, server.analytics)

	registration := postJSON(t, server.handler, "/api/v1/auth/register", map[string]string{
		"name": "Welcome Integration", "email": emailAddress, "password": testPassword,
	})
	if registration.Code != http.StatusCreated {
		t.Fatalf("registration status = %d: %s", registration.Code, registration.Body.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := worker.Shutdown(ctx); err != nil {
		t.Fatalf("drain email worker: %v", err)
	}

	var deliveryStatus, recipient, eventType string
	var attempts int
	if err := server.pool.QueryRow(ctx, `
		SELECT status, recipient, event_type, attempts
		FROM email_deliveries
		WHERE recipient = $1
	`, emailAddress).Scan(&deliveryStatus, &recipient, &eventType, &attempts); err != nil {
		t.Fatalf("load welcome email delivery: %v", err)
	}
	if deliveryStatus != "sent" || recipient != emailAddress || eventType != "account.welcome" || attempts != 1 {
		t.Fatalf("welcome delivery = status %q, recipient %q, type %q, attempts %d", deliveryStatus, recipient, eventType, attempts)
	}
	var notificationCount int
	if err := server.pool.QueryRow(ctx, `
		SELECT count(*) FROM notifications WHERE user_id = (
			SELECT id FROM users WHERE email = $1
		) AND type = 'account.welcome'
	`, emailAddress).Scan(&notificationCount); err != nil {
		t.Fatalf("count welcome notifications: %v", err)
	}
	if notificationCount != 1 {
		t.Fatalf("welcome notifications = %d, want 1", notificationCount)
	}
}

func TestEmailProviderFailureIsPersistedWithoutLeakingProviderError(t *testing.T) {
	server := newAuthTestServer(t)
	emailAddress := "welcome-failure+" + testEmail()
	t.Cleanup(func() { cleanupUser(t, server, emailAddress) })

	worker := jobs.NewWorker(server.notifications, log.New(io.Discard, "", 0), 8, jobs.EmailDependencies{
		Store:  email.NewRepository(server.pool),
		Sender: integrationEmailSender{err: errors.New("private SMTP credential diagnostic")},
	})
	worker.Start()
	server.handler = routes.New(auth.NewService(server.pool, users.NewRepository(server.pool), server.organizations, server.tokens), server.tokens, users.NewRepository(server.pool), server.organizations,
		server.services, server.customers, server.bookings, server.technicians, server.invoices,
		server.notifications, worker, server.analytics)
	registration := postJSON(t, server.handler, "/api/v1/auth/register", map[string]string{
		"name": "Welcome Failure", "email": emailAddress, "password": testPassword,
	})
	if registration.Code != http.StatusCreated {
		t.Fatalf("registration status = %d: %s", registration.Code, registration.Body.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := worker.Shutdown(ctx); err != nil {
		t.Fatalf("drain failed email worker: %v", err)
	}
	var deliveryID, status, lastError string
	var attempts int
	if err := server.pool.QueryRow(ctx, `
		SELECT id::text, status, last_error, attempts
		FROM email_deliveries
		WHERE recipient = $1
	`, emailAddress).Scan(&deliveryID, &status, &lastError, &attempts); err != nil {
		t.Fatalf("load failed email delivery: %v", err)
	}
	if status != "queued" || lastError != "email delivery failed" || attempts != 1 {
		t.Fatalf("failed delivery = status %q, safe error %q, attempts %d", status, lastError, attempts)
	}
	if strings.Contains(lastError, "credential") {
		t.Fatalf("persisted delivery error exposed provider details: %q", lastError)
	}
	var retryDelay float64
	if err := server.pool.QueryRow(ctx, `
		SELECT EXTRACT(EPOCH FROM available_at - CURRENT_TIMESTAMP)
		FROM email_deliveries WHERE id = $1::uuid
	`, deliveryID).Scan(&retryDelay); err != nil {
		t.Fatalf("load first retry delay: %v", err)
	}
	if retryDelay < 20 || retryDelay > 35 {
		t.Fatalf("first retry delay = %.1f seconds, want approximately 30 seconds", retryDelay)
	}

	deliveryStore := email.NewRepository(server.pool)
	if _, err := server.pool.Exec(ctx, `
		UPDATE email_deliveries SET available_at = CURRENT_TIMESTAMP WHERE id = $1::uuid
	`, deliveryID); err != nil {
		t.Fatalf("make second delivery attempt available: %v", err)
	}
	delivery, found, err := deliveryStore.ClaimNext(ctx)
	if err != nil || !found || delivery.Attempts != 2 {
		t.Fatalf("claim second attempt = %+v, found %t, error %v", delivery, found, err)
	}
	if err := deliveryStore.MarkFailed(ctx, deliveryID, delivery.Attempts, true, "email delivery failed"); err != nil {
		t.Fatalf("schedule second retry: %v", err)
	}
	if err := server.pool.QueryRow(ctx, `
		SELECT EXTRACT(EPOCH FROM available_at - CURRENT_TIMESTAMP)
		FROM email_deliveries WHERE id = $1::uuid
	`, deliveryID).Scan(&retryDelay); err != nil {
		t.Fatalf("load second retry delay: %v", err)
	}
	if retryDelay < 110 || retryDelay > 125 {
		t.Fatalf("second retry delay = %.1f seconds, want approximately 120 seconds", retryDelay)
	}

	if _, err := server.pool.Exec(ctx, `
		UPDATE email_deliveries SET available_at = CURRENT_TIMESTAMP WHERE id = $1::uuid
	`, deliveryID); err != nil {
		t.Fatalf("make final delivery attempt available: %v", err)
	}
	delivery, found, err = deliveryStore.ClaimNext(ctx)
	if err != nil || !found || delivery.Attempts != 3 {
		t.Fatalf("claim final attempt = %+v, found %t, error %v", delivery, found, err)
	}
	if err := deliveryStore.MarkFailed(ctx, deliveryID, delivery.Attempts, true, "email delivery failed"); err != nil {
		t.Fatalf("record terminal delivery failure: %v", err)
	}
	if err := server.pool.QueryRow(ctx, `
		SELECT status FROM email_deliveries WHERE id = $1::uuid
	`, deliveryID).Scan(&status); err != nil {
		t.Fatalf("load terminal delivery status: %v", err)
	}
	if status != "failed" {
		t.Fatalf("terminal delivery status = %q, want failed", status)
	}
}
