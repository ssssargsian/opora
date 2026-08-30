package user

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
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
	logger   *slog.Logger
}

type SMTPFailureKind string

const (
	SMTPConnectionFailed     SMTPFailureKind = "smtp_connection_failed"
	SMTPTLSFailed            SMTPFailureKind = "smtp_tls_failed"
	SMTPAuthenticationFailed SMTPFailureKind = "smtp_authentication_failed"
	SMTPSenderRejected       SMTPFailureKind = "smtp_sender_rejected"
	SMTPRecipientRejected    SMTPFailureKind = "smtp_recipient_rejected"
	SMTPSendFailed           SMTPFailureKind = "smtp_send_failed"
)

type SMTPDeliveryError struct {
	Kind     SMTPFailureKind
	Stage    string
	Code     int
	Response string
}

func (e *SMTPDeliveryError) Error() string { return string(e.Kind) }
func (e *SMTPDeliveryError) Unwrap() error { return ErrInvitationDelivery }

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
	return &SMTPInvitationMailer{settings: settings, dialer: net.Dialer{Timeout: 15 * time.Second}, logger: slog.Default()}, nil
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
		return err
	}
	defer func() { _ = client.Close() }()
	if m.settings.Username != "" {
		if err = client.Auth(smtp.PlainAuth("", m.settings.Username, m.settings.Password, m.settings.Host)); err != nil {
			return m.failure(ctx, SMTPAuthenticationFailed, "auth", err)
		}
		m.success(ctx, "auth")
	}
	// #nosec G707 -- sender.Address is validated by net/mail and exact-match checked above.
	if err = client.Mail(sender.Address); err != nil {
		return m.failure(ctx, SMTPSenderRejected, "send", err)
	}
	if err = client.Rcpt(message.Email); err != nil {
		return m.failure(ctx, SMTPRecipientRejected, "send", err)
	}
	writer, err := client.Data()
	if err != nil {
		return m.failure(ctx, SMTPSendFailed, "send", err)
	}
	if _, err = writer.Write([]byte(raw)); err != nil {
		_ = writer.Close()
		return m.failure(ctx, SMTPSendFailed, "send", err)
	}
	if err = writer.Close(); err != nil {
		return m.failure(ctx, SMTPSendFailed, "send", err)
	}
	if err = client.Quit(); err != nil {
		return m.failure(ctx, SMTPSendFailed, "send", err)
	}
	m.success(ctx, "send")
	return nil
}

func (m *SMTPInvitationMailer) connect(ctx context.Context, address string) (*smtp.Client, error) {
	tlsConfig := &tls.Config{ServerName: m.settings.Host, MinVersion: tls.VersionTLS12}
	connection, err := m.dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, m.failure(ctx, SMTPConnectionFailed, "connect", err)
	}
	if err = connection.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		_ = connection.Close()
		return nil, m.failure(ctx, SMTPConnectionFailed, "connect", err)
	}
	m.success(ctx, "connect")
	if m.settings.TLSMode == "tls" {
		tlsConnection := tls.Client(connection, tlsConfig)
		if err = tlsConnection.HandshakeContext(ctx); err != nil {
			_ = connection.Close()
			return nil, m.failure(ctx, SMTPTLSFailed, "tls", err)
		}
		connection = tlsConnection
		m.success(ctx, "tls")
	}
	client, err := smtp.NewClient(connection, m.settings.Host)
	if err != nil {
		_ = connection.Close()
		return nil, m.failure(ctx, SMTPConnectionFailed, "connect", err)
	}
	if m.settings.TLSMode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			_ = client.Close()
			return nil, m.failure(ctx, SMTPTLSFailed, "tls", errors.New("SMTP server does not support STARTTLS"))
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			_ = client.Close()
			return nil, m.failure(ctx, SMTPTLSFailed, "tls", err)
		}
		m.success(ctx, "tls")
	}
	return client, nil
}

func (m *SMTPInvitationMailer) success(ctx context.Context, stage string) {
	m.logger.InfoContext(ctx, "SMTP stage completed", "smtp_stage", stage, "smtp_host", m.settings.Host, "smtp_port", m.settings.Port)
}

func (m *SMTPInvitationMailer) failure(ctx context.Context, kind SMTPFailureKind, stage string, err error) error {
	code, response := smtpResponse(err)
	m.logger.WarnContext(ctx, "SMTP stage failed", "smtp_stage", stage, "smtp_host", m.settings.Host, "smtp_port", m.settings.Port,
		"error_class", string(kind), "smtp_code", code, "smtp_response", response)
	return &SMTPDeliveryError{Kind: kind, Stage: stage, Code: code, Response: response}
}

func smtpResponse(err error) (int, string) {
	var protocolError *textproto.Error
	if errors.As(err, &protocolError) {
		return protocolError.Code, safeSMTPMessage(protocolError.Msg)
	}
	return 0, safeSMTPMessage(err.Error())
}

func safeSMTPMessage(value string) string {
	fields := strings.Fields(strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return ' '
		}
		return r
	}, value))
	for index, field := range fields {
		if strings.Contains(field, "@") {
			fields[index] = "[address]"
		}
	}
	result := strings.Join(fields, " ")
	if len(result) > 240 {
		result = result[:240]
	}
	return result
}

func invitationEmailHTML(message InvitationMessage, invitationURL string) string {
	return fmt.Sprintf(`<!doctype html><html lang="ru"><body style="margin:0;background:#f4f5f1;color:#1d2c27;font-family:Arial,sans-serif"><div style="max-width:560px;margin:32px auto;padding:32px;background:#fff;border:1px solid #dce2de;border-radius:14px"><h1 style="margin:0 0 18px;color:#176b55;font-size:24px">Вас пригласили в Опору</h1><p>Здравствуйте, %s.</p><p>Организация «%s» приглашает вас в защищённую систему сопровождения детей и работы специалистов.</p><p style="margin:28px 0"><a href="%s" style="display:inline-block;padding:12px 18px;border-radius:9px;background:#176b55;color:#fff;text-decoration:none;font-weight:bold">Принять приглашение</a></p><p style="color:#67736e;font-size:13px">Ссылка одноразовая и действует до %s. Если вы не ожидали приглашение, просто проигнорируйте письмо.</p></div></body></html>`,
		html.EscapeString(message.DisplayName), html.EscapeString(message.OrganizationName), html.EscapeString(invitationURL),
		html.EscapeString(message.ExpiresAt.Format("02.01.2006 15:04 MST")))
}

func containsNewline(value string) bool { return strings.ContainsAny(value, "\r\n") }
