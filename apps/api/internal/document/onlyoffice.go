package document

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"opora.local/api/internal/access"
	"opora.local/api/internal/config"
)

var (
	ErrInvalidOnlyOfficeToken = errors.New("invalid ONLYOFFICE token")
	ErrUntrustedDownloadURL   = errors.New("untrusted ONLYOFFICE download URL")
	ErrOnlyOfficeDownload     = errors.New("ONLYOFFICE download failed")
)

type OnlyOfficeService struct {
	documents *Service
	actors    ActorLoader
	config    config.OnlyOffice
	maxBytes  int64
	client    *http.Client
}

type ActorLoader interface {
	Actor(context.Context, uuid.UUID, uuid.UUID) (access.Actor, error)
}

type EditorConfig struct {
	DocumentServerURL string         `json:"documentServerUrl"`
	Config            map[string]any `json:"config"`
}

type callbackClaims struct {
	OrganizationID uuid.UUID
	DocumentID     uuid.UUID
	VersionID      uuid.UUID
	ActorID        uuid.UUID
}

func NewOnlyOfficeService(documents *Service, actors ActorLoader, cfg config.OnlyOffice, maxBytes int64) *OnlyOfficeService {
	return &OnlyOfficeService{documents: documents, actors: actors, config: cfg, maxBytes: maxBytes, client: &http.Client{
		Timeout: 45 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			if !sameEndpoint(req.URL.String(), cfg.InternalURL) {
				return errors.New("untrusted ONLYOFFICE redirect")
			}
			return nil
		},
	}}
}

func (s *OnlyOfficeService) AuthorizeCallback(ctx context.Context, claims callbackClaims) error {
	actor, err := s.actors.Actor(ctx, claims.OrganizationID, claims.ActorID)
	if err != nil {
		return err
	}
	_, err = s.documents.AuthorizeEdit(ctx, actor, claims.DocumentID)
	return err
}

