package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/serveflow/serveflow/backend/internal/email"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	recipient := strings.TrimSpace(os.Getenv("SERVEFLOW_EMAIL_SMOKE_TEST_TO"))
	if recipient == "" {
		return errors.New("set SERVEFLOW_EMAIL_SMOKE_TEST_TO to an inbox you control")
	}
	message, err := email.BuildMessage(
		recipient,
		"account.welcome",
		"ServeFlow email delivery test",
		"This is a controlled ServeFlow SMTP smoke test. No account or business data is included.",
	)
	if err != nil {
		return errors.New("SERVEFLOW_EMAIL_SMOKE_TEST_TO must be a valid email address")
	}
	settings, err := email.LoadSettings(os.Getenv("APP_ENV"))
	if err != nil {
		return err
	}
	sender, err := email.NewSMTP(settings)
	if err != nil {
		return errors.New("SMTP email delivery is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := sender.Send(ctx, message); err != nil {
		return fmt.Errorf("SMTP smoke test failed: %w", err)
	}
	fmt.Println("SMTP accepted the controlled smoke-test message. Confirm receipt in the configured inbox; acceptance does not prove inbox delivery.")
	return nil
}
