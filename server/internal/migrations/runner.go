package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func RunUp(dbPath string, migrationsDir string) error {
	db, err := open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("run migrations up: %w", err)
	}
	return nil
}

func CurrentVersion(dbPath string, migrationsDir string) (int64, error) {
	db, err := open(dbPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	if err := goose.SetDialect("sqlite3"); err != nil {
		return 0, fmt.Errorf("set goose dialect: %w", err)
	}
	if _, err := goose.EnsureDBVersion(db); err != nil {
		return 0, fmt.Errorf("ensure goose version table: %w", err)
	}
	version, err := goose.GetDBVersion(db)
	if err != nil {
		return 0, fmt.Errorf("get db version: %w", err)
	}
	_ = migrationsDir
	return version, nil
}

func open(dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("translation db path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout = 3000;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	return db, nil
}
