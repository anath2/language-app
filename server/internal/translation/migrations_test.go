package translation

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/anath2/language-app/internal/migrations"
)

func TestNewDBRequiresMigratedSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "translations.db")
	_, err := NewDB(dbPath)
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

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("new migrated db: %v", err)
	}
	store := NewTranslationStore(db)

	tr, err := store.Create("你好", "text")
	if err != nil {
		t.Fatalf("create translation on migrated schema: %v", err)
	}
	if tr.ID == "" {
		t.Fatal("expected translation id")
	}
}
