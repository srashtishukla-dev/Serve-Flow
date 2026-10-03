package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/serveflow/serveflow/backend/internal/analytics"
	"github.com/serveflow/serveflow/backend/internal/auth"
	"github.com/serveflow/serveflow/backend/internal/bookings"
	"github.com/serveflow/serveflow/backend/internal/cache"
	"github.com/serveflow/serveflow/backend/internal/config"
	"github.com/serveflow/serveflow/backend/internal/customers"
	"github.com/serveflow/serveflow/backend/internal/database"
	"github.com/serveflow/serveflow/backend/internal/email"
	"github.com/serveflow/serveflow/backend/internal/handler"
	"github.com/serveflow/serveflow/backend/internal/invoices"
	"github.com/serveflow/serveflow/backend/internal/jobs"
	"github.com/serveflow/serveflow/backend/internal/middleware"
	"github.com/serveflow/serveflow/backend/internal/notifications"
	"github.com/serveflow/serveflow/backend/internal/organizations"
	"github.com/serveflow/serveflow/backend/internal/routes"
	"github.com/serveflow/serveflow/backend/internal/services"
	"github.com/serveflow/serveflow/backend/internal/technicians"
	"github.com/serveflow/serveflow/backend/internal/users"
)

func main() {
	logger := log.New(os.Stdout, "serveflow-api ", log.LstdFlags|log.LUTC)
	if err := run(logger); err != nil {
		logger.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}

func run(logger *log.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	databaseContext, cancelDatabaseContext := context.WithTimeout(context.Background(), 5*time.Second)
	pool, err := database.Open(databaseContext, cfg.Database)
	cancelDatabaseContext()
	if err != nil {
		return err
	}
	defer func() {
		pool.Close()
		logger.Println("PostgreSQL connection pool closed")
	}()
	logger.Println("connected to PostgreSQL")
	redisClient := cache.NewClient(cfg.Redis, logger)
	redisContext, cancelRedisContext := context.WithTimeout(context.Background(), time.Second)
	if err := redisClient.Connect(redisContext); err != nil {
		logger.Printf("Redis cache unavailable; continuing with PostgreSQL: %v", err)
	} else {
		logger.Println("connected to Redis cache")
	}
	cancelRedisContext()
	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.Printf("close Redis cache client: %v", err)
		}
	}()
	userRepository := users.NewRepository(pool)
	organizationRepository := organizations.NewRepository(pool)
	serviceRepository := services.NewRepository(pool, redisClient)
	customerRepository := customers.NewRepository(pool)
	bookingRepository := bookings.NewRepository(pool)
	technicianRepository := technicians.NewRepository(pool)
	invoiceRepository := invoices.NewRepository(pool)
	notificationRepository := notifications.NewRepository(pool)
	analyticsRepository := analytics.NewRepository(pool)
	var emailSender email.Sender
	if cfg.Email.Provider == "smtp" {
		emailSender, err = email.NewSMTP(cfg.Email)
		if err != nil {
			return fmt.Errorf("configure SMTP email delivery: %w", err)
		}
		logger.Println("SMTP email delivery configured")
	} else {
		logger.Println("SMTP email delivery is disabled; queued email delivery will be recorded as failed")
	}
	tokenManager, err := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTAccessTokenTTL)
	if err != nil {
		return fmt.Errorf("configure JWT: %w", err)
	}
	authService := auth.NewService(pool, userRepository, organizationRepository, tokenManager)
	notificationWorker := jobs.NewWorker(notificationRepository, logger, 100, jobs.EmailDependencies{
		Store:  email.NewRepository(pool),
		Sender: emailSender,
	})
	notificationWorker.Start()
	defer func() {
		workerContext, cancelWorkerContext := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelWorkerContext()
		if err := notificationWorker.Shutdown(workerContext); err != nil {
			logger.Printf("notification worker shutdown: %v", err)
		}
	}()

	server := &http.Server{
		Addr: ":" + cfg.HTTPPort,
		Handler: middleware.RequestLogging(logger)(middleware.SecurityHeaders(middleware.CORS(cfg.CORSAllowedOrigin)(
			routes.New(authService, tokenManager, userRepository, organizationRepository, serviceRepository, customerRepository, bookingRepository, technicianRepository, invoiceRepository, notificationRepository, notificationWorker, analyticsRepository, handler.HealthWithDependencies(pool, redisClient)),
		))),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Printf("HTTP server listening on :%s", cfg.HTTPPort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	shutdownSignal, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	select {
	case <-shutdownSignal.Done():
		logger.Println("shutdown signal received")
	case err := <-serverErrors:
		return fmt.Errorf("HTTP server: %w", err)
	}

	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("graceful HTTP shutdown: %w", err)
	}
	logger.Println("HTTP server stopped")
	return nil
}
