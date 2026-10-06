package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	ErrNotConfigured   = errors.New("SMTP email delivery is not configured")
	ErrInvalidSettings = errors.New("invalid SMTP email configuration")
	ErrInvalidMessage  = errors.New("invalid email message")
)

type Settings struct {
	Provider string
	From     string
	FromName string
	SMTPHost string
	SMTPPort uint16
	Username string
	Password string
}

type Message struct {
	To      string
	Type    string
	Title   string
	Content string
}

type Sender interface {
	Send(context.Context, Message) error
}

type Delivery struct {
	ID             string
	OrganizationID string
	UserID         string
	Recipient      string
	Type           string
	Title          string
	Content        string
	Attempts       int
}

type DeliveryStore interface {
	Queue(context.Context, string, string, string, string, string) (Delivery, error)
	RecoverStale(context.Context) error
	ClaimNext(context.Context) (Delivery, bool, error)
	MarkSent(context.Context, string) error
	MarkFailed(context.Context, string, int, bool, string) error
}

type SMTP struct {
	settings Settings
}

func LoadSettings(appEnvironment string) (Settings, error) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("EMAIL_PROVIDER")))
	if provider == "" {
		provider = "disabled"
	}
	settings := Settings{
		Provider: provider,
		From:     strings.TrimSpace(os.Getenv("EMAIL_FROM")),
		FromName: strings.TrimSpace(os.Getenv("EMAIL_FROM_NAME")),
		SMTPHost: strings.TrimSpace(os.Getenv("SMTP_HOST")),
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
	}
	switch provider {
	case "disabled":
		if strings.EqualFold(strings.TrimSpace(appEnvironment), "production") {
			return Settings{}, fmt.Errorf("%w: EMAIL_PROVIDER=smtp is required in production", ErrInvalidSettings)
		}
		return settings, nil
	case "smtp":
		if settings.SMTPHost == "" || settings.From == "" {
			return Settings{}, fmt.Errorf("%w: SMTP_HOST and EMAIL_FROM are required", ErrInvalidSettings)
		}
		if strings.ContainsAny(settings.SMTPHost, " \t\r\n/") {
			return Settings{}, fmt.Errorf("%w: SMTP_HOST must be a hostname or IP address", ErrInvalidSettings)
		}
		if _, err := mail.ParseAddress(settings.From); err != nil {
			return Settings{}, fmt.Errorf("%w: EMAIL_FROM must be a valid email address", ErrInvalidSettings)
		}
		if settings.FromName == "" {
			settings.FromName = "ServeFlow"
		}
		if strings.ContainsAny(settings.FromName, "\r\n") {
			return Settings{}, fmt.Errorf("%w: EMAIL_FROM_NAME must not contain newlines", ErrInvalidSettings)
		}
		port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
		if port == "" {
			port = "587"
		}
		value, err := strconv.ParseUint(port, 10, 16)
		if err != nil || value == 0 {
			return Settings{}, fmt.Errorf("%w: SMTP_PORT must be a valid port", ErrInvalidSettings)
		}
		settings.SMTPPort = uint16(value)
		if (settings.Username == "") != (settings.Password == "") {
			return Settings{}, fmt.Errorf("%w: SMTP_USERNAME and SMTP_PASSWORD must be set together", ErrInvalidSettings)
		}
		return settings, nil
	default:
		return Settings{}, fmt.Errorf("%w: EMAIL_PROVIDER must be smtp or disabled", ErrInvalidSettings)
	}
}

func NewSMTP(settings Settings) (*SMTP, error) {
	if settings.Provider != "smtp" || settings.SMTPHost == "" || settings.SMTPPort == 0 {
		return nil, ErrNotConfigured
	}
	if _, err := mail.ParseAddress(settings.From); err != nil {
		return nil, fmt.Errorf("%w: sender address is invalid", ErrInvalidSettings)
	}
	if strings.ContainsAny(settings.FromName, "\r\n") {
		return nil, fmt.Errorf("%w: sender display name is invalid", ErrInvalidSettings)
	}
	return &SMTP{settings: settings}, nil
}

func BuildMessage(recipient, eventType, title, content string) (Message, error) {
	address, err := mail.ParseAddress(strings.TrimSpace(recipient))
	if err != nil || address.Address != strings.TrimSpace(recipient) ||
		strings.ContainsAny(title, "\r\n") || strings.TrimSpace(title) == "" {
		return Message{}, ErrInvalidMessage
	}
	return Message{
		To:      address.Address,
		Type:    strings.TrimSpace(eventType),
		Title:   strings.TrimSpace(title),
		Content: strings.TrimSpace(content),
	}, nil
}

func (sender *SMTP) Send(ctx context.Context, message Message) error {
	if sender == nil || sender.settings.SMTPHost == "" {
		return ErrNotConfigured
	}
	address, err := mail.ParseAddress(strings.TrimSpace(message.To))
	if err != nil || address.Address != strings.TrimSpace(message.To) ||
		strings.ContainsAny(message.Title, "\r\n") || strings.TrimSpace(message.Title) == "" {
		return ErrInvalidMessage
	}

	deadline := time.Now().Add(15 * time.Second)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	host := sender.settings.SMTPHost
	dialer := net.Dialer{Timeout: time.Until(deadline)}
	connection, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(int(sender.settings.SMTPPort))))
	if err != nil {
		return errors.New("SMTP connection failed")
	}
	if err := connection.SetDeadline(deadline); err != nil {
		_ = connection.Close()
		return errors.New("SMTP connection deadline failed")
	}
	defer connection.Close()

	if usesImplicitTLS(sender.settings.SMTPPort) {
		tlsConnection := tls.Client(connection, &tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		})
		if err := tlsConnection.HandshakeContext(ctx); err != nil {
			return errors.New("SMTP TLS handshake failed")
		}
		connection = tlsConnection
	}

	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return errors.New("SMTP handshake failed")
	}
	defer client.Close()
	if err := client.Hello("localhost"); err != nil {
		return errors.New("SMTP greeting failed")
	}
	if !usesImplicitTLS(sender.settings.SMTPPort) {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("SMTP server does not offer required STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return errors.New("SMTP STARTTLS failed")
		}
	}
	if sender.settings.Username != "" {
		if ok, _ := client.Extension("AUTH"); !ok {
			return errors.New("SMTP server does not offer authentication")
		}
		auth := smtp.PlainAuth("", sender.settings.Username, sender.settings.Password, host)
		if err := client.Auth(auth); err != nil {
			return errors.New("SMTP authentication failed")
		}
	}

	fromAddress, err := mail.ParseAddress(sender.settings.From)
	if err != nil {
		return ErrInvalidSettings
	}
	if err := client.Mail(fromAddress.Address); err != nil {
		return errors.New("SMTP sender was rejected")
	}
	if err := client.Rcpt(address.Address); err != nil {
		return errors.New("SMTP recipient was rejected")
	}
	data, err := client.Data()
	if err != nil {
		return errors.New("SMTP message was rejected")
	}
	from := (&mail.Address{Name: sender.settings.FromName, Address: fromAddress.Address}).String()
	_, writeErr := fmt.Fprintf(data, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s\r\n",
		from, address.String(), mime.QEncoding.Encode("UTF-8", message.Title), strings.ReplaceAll(message.Content, "\n", "\r\n"))
	closeErr := data.Close()
	if writeErr != nil || closeErr != nil {
		return errors.New("SMTP message transmission failed")
	}
	if err := client.Quit(); err != nil {
		return errors.New("SMTP delivery confirmation failed")
	}
	return nil
}

func usesImplicitTLS(port uint16) bool {
	return port == 465
}
