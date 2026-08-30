package user

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type SMTPSettings struct {
	Host         string
	Port         int
	Username     string
	Password     string
	FromEmail    string
	FromName     string
	TLSMode      string
	AppPublicURL string
}

type SMTPInvitationMailer struct {
	settings SMTPSettings
	dialer   net.Dialer
}

func NewSMTPInvitationMailer(settings SMTPSettings) (InvitationMailer, error) {
	if settings.Host == "" {
		return DisabledInvitationMailer{}, nil
	}
	from, err := mail.ParseAddress(settings.FromEmail)
	if err != nil || from.Address != settings.FromEmail || containsNewline(settings.FromName) {
		return nil, errors.New("invalid SMTP sender")
	}
	appURL, err := url.Parse(settings.AppPublicURL)
	if err != nil || (appURL.Scheme != "http" && appURL.Scheme != "https") || appURL.Host == "" {
		return nil, errors.New("invalid public application URL")
	}
	if settings.TLSMode != "starttls" && settings.TLSMode != "tls" {
		return nil, errors.New("invalid SMTP TLS mode")
	}
	return &SMTPInvitationMailer{settings: settings, dialer: net.Dialer{Timeout: 15 * time.Second}}, nil
}

func (m *SMTPInvitationMailer) SendInvitation(ctx context.Context, message InvitationMessage) error {
	sender, err := mail.ParseAddress(m.settings.FromEmail)
	if err != nil || sender.Address != m.settings.FromEmail {
		return ErrInvitationDelivery
	}
	recipient, err := mail.ParseAddress(message.Email)
	if err != nil || recipient.Address != message.Email || containsNewline(message.DisplayName) || containsNewline(message.OrganizationName) {
		return ErrInvitationDelivery
	}
	invitationURL := strings.TrimRight(m.settings.AppPublicURL, "/") + "/invite/" + url.PathEscape(message.Token)
	subject := "Вас пригласили в Опору"
	body := invitationEmailHTML(message, invitationURL)
	raw := strings.Join([]string{
		"From: " + (&mail.Address{Name: m.settings.FromName, Address: m.settings.FromEmail}).String(),
		"To: " + (&mail.Address{Name: message.DisplayName, Address: message.Email}).String(),
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		body,
	}, "\r\n")

	address := net.JoinHostPort(m.settings.Host, strconv.Itoa(m.settings.Port))
	client, err := m.connect(ctx, address)
	if err != nil {
		return ErrInvitationDelivery
	}
	defer func() { _ = client.Close() }()
	if m.settings.Username != "" {
		if err = client.Auth(smtp.PlainAuth("", m.settings.Username, m.settings.Password, m.settings.Host)); err != nil {
			return ErrInvitationDelivery
		}
	}
	// #nosec G707 -- sender.Address is validated by net/mail and exact-match checked above.
	if err = client.Mail(sender.Address); err != nil {
		return ErrInvitationDelivery
	}
	if err = client.Rcpt(message.Email); err != nil {
		return ErrInvitationDelivery
	}
	writer, err := client.Data()
	if err != nil {
		return ErrInvitationDelivery
	}
	if _, err = writer.Write([]byte(raw)); err != nil {
		_ = writer.Close()
		return ErrInvitationDelivery
	}
	if err = writer.Close(); err != nil {
		return ErrInvitationDelivery
	}
	return client.Quit()
}

func (m *SMTPInvitationMailer) connect(ctx context.Context, address string) (*smtp.Client, error) {
	tlsConfig := &tls.Config{ServerName: m.settings.Host, MinVersion: tls.VersionTLS12}
	var connection net.Conn
	var err error
	if m.settings.TLSMode == "tls" {
		tlsDialer := tls.Dialer{NetDialer: &m.dialer, Config: tlsConfig}
		connection, err = tlsDialer.DialContext(ctx, "tcp", address)
	} else {
		connection, err = m.dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return nil, err
	}
	client, err := smtp.NewClient(connection, m.settings.Host)
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	if m.settings.TLSMode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			_ = client.Close()
			return nil, errors.New("SMTP server does not support STARTTLS")
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			_ = client.Close()
			return nil, err
		}
	}
	return client, nil
}

func invitationEmailHTML(message InvitationMessage, invitationURL string) string {
	return fmt.Sprintf(`<!doctype html><html lang="ru"><body style="margin:0;background:#f4f5f1;color:#1d2c27;font-family:Arial,sans-serif"><div style="max-width:560px;margin:32px auto;padding:32px;background:#fff;border:1px solid #dce2de;border-radius:14px"><h1 style="margin:0 0 18px;color:#176b55;font-size:24px">Вас пригласили в Опору</h1><p>Здравствуйте, %s.</p><p>Организация «%s» приглашает вас в защищённую систему сопровождения детей и работы специалистов.</p><p style="margin:28px 0"><a href="%s" style="display:inline-block;padding:12px 18px;border-radius:9px;background:#176b55;color:#fff;text-decoration:none;font-weight:bold">Принять приглашение</a></p><p style="color:#67736e;font-size:13px">Ссылка одноразовая и действует до %s. Если вы не ожидали приглашение, просто проигнорируйте письмо.</p></div></body></html>`,
		html.EscapeString(message.DisplayName), html.EscapeString(message.OrganizationName), html.EscapeString(invitationURL),
		html.EscapeString(message.ExpiresAt.Format("02.01.2006 15:04 MST")))
}

func containsNewline(value string) bool { return strings.ContainsAny(value, "\r\n") }
