package translation

import (
	"path/filepath"
	"testing"

	"github.com/anath2/language-app/internal/migrations"
)

func newTranslationStoreWithMigrations(t *testing.T) *TranslationStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "translations.db")
	migrationsDir := filepath.Join("..", "..", "migrations")
	if err := migrations.RunUp(dbPath, migrationsDir); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	return NewTranslationStore(db)
}
