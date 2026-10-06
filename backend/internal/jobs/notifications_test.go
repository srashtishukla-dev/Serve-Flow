package jobs

import (
	"context"
	"errors"
	"io"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/serveflow/serveflow/backend/internal/email"
	"github.com/serveflow/serveflow/backend/internal/notifications"
)

type recordingCreator struct {
	mu        sync.Mutex
	calls     int
	firstErr  error
	processed chan notifications.Input
}

func (creator *recordingCreator) Create(_ context.Context, _, _ string, input notifications.Input) (notifications.Notification, error) {
	creator.mu.Lock()
	creator.calls++
	call := creator.calls
	creator.mu.Unlock()
	if call == 1 && creator.firstErr != nil {
		return notifications.Notification{}, creator.firstErr
	}
	creator.processed <- input
	return notifications.Notification{Type: input.Type, Title: input.Title, Message: input.Message}, nil
}

func TestWorkerProcessesJobsAndContinuesAfterError(t *testing.T) {
	creator := &recordingCreator{
		firstErr:  errors.New("database temporarily unavailable"),
		processed: make(chan notifications.Input, 1),
	}
	var logOutput strings.Builder
	worker := NewWorker(creator, log.New(&logOutput, "", 0), 2)
	worker.Start()
	if !worker.Enqueue(NotificationJob{OrganizationID: "org-a", UserID: "user-a", Type: "first"}) {
		t.Fatal("first job was not accepted")
	}
	if !worker.Enqueue(NotificationJob{OrganizationID: "org-a", UserID: "user-a", Type: "appointment.created", Title: "Appointment created", Message: "An appointment was scheduled."}) {
		t.Fatal("second job was not accepted")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.Shutdown(ctx); err != nil {
		t.Fatalf("graceful worker shutdown: %v", err)
	}
	select {
	case notification := <-creator.processed:
		if notification.Type != "appointment.created" {
			t.Fatalf("processed notification type = %q", notification.Type)
		}
	default:
		t.Fatal("worker did not process the job after the failed job")
	}
	creator.mu.Lock()
	calls := creator.calls
	creator.mu.Unlock()
	if calls != 2 {
		t.Fatalf("jobs processed = %d, want 2", calls)
	}
	if !strings.Contains(logOutput.String(), "notification job failed type=first") {
		t.Fatalf("worker did not safely log the failed job: %s", logOutput.String())
	}
}

type memoryDeliveryStore struct {
	delivery email.Delivery
	claimed  bool
	recover  bool
	sent     bool
	failed   bool
	retry    bool
	safeErr  string
}

func (store *memoryDeliveryStore) Queue(_ context.Context, organizationID, userID, eventType, title, content string) (email.Delivery, error) {
	store.delivery = email.Delivery{
		ID: "delivery-id", OrganizationID: organizationID, UserID: userID,
		Recipient: "owner@example.test", Type: eventType, Title: title, Content: content,
	}
	return store.delivery, nil
}

func (store *memoryDeliveryStore) RecoverStale(context.Context) error {
	store.recover = true
	return nil
}

func (store *memoryDeliveryStore) ClaimNext(context.Context) (email.Delivery, bool, error) {
	if store.claimed {
		return email.Delivery{}, false, nil
	}
	store.claimed = true
	store.delivery.Attempts++
	return store.delivery, true, nil
}

func (store *memoryDeliveryStore) MarkSent(context.Context, string) error {
	store.sent = true
	return nil
}

func (store *memoryDeliveryStore) MarkFailed(_ context.Context, _ string, _ int, retry bool, safeError string) error {
	store.failed = true
	store.retry = retry
	store.safeErr = safeError
	return nil
}

type recordingEmailSender struct {
	message email.Message
	err     error
}

func (sender *recordingEmailSender) Send(_ context.Context, message email.Message) error {
	sender.message = message
	return sender.err
}

func TestWorkerQueuesAndSendsEachExistingEmailEvent(t *testing.T) {
	for _, eventType := range []string{"account.welcome", "appointment.created", "invoice.created", "payment.recorded"} {
		t.Run(eventType, func(t *testing.T) {
			store := &memoryDeliveryStore{}
			sender := &recordingEmailSender{}
			worker := NewWorker(nil, log.New(io.Discard, "", 0), 1, EmailDependencies{Store: store, Sender: sender})
			worker.Start()
			if !worker.Enqueue(NotificationJob{
				OrganizationID: "org-id", UserID: "user-id", Type: eventType,
				Title: "ServeFlow update", Message: "Your update is ready.",
			}) {
				t.Fatal("event was not queued")
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := worker.Shutdown(ctx); err != nil {
				t.Fatalf("worker shutdown: %v", err)
			}
			if !store.recover || !store.sent {
				t.Fatalf("delivery recovery/sent = %t/%t", store.recover, store.sent)
			}
			if sender.message.To != "owner@example.test" || sender.message.Type != eventType {
				t.Fatalf("sent message = %+v", sender.message)
			}
		})
	}
}

func TestWorkerPersistsProviderFailureWithoutLoggingCredentials(t *testing.T) {
	store := &memoryDeliveryStore{}
	sender := &recordingEmailSender{err: errors.New("SMTP_AUTH_SECRET must never reach logs")}
	var logOutput strings.Builder
	worker := NewWorker(nil, log.New(&logOutput, "", 0), 1, EmailDependencies{Store: store, Sender: sender})
	worker.Start()
	if !worker.Enqueue(NotificationJob{
		OrganizationID: "org-id", UserID: "user-id", Type: "account.welcome",
		Title: "Welcome to ServeFlow", Message: "Your account is ready.",
	}) {
		t.Fatal("event was not queued")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.Shutdown(ctx); err != nil {
		t.Fatalf("worker shutdown: %v", err)
	}
	if !store.failed || !store.retry || store.safeErr != "email delivery failed" {
		t.Fatalf("failed delivery record = %+v", store)
	}
	if strings.Contains(logOutput.String(), "SMTP_AUTH_SECRET") {
		t.Fatalf("worker log exposed provider details: %s", logOutput.String())
	}
}
