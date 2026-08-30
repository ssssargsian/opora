package document

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"opora.local/api/internal/config"
)

func TestOnlyOfficeTrustedDownloadURL(t *testing.T) {
	service := &OnlyOfficeService{config: config.OnlyOffice{InternalURL: "http://onlyoffice", CallbackOrigin: "https://office.opora.example"}}
	trusted, err := service.trustedDownloadURL("https://office.opora.example/cache/files/data.docx?token=secret")
	if err != nil || trusted != "http://onlyoffice/cache/files/data.docx?token=secret" {
		t.Fatalf("trustedDownloadURL() = %q, %v", trusted, err)
	}
	if _, err = service.trustedDownloadURL("https://attacker.example/data.docx"); !errors.Is(err, ErrInvalidOnlyOfficeToken) {
		t.Fatalf("untrusted origin error=%v", err)
	}
}

func TestOnlyOfficeCallbackJWTValidation(t *testing.T) {
	secret := "test-onlyoffice-secret-with-at-least-32-characters"
	documentID := uuid.New()
	token, err := signJWT(secret, map[string]any{"purpose": "onlyoffice_callback", "organizationId": uuid.New().String(),
		"documentId": documentID.String(), "versionId": uuid.New().String(), "actorId": uuid.New().String(), "exp": time.Now().Add(time.Minute).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	service := &OnlyOfficeService{config: config.OnlyOffice{JWTSecret: secret}}
	if _, err = service.VerifyCallbackToken(token, documentID); err != nil {
		t.Fatalf("valid callback token rejected: %v", err)
	}
	if _, err = service.VerifyCallbackToken(token+"x", documentID); !errors.Is(err, ErrInvalidOnlyOfficeToken) {
		t.Fatalf("invalid callback token error=%v", err)
	}
}
