package translation

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

func NewDB(dbPath string) (*DB, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("translation db path is required")
	}

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	if _, err := conn.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if _, err := conn.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("set wal mode: %w", err)
	}
	if _, err := conn.Exec(`PRAGMA busy_timeout = 3000;`); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	if err := verifySchema(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return &DB{Conn: conn}, nil
}

func verifySchema(db *sql.DB) error {
	requiredTables := []string{
		"translations",
		"translation_paragraphs",
		"translation_segments",
		"translation_jobs",
		"texts",
		"segments",
		"events",
		"vocab_items",
		"vocab_occurrences",
		"srs_state",
		"vocab_lookups",
		"user_profile",
		"character_word_links",
		"discovery_preferences",
		"discovery_runs",
		"article_recommendations",
	}
	for _, table := range requiredTables {
		var exists int
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
			table,
		).Scan(&exists); err != nil {
			return fmt.Errorf("verify schema table %s: %w", table, err)
		}
		if exists == 0 {
			return fmt.Errorf("database schema is not migrated: missing table %s", table)
		}
	}
	return nil
}

func newID() (string, error) {
	return fmt.Sprintf("%d", time.Now().UTC().UnixNano()), nil
}

func isDBLocked(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "database is locked")
}
