package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"hrm/internal/domain/notifications"
	"hrm/internal/platform/config"
)

type noopMailer struct{}

func (noopMailer) Send(ctx context.Context, from, to, subject, body string) error {
	return nil
}

type smtpMailer struct {
	cfg config.Config
}

func New(cfg config.Config) notifications.Mailer {
	if !cfg.EmailEnabled || cfg.SMTPHost == "" {
		return noopMailer{}
	}
	return &smtpMailer{cfg: cfg}
}

func (s *smtpMailer) Send(ctx context.Context, from, to, subject, body string) error {
	if strings.TrimSpace(to) == "" {
		return nil
	}
	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)
	msg := buildMessage(from, to, subject, body)

	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.cfg.SMTPHost)
	if err != nil {
		return err
	}
	defer client.Close()

	if s.cfg.SMTPUseTLS {
		tlsConfig := &tls.Config{ServerName: s.cfg.SMTPHost}
		if err := client.StartTLS(tlsConfig); err != nil {
			return err
		}
	}

	if s.cfg.SMTPUser != "" {
		auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPassword, s.cfg.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return err
		}
	}

	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func buildMessage(from, to, subject, body string) []byte {
	headers := []string{
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=\"UTF-8\"",
		"",
	}
	return []byte(strings.Join(headers, "\r\n") + "\r\n" + body)
}
