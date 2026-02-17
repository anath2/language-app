package queue

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anath2/language-app/internal/migrations"
	"github.com/anath2/language-app/internal/translation"
)

type mockProvider struct{}

func newTranslationStoreForTest(t *testing.T, dbPath string) *translation.TranslationStore {
	t.Helper()

	db, err := translation.NewDB(dbPath)
	if err != nil {
		t.Fatalf("new translation db: %v", err)
	}
	return translation.NewTranslationStore(db)
}

func (m mockProvider) TranslateFull(_ context.Context, text string) (string, error) {
	return "mock translation of: " + text, nil
}

func (m mockProvider) Segment(_ context.Context, text string) ([]string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{}, nil
	}
	out := make([]string, 0, len([]rune(text)))
	for _, r := range []rune(text) {
		if r == ' ' || r == '\n' || r == '\t' {
			continue
		}
		out = append(out, string(r))
	}
	return out, nil
}

func (m mockProvider) SuggestArticleURLs(_ context.Context, _ []string, _ []string) ([]string, error) {
	return nil, nil
}

func (m mockProvider) LookupCharacter(_ string) (string, string, bool) {
	return "", "", false
}

func (m mockProvider) TranslateSegments(_ context.Context, segments []string, _ string) ([]translation.SegmentResult, error) {
	out := make([]translation.SegmentResult, 0, len(segments))
	for _, seg := range segments {
		out = append(out, translation.SegmentResult{
			Segment: seg,
			Pinyin:  "",
			English: "translation_of_" + seg,
		})
	}
	return out, nil
}

func TestQueueProgressLifecycle(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "translations.db")
	if err := migrations.RunUp(dbPath, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	store := newTranslationStoreForTest(t, dbPath)
	manager := NewManager(store, mockProvider{})

	item, err := store.Create("你好世界", "text")
	if err != nil {
		t.Fatalf("create translation: %v", err)
	}

	manager.Submit(item.ID)
	manager.StartProcessing(item.ID)

	deadline := time.Now().Add(2 * time.Second)
	for {
		progress, ok := manager.GetProgress(item.ID)
		if ok && progress.Status == "completed" {
			if progress.Current == 0 {
				t.Fatal("expected current progress > 0")
			}
			if progress.Total == 0 {
				t.Fatal("expected total > 0")
			}
			if len(progress.Results) == 0 {
				t.Fatal("expected non-empty progress results")
			}
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for completed progress")
		}
		time.Sleep(20 * time.Millisecond)
	}

	tr, ok := store.Get(item.ID)
	if !ok {
		t.Fatal("expected translation to exist")
	}
	if tr.Status != "completed" {
		t.Fatalf("expected completed translation status, got %q", tr.Status)
	}
	if tr.FullTranslation == nil || *tr.FullTranslation == "" {
		t.Fatal("expected full translation to be set")
	}

	manager.CleanupProgress(item.ID)
	if progress, ok := manager.GetProgress(item.ID); !ok || progress.Status != "completed" {
		t.Fatal("expected persisted completed progress after cleanup")
	}
}

func TestQueueProgressSurvivesManagerRestart(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "translations.db")
	if err := migrations.RunUp(dbPath, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	store := newTranslationStoreForTest(t, dbPath)
	manager := NewManager(store, mockProvider{})

	item, err := store.Create("你好", "text")
	if err != nil {
		t.Fatalf("create translation: %v", err)
	}

	manager.StartProcessing(item.ID)

	deadline := time.Now().Add(2 * time.Second)
	for {
		progress, ok := manager.GetProgress(item.ID)
		if ok && progress.Status == "completed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for completed progress")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Simulate process restart by creating a new manager over same DB-backed store.
	managerAfterRestart := NewManager(store, mockProvider{})
	progress, ok := managerAfterRestart.GetProgress(item.ID)
	if !ok {
		t.Fatal("expected progress to be recoverable from DB after restart")
	}
	if progress.Status != "completed" {
		t.Fatalf("expected completed status, got %q", progress.Status)
	}
	if len(progress.Results) == 0 {
		t.Fatal("expected persisted segment results")
	}
}

func TestResumeRestartableJobsCompletesPendingTranslation(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "translations.db")
	if err := migrations.RunUp(dbPath, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	store := newTranslationStoreForTest(t, dbPath)

	item, err := store.Create("你好世界", "text")
	if err != nil {
		t.Fatalf("create translation: %v", err)
	}
	if item.Status != "pending" {
		t.Fatalf("expected pending status, got %q", item.Status)
	}

	// Simulate startup: new manager should discover and process pending jobs.
	manager := NewManager(store, mockProvider{})
	manager.ResumeRestartableJobs()

	deadline := time.Now().Add(2 * time.Second)
	for {
		progress, ok := manager.GetProgress(item.ID)
		if ok && progress.Status == "completed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for resumed pending translation to complete")
		}
		time.Sleep(20 * time.Millisecond)
	}

	tr, ok := store.Get(item.ID)
	if !ok {
		t.Fatal("expected translation to exist")
	}
	if tr.Status != "completed" {
		t.Fatalf("expected completed translation status, got %q", tr.Status)
	}
}