func (s *OnlyOfficeService) Editor(ctx context.Context, actor access.Actor, documentID uuid.UUID, audit AuditContext) (EditorConfig, error) {
	document, version, err := s.documents.EditorDocument(ctx, actor, documentID, audit)
	if err != nil {
		return EditorConfig{}, err
	}
	expires := time.Now().Add(15 * time.Minute)
	fileToken, err := signJWT(s.config.JWTSecret, map[string]any{
		"purpose": "onlyoffice_file", "organizationId": actor.OrganizationID.String(),
		"documentId": document.ID.String(), "versionId": version.ID.String(), "exp": expires.Unix(),
	})
	if err != nil {
		return EditorConfig{}, err
	}
	callbackToken, err := signJWT(s.config.JWTSecret, map[string]any{
		"purpose": "onlyoffice_callback", "organizationId": actor.OrganizationID.String(),
		"documentId": document.ID.String(), "versionId": version.ID.String(), "actorId": actor.UserID.String(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	if err != nil {
		return EditorConfig{}, err
	}
	base := strings.TrimRight(s.config.InternalAPIURL, "/")
	fileURL := fmt.Sprintf("%s/api/v1/internal/onlyoffice/files/%s/%s?token=%s", base, document.ID, version.ID, url.QueryEscape(fileToken))
	callbackURL := fmt.Sprintf("%s/api/v1/internal/onlyoffice/callback/%s?token=%s", base, document.ID, url.QueryEscape(callbackToken))
	configPayload := map[string]any{
		"document": map[string]any{
			"fileType": "docx", "key": "opora-" + strings.ReplaceAll(version.ID.String(), "-", ""),
			"title": document.Title, "url": fileURL,
			"permissions": map[string]bool{"edit": true, "download": true, "print": true},
		},
		"documentType": "word",
		"editorConfig": map[string]any{
			"callbackUrl": callbackURL, "lang": "ru", "mode": "edit",
			"user":          map[string]string{"id": actor.UserID.String(), "name": document.ChangedBy},
			"customization": map[string]any{"autosave": true, "forcesave": true},
		},
		"height": "100%", "width": "100%", "type": "desktop",
	}
	configToken, err := signJWT(s.config.JWTSecret, configPayload)
	if err != nil {
		return EditorConfig{}, err
	}
	configPayload["token"] = configToken
	return EditorConfig{DocumentServerURL: strings.TrimRight(s.config.PublicURL, "/"), Config: configPayload}, nil
}

func (s *OnlyOfficeService) VerifyFileToken(token string, documentID, versionID uuid.UUID) (uuid.UUID, error) {
	claims, err := verifyJWT(s.config.JWTSecret, token)
	if err != nil || claimString(claims, "purpose") != "onlyoffice_file" || claimString(claims, "documentId") != documentID.String() || claimString(claims, "versionId") != versionID.String() {
		return uuid.Nil, ErrInvalidOnlyOfficeToken
	}
	organizationID, err := uuid.Parse(claimString(claims, "organizationId"))
	if err != nil {
		return uuid.Nil, ErrInvalidOnlyOfficeToken
	}
	return organizationID, nil
}

func (s *OnlyOfficeService) VerifyCallbackToken(token string, documentID uuid.UUID) (callbackClaims, error) {
	claims, err := verifyJWT(s.config.JWTSecret, token)
	if err != nil || claimString(claims, "purpose") != "onlyoffice_callback" || claimString(claims, "documentId") != documentID.String() {
		return callbackClaims{}, ErrInvalidOnlyOfficeToken
	}
	result := callbackClaims{DocumentID: documentID}
	result.OrganizationID, err = uuid.Parse(claimString(claims, "organizationId"))
	if err != nil {
		return callbackClaims{}, ErrInvalidOnlyOfficeToken
	}
	result.VersionID, err = uuid.Parse(claimString(claims, "versionId"))
	if err != nil {
		return callbackClaims{}, ErrInvalidOnlyOfficeToken
	}
	result.ActorID, err = uuid.Parse(claimString(claims, "actorId"))
	if err != nil {
		return callbackClaims{}, ErrInvalidOnlyOfficeToken
	}
	return result, nil
}

func (s *OnlyOfficeService) VerifyDocumentServerToken(token string) error {
	_, err := verifyJWT(s.config.JWTSecret, strings.TrimPrefix(token, "Bearer "))
	return err
}

func (s *OnlyOfficeService) FetchEdited(ctx context.Context, downloadURL string) ([]byte, error) {
	trustedURL, err := s.trustedDownloadURL(downloadURL)
	if err != nil {
		return nil, ErrUntrustedDownloadURL
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, trustedURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: request", ErrOnlyOfficeDownload)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrOnlyOfficeDownload, response.StatusCode)
	}
	if response.ContentLength > s.maxBytes {
		return nil, ErrFileTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, s.maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > s.maxBytes {
		return nil, ErrFileTooLarge
	}
	return data, nil
}

// DownloadEndpoint returns non-sensitive endpoint metadata for integration diagnostics.
func (s *OnlyOfficeService) DownloadEndpoint(raw string) (scheme, host string) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "invalid", "invalid"
	}
	return parsed.Scheme, parsed.Host
}

func (s *OnlyOfficeService) trustedDownloadURL(raw string) (string, error) {
	actual, err := url.Parse(raw)
	if err != nil || actual.User != nil || (actual.Scheme != "http" && actual.Scheme != "https") {
		return "", ErrInvalidOnlyOfficeToken
	}
	if !sameEndpoint(raw, s.config.InternalURL) && !sameEndpoint(raw, s.config.CallbackOrigin) {
		return "", ErrInvalidOnlyOfficeToken
	}
	internal, err := url.Parse(s.config.InternalURL)
	if err != nil {
		return "", err
	}
	actual.Scheme, actual.Host = internal.Scheme, internal.Host
	return actual.String(), nil
}

func sameEndpoint(rawURL, configured string) bool {
	actual, err := url.Parse(rawURL)
	if err != nil || (actual.Scheme != "http" && actual.Scheme != "https") || actual.User != nil {
		return false
	}
	base, err := url.Parse(configured)
	if err != nil {
		return false
	}
	return strings.EqualFold(actual.Scheme, base.Scheme) && strings.EqualFold(actual.Host, base.Host)
}

func signJWT(secret string, claims map[string]any) (string, error) {
	if len(secret) < 32 {
		return "", errors.New("ONLYOFFICE JWT secret is too short")
	}
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func verifyJWT(secret, token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(secret) < 32 {
		return nil, ErrInvalidOnlyOfficeToken
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidOnlyOfficeToken
	}
	var header map[string]any
	if json.Unmarshal(headerBytes, &header) != nil || header["alg"] != "HS256" {
		return nil, ErrInvalidOnlyOfficeToken
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, ErrInvalidOnlyOfficeToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidOnlyOfficeToken
	}
	var claims map[string]any
	if json.Unmarshal(payload, &claims) != nil {
		return nil, ErrInvalidOnlyOfficeToken
	}
	if exp, ok := claims["exp"].(float64); ok && time.Now().Unix() >= int64(exp) {
		return nil, ErrInvalidOnlyOfficeToken
	}
	return claims, nil
}

func claimString(claims map[string]any, key string) string {
	value, _ := claims[key].(string)
	return value
}
