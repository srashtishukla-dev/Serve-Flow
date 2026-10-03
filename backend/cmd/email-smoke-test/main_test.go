package main

import (
	"strings"
	"testing"
)

func TestRunRequiresControlledRecipient(t *testing.T) {
	t.Setenv("SERVEFLOW_EMAIL_SMOKE_TEST_TO", "")
	t.Setenv("EMAIL_PROVIDER", "disabled")
	if err := run(); err == nil || !strings.Contains(err.Error(), "SERVEFLOW_EMAIL_SMOKE_TEST_TO") {
		t.Fatalf("run() error = %v, want controlled-recipient error", err)
	}
}

func TestRunRejectsInvalidRecipientBeforeProviderSetup(t *testing.T) {
	t.Setenv("SERVEFLOW_EMAIL_SMOKE_TEST_TO", "not-an-email")
	t.Setenv("EMAIL_PROVIDER", "disabled")
	if err := run(); err == nil || !strings.Contains(err.Error(), "must be a valid email address") {
		t.Fatalf("run() error = %v, want invalid-recipient error", err)
	}
}
