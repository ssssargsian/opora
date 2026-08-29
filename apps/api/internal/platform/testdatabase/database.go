// Package testdatabase provides real PostgreSQL databases for integration tests.
package testdatabase

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"opora.local/api/internal/platform/database"
)

func Start(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:18.6-alpine", postgres.WithDatabase("opora_test"),
		postgres.WithUsername("opora"), postgres.WithPassword("test-only-password"), postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatalf("start PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("PostgreSQL connection string: %v", err)
	}
	pool, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	t.Cleanup(pool.Close)
	applyMigrations(t, pool)
	return pool
}

func applyMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migrations path")
	}
	directory := filepath.Join(filepath.Dir(filename), "..", "..", "..", "db", "migrations")
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		// #nosec G304 -- names come from ReadDir and are filtered to migration files.
		migration, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		upSQL, _, found := strings.Cut(string(migration), "-- +goose Down")
		if !found {
			t.Fatalf("migration %s has no Down section", name)
		}
		if _, err := pool.Exec(context.Background(), upSQL); err != nil {
			t.Fatalf("apply migration %s: %v", name, err)
		}
	}
}
