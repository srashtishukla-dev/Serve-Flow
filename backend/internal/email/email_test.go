package email

import (
	"errors"
	"strings"
	"testing"
)

func TestBuildMessageSupportsExistingTransactionalEvents(t *testing.T) {
	events := []string{"account.welcome", "appointment.created", "invoice.created", "payment.recorded"}
	for _, event := range events {
		t.Run(event, func(t *testing.T) {
			message, err := BuildMessage("owner@example.test", event, "ServeFlow update", "Your update is ready.")
			if err != nil {
				t.Fatalf("BuildMessage() error = %v", err)
			}
			if message.To != "owner@example.test" || message.Type != event || message.Title != "ServeFlow update" {
				t.Fatalf("BuildMessage() = %+v", message)
			}
		})
	}
}

func TestBuildMessageRejectsInvalidRecipientAndHeaderInjection(t *testing.T) {
	for _, testCase := range []struct {
		name, recipient, title string
	}{
		{name: "invalid address", recipient: "not-an-email", title: "Update"},
		{name: "display name", recipient: "Owner <owner@example.test>", title: "Update"},
		{name: "header injection", recipient: "owner@example.test", title: "Update\r\nBcc: attacker@example.test"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := BuildMessage(testCase.recipient, "account.welcome", testCase.title, "content"); !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("BuildMessage() error = %v, want ErrInvalidMessage", err)
			}
		})
	}
}

func TestDisabledSMTPDoesNotReportDeliverySuccess(t *testing.T) {
	if _, err := NewSMTP(Settings{Provider: "disabled"}); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("NewSMTP() error = %v, want ErrNotConfigured", err)
	}
	var sender *SMTP
	if err := sender.Send(t.Context(), Message{To: "owner@example.test", Title: "Update"}); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("Send() error = %v, want ErrNotConfigured", err)
	}
}

func TestLoadSettingsRequiresProductionSMTPAndPairedCredentials(t *testing.T) {
	t.Setenv("EMAIL_PROVIDER", "disabled")
	if _, err := LoadSettings("production"); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("production without SMTP error = %v, want ErrInvalidSettings", err)
	}

	t.Setenv("EMAIL_PROVIDER", "smtp")
	t.Setenv("SMTP_HOST", "smtp.example.test")
	t.Setenv("EMAIL_FROM", "ServeFlow <no-reply@example.test>")
	t.Setenv("EMAIL_FROM_NAME", "ServeFlow")
	t.Setenv("SMTP_PORT", "")
	smtpUsername := "smtp-user-do-not-log"
	t.Setenv("SMTP_USERNAME", smtpUsername)
	t.Setenv("SMTP_PASSWORD", "")
	settings, err := LoadSettings("production")
	if !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("unpaired SMTP credentials error = %v, want ErrInvalidSettings", err)
	}
	if strings.Contains(err.Error(), smtpUsername) {
		t.Fatalf("configuration error exposed credential information: %v", err)
	}

	t.Setenv("SMTP_USERNAME", "")
	t.Setenv("SMTP_PASSWORD", "")
	settings, err = LoadSettings("production")
	if err != nil {
		t.Fatalf("valid unauthenticated STARTTLS configuration: %v", err)
	}
	if settings.Provider != "smtp" || settings.SMTPPort != 587 || settings.From != "ServeFlow <no-reply@example.test>" {
		t.Fatalf("SMTP settings = %+v", settings)
	}
}

func TestLoadSettingsRejectsInvalidSMTPValues(t *testing.T) {
	t.Setenv("EMAIL_PROVIDER", "smtp")
	t.Setenv("EMAIL_FROM", "no-reply@example.test")
	t.Setenv("EMAIL_FROM_NAME", "ServeFlow")
	t.Setenv("SMTP_HOST", "smtp.example.test")
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("SMTP_USERNAME", "")
	t.Setenv("SMTP_PASSWORD", "")

	for _, testCase := range []struct {
		name  string
		key   string
		value string
	}{
		{name: "host with whitespace", key: "SMTP_HOST", value: "smtp example.test"},
		{name: "invalid sender", key: "EMAIL_FROM", value: "not-an-email"},
		{name: "invalid port", key: "SMTP_PORT", value: "not-a-port"},
		{name: "zero port", key: "SMTP_PORT", value: "0"},
		{name: "out of range port", key: "SMTP_PORT", value: "65536"},
		{name: "newline in display name", key: "EMAIL_FROM_NAME", value: "ServeFlow\r\nBcc: attacker@example.test"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(testCase.key, testCase.value)
			if _, err := LoadSettings("development"); !errors.Is(err, ErrInvalidSettings) {
				t.Fatalf("LoadSettings() error = %v, want ErrInvalidSettings", err)
			}
		})
	}
}

func TestSMTPUsesImplicitTLSOnlyOnPort465(t *testing.T) {
	for port, want := range map[uint16]bool{25: false, 465: true, 587: false, 2525: false} {
		if got := usesImplicitTLS(port); got != want {
			t.Errorf("usesImplicitTLS(%d) = %t, want %t", port, got, want)
		}
	}
}
