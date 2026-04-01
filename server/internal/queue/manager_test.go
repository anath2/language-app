package queue

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anath2/language-app/internal/intelligence"
	"github.com/anath2/language-app/internal/migrations"
	"github.com/anath2/language-app/internal/translation"
)

type mockProvider struct {
	translateFullCalls int
	translateFullErr   error
}

func newTranslationStoreForTest(t *testing.T, dbPath string) *translation.TranslationStore {
	t.Helper()

	db, err := translation.NewDB(dbPath)
	if err != nil {
		t.Fatalf("new translation db: %v", err)
	}
	return translation.NewTranslationStore(db)
}

func (m *mockProvider) TranslateFull(_ context.Context, text string) (string, error) {
	m.translateFullCalls++
	if m.translateFullErr != nil {
		return "", m.translateFullErr
	}
	return "mock translation of: " + text, nil
}

func (m *mockProvider) Segment(_ context.Context, text string) ([]string, error) {
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

func (m *mockProvider) LookupCharacter(_ string) (string, string, bool) {
	return "", "", false
}

func (m *mockProvider) TranslateSegments(_ context.Context, segments []string, _ string) ([]translation.SegmentResult, error) {
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

func (m *mockProvider) ChatWithTranslationContext(_ context.Context, req intelligence.ChatWithTranslationRequest, onChunk func(string) error) (string, error) {
	reply := "mock chat response to: " + req.UserMessage
	if onChunk != nil {
		_ = onChunk(reply)
	}
	return reply, nil
}

func TestQueueProgressLifecycle(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "translations.db")
	if err := migrations.RunUp(dbPath, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	store := newTranslationStoreForTest(t, dbPath)
	manager := NewManager(store, &mockProvider{})

	item, err := store.Create("你好世界", "text")
	if err != nil {
		t.Fatalf("create translation: %v", err)
	}

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

}

func TestQueueProgressSurvivesManagerRestart(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "translations.db")
	if err := migrations.RunUp(dbPath, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	store := newTranslationStoreForTest(t, dbPath)
	manager := NewManager(store, &mockProvider{})

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
	managerAfterRestart := NewManager(store, &mockProvider{})
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
	manager := NewManager(store, &mockProvider{})
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

func TestTranslateFullFailureFails(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "translations.db")
	if err := migrations.RunUp(dbPath, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	store := newTranslationStoreForTest(t, dbPath)
	provider := &mockProvider{translateFullErr: fmt.Errorf("upstream unavailable")}
	manager := NewManager(store, provider)

	item, err := store.Create("你好世界", "text")
	if err != nil {
		t.Fatalf("create translation: %v", err)
	}

	manager.StartProcessing(item.ID)

	deadline := time.Now().Add(2 * time.Second)
	for {
		tr, ok := store.Get(item.ID)
		if ok && (tr.Status == "failed" || tr.Status == "completed") {
			if tr.Status != "failed" {
				t.Fatalf("expected failed status, got %q", tr.Status)
			}
			if tr.ErrorMessage == nil || *tr.ErrorMessage == "" {
				t.Fatal("expected error message to be set")
			}
			if tr.FullTranslation != nil {
				t.Fatal("expected full translation to be nil on failure")
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for translation to fail")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestReprocessingPreservesFullTranslation(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "translations.db")
	if err := migrations.RunUp(dbPath, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	store := newTranslationStoreForTest(t, dbPath)
	provider := &mockProvider{}
	manager := NewManager(store, provider)

	item, err := store.Create("你好世界", "text")
	if err != nil {
		t.Fatalf("create translation: %v", err)
	}
	manager.StartProcessing(item.ID)

	deadline := time.Now().Add(2 * time.Second)
	for {
		tr, ok := store.Get(item.ID)
		if ok && tr.Status == "completed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for initial translation to complete")
		}
		time.Sleep(20 * time.Millisecond)
	}

	tr, _ := store.Get(item.ID)
	originalFull := *tr.FullTranslation
	callsBeforeReprocess := provider.translateFullCalls

	sentencesToProcess, err := store.UpdateInputTextForReprocessing(item.ID, "你好世界 今天")
	if err != nil {
		t.Fatalf("update input text: %v", err)
	}
	manager.StartReprocessing(item.ID, sentencesToProcess)

	deadline = time.Now().Add(2 * time.Second)
	for {
		tr, ok := store.Get(item.ID)
		if ok && tr.Status == "completed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for reprocessing to complete")
		}
		time.Sleep(20 * time.Millisecond)
	}

	tr, _ = store.Get(item.ID)
	if tr.FullTranslation == nil || *tr.FullTranslation != originalFull {
		t.Fatalf("expected full translation to be preserved %q, got %v", originalFull, tr.FullTranslation)
	}
	if provider.translateFullCalls != callsBeforeReprocess {
		t.Fatalf("expected TranslateFull not to be called during reprocessing, call count went from %d to %d",
			callsBeforeReprocess, provider.translateFullCalls)
	}
}

func TestReprocessingGeneratesFullTranslationWhenAbsent(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "translations.db")
	if err := migrations.RunUp(dbPath, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	store := newTranslationStoreForTest(t, dbPath)
	provider := &mockProvider{}
	manager := NewManager(store, provider)

	// Create a translation that has never been processed (full_translation is NULL).
	item, err := store.Create("你好世界", "text")
	if err != nil {
		t.Fatalf("create translation: %v", err)
	}

	sentencesToProcess, err := store.UpdateInputTextForReprocessing(item.ID, "你好世界")
	if err != nil {
		t.Fatalf("update input text: %v", err)
	}
	manager.StartReprocessing(item.ID, sentencesToProcess)

	deadline := time.Now().Add(2 * time.Second)
	for {
		tr, ok := store.Get(item.ID)
		if ok && (tr.Status == "completed" || tr.Status == "failed") {
			if tr.Status != "completed" {
				t.Fatalf("expected completed status, got %q", tr.Status)
			}
			if tr.FullTranslation == nil || *tr.FullTranslation == "" {
				t.Fatal("expected full translation to be generated")
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for reprocessing to complete")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestScannerRecoversStaleLeasedJob(t *testing.T) {
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

	// Simulate a job claimed by a crashed worker: leased with an already-expired lease.
	claimed, err := store.ClaimTranslationJob(item.ID, 1*time.Millisecond)
	if err != nil || !claimed {
		t.Fatalf("claim job: err=%v claimed=%v", err, claimed)
	}
	time.Sleep(5 * time.Millisecond) // ensure lease has expired

	manager := NewManager(store, &mockProvider{})
	manager.ResumeRestartableJobs()

	deadline := time.Now().Add(2 * time.Second)
	for {
		tr, ok := store.Get(item.ID)
		if ok && tr.Status == "completed" {
			return
		}
		if time.Now().After(deadline) {
			tr, _ = store.Get(item.ID)
			t.Fatalf("timed out waiting for stale job recovery; status=%q", tr.Status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestScannerDoesNotDoubleProcessActiveJob(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "translations.db")
	if err := migrations.RunUp(dbPath, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	store := newTranslationStoreForTest(t, dbPath)
	manager := NewManager(store, &mockProvider{})

	item, err := store.Create("你好世界", "text")
	if err != nil {
		t.Fatalf("create translation: %v", err)
	}

	manager.StartProcessing(item.ID)

	// While the job is in-flight, fire a scanner tick.
	// StartProcessing checks m.running and returns early — no second claim.
	manager.ResumeRestartableJobs()

	deadline := time.Now().Add(2 * time.Second)
	for {
		tr, ok := store.Get(item.ID)
		if ok && tr.Status == "completed" {
			attempts, err := store.GetJobAttempts(item.ID)
			if err != nil {
				t.Fatalf("get job attempts: %v", err)
			}
			if attempts != 1 {
				t.Fatalf("expected attempts=1, got %d (job was claimed more than once)", attempts)
			}
			return
		}
		if ok && tr.Status == "failed" {
			t.Fatalf("job failed: %v", tr.ErrorMessage)
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
