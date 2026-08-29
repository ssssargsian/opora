// Package config loads and validates API configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config contains process configuration. Secret fields must never be logged.
type Config struct {
	Environment string
	DatabaseURL string
	HTTP        HTTP
	Auth        Auth
	Upload      Upload
	Storage     Storage
	OnlyOffice  OnlyOffice
	ClamAV      ClamAV
	Dev         Dev
}

// HTTP defines the API listener and bounded server timeouts.
type HTTP struct {
	Address           string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// Auth defines persistent browser session and origin settings.
type Auth struct {
	SessionCookieName string
	CSRFCookieName    string
	SessionTTL        time.Duration
	CookieSecure      bool
	AllowedOrigin     string
}

// Upload defines hard limits for untrusted document input.
type Upload struct {
	MaxBytes int64
}

// Storage defines the S3-compatible private object store.
type Storage struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	UseTLS    bool
}

// OnlyOffice defines server-side Document Server integration settings.
type OnlyOffice struct {
	PublicURL      string
	InternalURL    string
	InternalAPIURL string
	CallbackOrigin string
	JWTSecret      string
}

// ClamAV defines the malware scanner endpoint.
type ClamAV struct {
	Address string
}

// Dev contains explicitly opt-in local bootstrap settings.
type Dev struct {
	AdminEmail    string
	AdminPassword string
	Organization  string
}

// Load reads environment variables and validates settings needed at API startup.
func Load() (Config, error) {
	cfg := Config{
		Environment: env("APP_ENV", "development"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		HTTP: HTTP{
			Address:           env("HTTP_ADDRESS", ":8080"),
			ReadTimeout:       duration("HTTP_READ_TIMEOUT", 15*time.Second),
			ReadHeaderTimeout: duration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			WriteTimeout:      duration("HTTP_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:       duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout:   duration("HTTP_SHUTDOWN_TIMEOUT", 15*time.Second),
		},
		Auth: Auth{
			SessionCookieName: env("SESSION_COOKIE_NAME", "opora_session"),
			CSRFCookieName:    env("CSRF_COOKIE_NAME", "opora_csrf"),
			SessionTTL:        duration("SESSION_TTL", 7*24*time.Hour),
			CookieSecure:      boolean("COOKIE_SECURE", false),
			AllowedOrigin:     env("WEB_ORIGIN", "http://localhost:3000"),
		},
		Upload: Upload{MaxBytes: int64Value("MAX_UPLOAD_BYTES", 25*1024*1024)},
		Storage: Storage{
			Endpoint:  env("S3_ENDPOINT", "http://localhost:9000"),
			Region:    env("S3_REGION", "us-east-1"),
			Bucket:    env("S3_BUCKET", "opora-documents"),
			AccessKey: os.Getenv("S3_ACCESS_KEY"),
			SecretKey: os.Getenv("S3_SECRET_KEY"),
			UseTLS:    boolean("S3_USE_TLS", false),
		},
		OnlyOffice: OnlyOffice{
			PublicURL:      env("ONLYOFFICE_PUBLIC_URL", "http://localhost:8082"),
			InternalURL:    env("ONLYOFFICE_INTERNAL_URL", "http://localhost:8082"),
			InternalAPIURL: env("ONLYOFFICE_INTERNAL_API_URL", "http://host.docker.internal:8080"),
			CallbackOrigin: env("ONLYOFFICE_CALLBACK_ORIGIN", "http://localhost"),
			JWTSecret:      os.Getenv("ONLYOFFICE_JWT_SECRET"),
		},
		ClamAV: ClamAV{Address: env("CLAMAV_ADDRESS", "localhost:3310")},
		Dev: Dev{
			AdminEmail:    env("DEV_ADMIN_EMAIL", "admin@opora.local"),
			AdminPassword: os.Getenv("DEV_ADMIN_PASSWORD"),
			Organization:  env("DEV_ORGANIZATION_NAME", "Демонстрационная школа"),
		},
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.HTTP.Address == "" {
		return Config{}, errors.New("HTTP_ADDRESS must not be empty")
	}
	if cfg.Upload.MaxBytes <= 0 {
		return Config{}, errors.New("MAX_UPLOAD_BYTES must be positive")
	}
	if cfg.Environment == "production" && !cfg.Auth.CookieSecure {
		return Config{}, errors.New("COOKIE_SECURE must be true in production")
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func duration(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func boolean(name string, fallback bool) bool {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func int64Value(name string, fallback int64) int64 {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

// ValidateDocumentIntegrations checks settings deferred until document features are enabled.
func (c Config) ValidateDocumentIntegrations() error {
	if c.Storage.AccessKey == "" || c.Storage.SecretKey == "" {
		return errors.New("S3 credentials are required for document operations")
	}
	if c.OnlyOffice.JWTSecret == "" {
		return errors.New("ONLYOFFICE_JWT_SECRET is required for editor operations")
	}
	if len(c.OnlyOffice.JWTSecret) < 32 {
		return fmt.Errorf("ONLYOFFICE_JWT_SECRET must contain at least %d characters", 32)
	}
	return nil
}
