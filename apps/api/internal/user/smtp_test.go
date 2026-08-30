package user

import (
	"errors"
	"net/textproto"
	"testing"
)

func TestSMTPErrorClassificationAndRedaction(t *testing.T) {
	code, response := smtpResponse(&textproto.Error{Code: 535, Msg: "Authentication failed for user@example.test"})
	if code != 535 || response != "Authentication failed for [address]" {
		t.Fatalf("smtpResponse()=%d,%q", code, response)
	}
	err := &SMTPDeliveryError{Kind: SMTPAuthenticationFailed, Stage: "auth", Code: code, Response: response}
	if !errors.Is(err, ErrInvitationDelivery) || smtpPublicCode(err) != string(SMTPAuthenticationFailed) {
		t.Fatalf("unexpected typed delivery error: %v", err)
	}
	publicCode, publicMessage := smtpPublicError(err)
	if publicCode != "smtp_authentication_failed" || publicMessage != "SMTP authentication failed" {
		t.Fatalf("smtpPublicError()=%q,%q", publicCode, publicMessage)
	}
}
