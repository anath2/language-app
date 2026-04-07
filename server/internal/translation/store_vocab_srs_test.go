package translation

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/anath2/language-app/internal/migrations"
	_ "modernc.org/sqlite"
)

func newSRSStoreWithMigrations(t *testing.T) *SRSStore {
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
	return NewSRSStore(db)
}

func copyMigrationsThrough(t *testing.T, lastPrefix string) string {
	t.Helper()
	srcDir := filepath.Join("..", "..", "migrations")
	dstDir := filepath.Join(t.TempDir(), "migrations")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sql" {
			continue
		}
		if e.Name() > lastPrefix {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		srcPath := filepath.Join(srcDir, name)
		dstPath := filepath.Join(dstDir, name)
		data, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			t.Fatalf("write migration %s: %v", name, err)
		}
	}
	return dstDir
}

func TestVocabSplitMigrationDedupKeepsLatestEnglish(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "translations.db")
	preSplitDir := copyMigrationsThrough(t, "00015_drop_texts_and_rewire_translation_fks.sql")
	if err := migrations.RunUp(dbPath, preSplitDir); err != nil {
		t.Fatalf("run migrations through 00015: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open pre-split db: %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		t.Fatalf("enable fk on pre-split db: %v", err)
	}
	defer db.Close()
	nowOld := "2026-04-01T00:00:00Z"
	nowNew := "2026-04-02T00:00:00Z"
	_, err = db.Exec(
		`INSERT INTO vocab_items (id, headword, pinyin, english, type, status, created_at, updated_at, last_seen_snippet, seen_count)
		 VALUES
		 ('w1', '你好', 'ni hao', 'hello-old', 'word', 'learning', ?, ?, '', 1),
		 ('w2', '你好', 'ni hao', 'hello-new', 'word', 'learning', ?, ?, '', 2)`,
		nowOld, nowOld, nowNew, nowNew,
	)
	if err != nil {
		t.Fatalf("seed old vocab_items: %v", err)
	}

	// Apply the split migration.
	allMigrationsDir := filepath.Join("..", "..", "migrations")
	if err := migrations.RunUp(dbPath, allMigrationsDir); err != nil {
		t.Fatalf("run migrations through 00016: %v", err)
	}

	dbAfter, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("new db post-split: %v", err)
	}
	var count int
	var english string
	if err := dbAfter.Conn.QueryRow(
		`SELECT COUNT(*), COALESCE(MAX(english), '')
		 FROM saved_segments
		 WHERE headword = '你好' AND pinyin = 'ni hao'`,
	).Scan(&count, &english); err != nil {
		t.Fatalf("query saved_segments dedup: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 deduped saved segment, got %d", count)
	}
	if english != "hello-new" {
		t.Fatalf("expected latest english to win, got %q", english)
	}
}

func TestCharacterReviewQueueIncludesExampleSegments(t *testing.T) {
	srs := newSRSStoreWithMigrations(t)

	segmentID, err := srs.SaveSegment("银行", "yin hang", "bank", nil, nil, "learning")
	if err != nil {
		t.Fatalf("save segment: %v", err)
	}
	err = srs.ExtractAndLinkCharacters(segmentID, "银行", "yin hang", "bank", []CharTranslation{
		{Char: "银", Pinyin: "yin"},
		{Char: "行", Pinyin: "hang"},
	})
	if err != nil {
		t.Fatalf("extract and link characters: %v", err)
	}

	cards, err := srs.GetCharacterReviewQueue(10)
	if err != nil {
		t.Fatalf("get character review queue: %v", err)
	}
	if len(cards) == 0 {
		t.Fatal("expected character review cards after extraction")
	}
	foundExample := false
	for _, card := range cards {
		for _, ex := range card.ExampleSegments {
			if ex.Segment == "银行" && ex.SegmentTranslation == "bank" {
				foundExample = true
			}
		}
	}
	if !foundExample {
		t.Fatal("expected character review queue to include segment context examples")
	}
}

func TestExportImportProgressJSONSplitTablesRoundtrip(t *testing.T) {
	origin := newSRSStoreWithMigrations(t)
	segmentID, err := origin.SaveSegment("人工智能", "ren gong zhi neng", "artificial intelligence", nil, nil, "learning")
	if err != nil {
		t.Fatalf("save segment: %v", err)
	}
	if err := origin.ExtractAndLinkCharacters(segmentID, "人工智能", "ren gong zhi neng", "artificial intelligence", nil); err != nil {
		t.Fatalf("extract and link characters: %v", err)
	}
	exported, err := origin.ExportProgressJSON()
	if err != nil {
		t.Fatalf("export progress json: %v", err)
	}

	// Fresh DB import.
	dbPath := filepath.Join(t.TempDir(), "import.db")
	migrationsDir := filepath.Join("..", "..", "migrations")
	if err := migrations.RunUp(dbPath, migrationsDir); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("new db for import: %v", err)
	}
	target := NewSRSStore(db)

	counts, err := target.ImportProgressJSON(exported)
	if err != nil {
		t.Fatalf("import progress json: %v", err)
	}
	if counts["saved_segments"] == 0 {
		t.Fatal("expected imported saved_segments count to be > 0")
	}
	if counts["saved_characters"] == 0 {
		t.Fatal("expected imported saved_characters count to be > 0")
	}

	segmentCards, err := target.GetSegmentReviewQueue(10)
	if err != nil {
		t.Fatalf("get segment review queue: %v", err)
	}
	if len(segmentCards) == 0 {
		t.Fatal("expected segment review queue to be populated after import")
	}
	charCards, err := target.GetCharacterReviewQueue(10)
	if err != nil {
		t.Fatalf("get character review queue: %v", err)
	}
	if len(charCards) == 0 {
		t.Fatal("expected character review queue to be populated after import")
	}
}
