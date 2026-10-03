package jobs

import (
	"context"
	"io"
	"log"
	"sync"
	"time"

	"github.com/serveflow/serveflow/backend/internal/email"
	"github.com/serveflow/serveflow/backend/internal/notifications"
)

type NotificationJob struct {
	OrganizationID string
	UserID         string
	Type           string
	Title          string
	Message        string
}

type NotificationCreator interface {
	Create(context.Context, string, string, notifications.Input) (notifications.Notification, error)
}

type EmailDependencies struct {
	Store  email.DeliveryStore
	Sender email.Sender
}

type Worker struct {
	creator     NotificationCreator
	logger      *log.Logger
	queue       chan NotificationJob
	done        chan struct{}
	wake        chan struct{}
	emailStore  email.DeliveryStore
	emailSender email.Sender
	mu          sync.RWMutex
	started     bool
	stopped     bool
}

func NewWorker(creator NotificationCreator, logger *log.Logger, queueCapacity int, emailDependencies ...EmailDependencies) *Worker {
	if queueCapacity < 1 {
		queueCapacity = 1
	}
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	worker := &Worker{
		creator: creator,
		logger:  logger,
		queue:   make(chan NotificationJob, queueCapacity),
		done:    make(chan struct{}),
		wake:    make(chan struct{}, 1),
	}
	if len(emailDependencies) > 0 {
		worker.emailStore = emailDependencies[0].Store
		worker.emailSender = emailDependencies[0].Sender
	}
	return worker
}

func (worker *Worker) Start() {
	worker.mu.Lock()
	if worker.started || worker.stopped {
		worker.mu.Unlock()
		return
	}
	worker.started = true
	worker.mu.Unlock()
	if worker.emailStore != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := worker.emailStore.RecoverStale(ctx); err != nil {
			worker.logger.Printf("email delivery recovery failed")
		}
		cancel()
	}
	go worker.run()
}

func (worker *Worker) Enqueue(job NotificationJob) bool {
	if worker.emailStore != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, err := worker.emailStore.Queue(ctx, job.OrganizationID, job.UserID, job.Type, job.Title, job.Message)
		cancel()
		if err != nil {
			worker.logger.Printf("notification and email job could not be queued type=%s", job.Type)
			return false
		}
		select {
		case worker.wake <- struct{}{}:
		default:
		}
		return true
	}
	worker.mu.RLock()
	if !worker.started || worker.stopped {
		worker.mu.RUnlock()
		return false
	}
	accepted := false
	select {
	case worker.queue <- job:
		accepted = true
	default:
	}
	worker.mu.RUnlock()
	if !accepted {
		worker.logger.Printf("notification job queue full; dropping type=%s", job.Type)
	}
	return accepted
}

func (worker *Worker) Shutdown(ctx context.Context) error {
	worker.mu.Lock()
	if !worker.stopped {
		worker.stopped = true
		close(worker.queue)
		if !worker.started {
			close(worker.done)
		}
	}
	worker.mu.Unlock()

	select {
	case <-worker.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (worker *Worker) run() {
	defer close(worker.done)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case job, ok := <-worker.queue:
			if !ok {
				worker.processDeliveries()
				return
			}
			worker.process(job)
			worker.processDeliveries()
		case <-worker.wake:
			worker.processDeliveries()
		case <-ticker.C:
			worker.processDeliveries()
		}
	}
}

func (worker *Worker) processDeliveries() {
	if worker.emailStore == nil {
		return
	}
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		delivery, found, err := worker.emailStore.ClaimNext(ctx)
		cancel()
		if err != nil {
			worker.logger.Printf("email delivery queue read failed")
			return
		}
		if !found {
			return
		}
		message, err := email.BuildMessage(delivery.Recipient, delivery.Type, delivery.Title, delivery.Content)
		if err == nil {
			if worker.emailSender == nil {
				err = email.ErrNotConfigured
			} else {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				err = worker.emailSender.Send(ctx, message)
				cancel()
			}
		}
		ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
		if err == nil {
			if markErr := worker.emailStore.MarkSent(ctx, delivery.ID); markErr != nil {
				worker.logger.Printf("email delivery status update failed id=%s", delivery.ID)
			}
		} else {
			if markErr := worker.emailStore.MarkFailed(ctx, delivery.ID, delivery.Attempts, true, "email delivery failed"); markErr != nil {
				worker.logger.Printf("email delivery failure status update failed id=%s", delivery.ID)
			}
			worker.logger.Printf("email delivery failed id=%s type=%s attempt=%d", delivery.ID, delivery.Type, delivery.Attempts)
		}
		cancel()
	}
}

func (worker *Worker) process(job NotificationJob) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := worker.creator.Create(ctx, job.OrganizationID, job.UserID, notifications.Input{
		Type: job.Type, Title: job.Title, Message: job.Message,
	})
	if err != nil {
		worker.logger.Printf("notification job failed type=%s: %v", job.Type, err)
	}
}
