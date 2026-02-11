package translation

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/anath2/language-app/internal/migrations"
)

func TestNewStoreRequiresMigratedSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "translations.db")
	_, err := NewStore(dbPath)
	if err == nil {
		t.Fatal("expected schema verification error for unmigrated database")
	}
	if !strings.Contains(err.Error(), "not migrated") {
		t.Fatalf("expected unmigrated schema error, got: %v", err)
	}
}

func TestRunUpIsIdempotentAndCreatesUsableSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "translations.db")
	migrationsDir := filepath.Join("..", "..", "migrations")

	if err := migrations.RunUp(dbPath, migrationsDir); err != nil {
		t.Fatalf("first run migrations: %v", err)
	}
	if err := migrations.RunUp(dbPath, migrationsDir); err != nil {
		t.Fatalf("second run migrations: %v", err)
	}

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("new migrated store: %v", err)
	}

	tr, err := store.Create("你好", "text")
	if err != nil {
		t.Fatalf("create translation on migrated schema: %v", err)
	}
	if tr.ID == "" {
		t.Fatal("expected translation id")
	}
}
