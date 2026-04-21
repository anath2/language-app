package translation

import (
	"fmt"
	"github.com/anath2/language-app/internal/storage"
)

type DB = storage.DB

func NewDB(dbPath string) (*DB, error) {
	db, err := storage.NewDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("initialize db: %w", err)
	}

	return &DB{Conn: db.Conn}, nil
}

func newID() (string, error) {
	return storage.NewID(), nil
}

func isDBLocked(err error) bool {
	return storage.IsDBLocked(err)
}
