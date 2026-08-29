package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error without DATABASE_URL")
	}
}

func TestLoadUsesSafeHTTPDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/opora")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTP.ReadHeaderTimeout <= 0 || cfg.HTTP.ShutdownTimeout <= 0 {
		t.Fatal("HTTP timeouts must be positive")
	}
}
