package database_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"opora.local/api/internal/platform/database"
)

func TestPostgresConnectionAndMigrations(t *testing.T) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:18.6-alpine",
		postgres.WithDatabase("opora_test"),
		postgres.WithUsername("opora"),
		postgres.WithPassword("test-only-password"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start PostgreSQL container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Errorf("terminate PostgreSQL container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("PostgreSQL connection string: %v", err)
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration path")
	}

	pool, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer pool.Close()

	migrationRoot, err := os.OpenRoot(filepath.Join(filepath.Dir(filename), "..", "..", "..", "db", "migrations"))
	if err != nil {
		t.Fatalf("open migration root: %v", err)
	}
	t.Cleanup(func() {
		if err := migrationRoot.Close(); err != nil {
			t.Errorf("close migration root: %v", err)
		}
	})
	migration, err := migrationRoot.ReadFile("00001_foundation.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	upSQL, _, found := strings.Cut(string(migration), "-- +goose Down")
	if !found {
		t.Fatal("migration is missing a goose Down section")
	}
	if _, err := pool.Exec(ctx, upSQL); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	var permissionCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM permissions").Scan(&permissionCount); err != nil {
		t.Fatalf("query permissions: %v", err)
	}
	if permissionCount != 16 {
		t.Fatalf("permission count = %d, want 16", permissionCount)
	}
}
