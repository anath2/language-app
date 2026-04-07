# Lease Renewal and Expired-Lease Scanner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add lease renewal to active job workers and a periodic background scanner that re-queues jobs with expired leases, so stuck jobs recover without a server restart.

**Architecture:** A renewal goroutine (100s tick) lives at the top of `runJob`, which is expanded to own the full job pipeline. A `StartBackgroundScanner` method on `Manager` ticks every 30s and calls the existing `ResumeRestartableJobs`. The in-memory `running` map already prevents double-processing at no extra cost.

**Tech Stack:** Go standard library (`context`, `time`), SQLite via `database/sql`, existing `queue` and `translation` packages.

**Spec:** `docs/superpowers/specs/2026-04-01-lease-renewal-design.md`

---

## File Map

| File | Change |
|------|--------|
| `server/internal/translation/store_jobs.go` | Add `RenewLease` and `GetJobAttempts` methods |
| `server/internal/translation/store_jobs_test.go` | New — unit tests for `RenewLease` |
| `server/internal/queue/manager.go` | Update constants; add `RenewLease` to interface; expand `runJob` to own full pipeline + renewal goroutine; add renewal to `StartReprocessing`; add `StartBackgroundScanner` |
| `server/internal/queue/manager_test.go` | Add scanner recovery test, scanner no-double-process test |
| `server/internal/http/server.go` | Call `StartBackgroundScanner` after `ResumeRestartableJobs`; add `"context"` import |

---

## Task 1: Add `RenewLease` and `GetJobAttempts` to the store

**Files:**
- Modify: `server/internal/translation/store_jobs.go`
- Create: `server/internal/translation/store_jobs_test.go`

`GetJobAttempts` is a test-support helper: `Get` only queries the `translations` table, not `translation_jobs`, so `attempts` is not otherwise accessible to callers. It is added to the store (not the interface) so that tests holding a concrete `*TranslationStore` can verify claim counts.

- [ ] **Step 1: Create the test file with failing tests**

Create `server/internal/translation/store_jobs_test.go`:

```go
package translation

import (
	"testing"
	"time"
)

func TestRenewLeaseUpdatesLeaseUntil(t *testing.T) {
	store := newTranslationStoreWithMigrations(t)

	item, err := store.Create("你好", "text")
	if err != nil {
		t.Fatalf("create translation: %v", err)
	}

	claimed, err := store.ClaimTranslationJob(item.ID, 30*time.Second)
	if err != nil || !claimed {
		t.Fatalf("claim translation job: err=%v claimed=%v", err, claimed)
	}

	before := getLeaseUntil(t, store, item.ID)
	time.Sleep(5 * time.Millisecond)

	if err := store.RenewLease(item.ID, 5*time.Minute); err != nil {
		t.Fatalf("renew lease: %v", err)
	}

	after := getLeaseUntil(t, store, item.ID)
	if after <= before {
		t.Fatalf("expected lease_until to advance: before=%q after=%q", before, after)
	}
}

func TestRenewLeaseNoopForCompletedJob(t *testing.T) {
	store := newTranslationStoreWithMigrations(t)

	item, err := store.Create("你好", "text")
	if err != nil {
		t.Fatalf("create translation: %v", err)
	}

	_, _ = store.ClaimTranslationJob(item.ID, 30*time.Second)
	// Complete requires full_translation to be set.
	if err := store.SetFullTranslation(item.ID, "mock full translation"); err != nil {
		t.Fatalf("set full translation: %v", err)
	}
	if err := store.Complete(item.ID); err != nil {
		t.Fatalf("complete: %v", err)
	}

	if err := store.RenewLease(item.ID, 5*time.Minute); err != nil {
		t.Fatalf("renew lease on completed job should not error: %v", err)
	}

	tr, ok := store.Get(item.ID)
	if !ok {
		t.Fatal("translation not found")
	}
	if tr.Status != "completed" {
		t.Fatalf("expected status to remain completed, got %q", tr.Status)
	}
}

// getLeaseUntil reads the raw lease_until string from the DB.
// White-box helper — only valid inside package translation.
func getLeaseUntil(t *testing.T, store *TranslationStore, translationID string) string {
	t.Helper()
	var leaseUntil string
	err := store.db.QueryRow(
		"SELECT COALESCE(lease_until, '') FROM translation_jobs WHERE translation_id = ?",
		translationID,
	).Scan(&leaseUntil)
	if err != nil {
		t.Fatalf("get lease_until: %v", err)
	}
	return leaseUntil
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

```bash
cd server && go test ./internal/translation/ -run 'TestRenewLease' -v
```

Expected: compile error — `store.RenewLease undefined`.

- [ ] **Step 3: Implement `RenewLease` and `GetJobAttempts` in `store_jobs.go`**

Add both methods to the bottom of `server/internal/translation/store_jobs.go`:

```go
// RenewLease extends the lease_until for a leased job. Intentionally has no
// retry loop — unlike ClaimTranslationJob, a missed renewal is harmless because
// the 5-minute lease provides ample headroom; the next tick retries.
func (s *TranslationStore) RenewLease(translationID string, d time.Duration) error {
	leaseUntil := time.Now().UTC().Add(d).Format(time.RFC3339Nano)
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(
		`UPDATE translation_jobs
		 SET lease_until = ?, updated_at = ?
		 WHERE translation_id = ? AND state = 'leased'`,
		leaseUntil, updatedAt, translationID,
	)
	if err != nil {
		return fmt.Errorf("renew lease: %w", err)
	}
	return nil
}

// GetJobAttempts returns the number of times a job has been claimed.
// Used in tests to verify a job was not double-claimed. Not added to the
// translationStore interface because Get() only queries the translations table.
func (s *TranslationStore) GetJobAttempts(translationID string) (int, error) {
	var attempts int
	err := s.db.QueryRow(
		`SELECT attempts FROM translation_jobs WHERE translation_id = ?`,
		translationID,
	).Scan(&attempts)
	if err != nil {
		return 0, fmt.Errorf("get job attempts: %w", err)
	}
	return attempts, nil
}
```

- [ ] **Step 4: Run the tests to confirm they pass**

```bash
cd server && go test ./internal/translation/ -run 'TestRenewLease' -v
```

Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
cd server && gofmt -w internal/translation/store_jobs.go internal/translation/store_jobs_test.go
git add server/internal/translation/store_jobs.go server/internal/translation/store_jobs_test.go
git commit -m "feat: add RenewLease and GetJobAttempts to TranslationStore"
```

---

## Task 2: Update constants and interface in `manager.go`

**Files:**
- Modify: `server/internal/queue/manager.go`

- [ ] **Step 1: Replace the `jobLeaseDuration` constant and add two new ones**

Find the existing constant at line ~64:
```go
// Before:
const jobLeaseDuration = 30 * time.Second

// After:
const jobLeaseDuration         = 5 * time.Minute
const leaseRenewalInterval     = 100 * time.Second // renew at ~1/3 of jobLeaseDuration
const expiredLeaseScanInterval = 30 * time.Second  // how often the scanner polls for expired leases
```

- [ ] **Step 2: Add `RenewLease` to the `translationStore` interface**

The interface starts at line 38. Add one line:

```go
type translationStore interface {
	ListRestartableTranslationIDs() ([]string, error)
	ClaimTranslationJob(translationID string, leaseDuration time.Duration) (bool, error)
	RenewLease(translationID string, d time.Duration) error  // ← add this
	Fail(id string, message string) error
	// ... rest unchanged
```

- [ ] **Step 3: Verify compile**

```bash
cd server && go build ./internal/queue/
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
cd server && gofmt -w internal/queue/manager.go
git add server/internal/queue/manager.go
git commit -m "feat: update lease constants and add RenewLease to translationStore interface"
```

---

## Task 3: Expand `runJob` to own the full pipeline and add the renewal goroutine

**Files:**
- Modify: `server/internal/queue/manager.go`

`runJob` currently only runs the segment loop (lines ~325–348). All the prep work — sentence splitting, `TranslateFull`, `segmentInputBySentence`, `SetProcessing` — lives in the anonymous goroutine in `StartProcessing` (lines 113–173). This task moves all of that into `runJob` so the renewal goroutine at the top of `runJob` covers the entire job lifetime.

- [ ] **Step 1: Replace the `runJob` function entirely**

Find the existing `runJob` (starts at line ~325) and replace the whole function with:

```go
func (m *Manager) runJob(ctx context.Context, translationID string, item translation.Translation) {
	defer m.removeRunning(translationID)

	// Renewal goroutine: extends the lease every leaseRenewalInterval.
	// Cancelled automatically when runJob returns via defer cancelRenew() —
	// no per-return cancelRenew() call is needed anywhere in this function.
	renewCtx, cancelRenew := context.WithCancel(ctx)
	defer cancelRenew()
	go func() {
		ticker := time.NewTicker(leaseRenewalInterval)
		defer ticker.Stop()
		consecutiveFailures := 0
		for {
			select {
			case <-ticker.C:
				if err := m.store.RenewLease(translationID, jobLeaseDuration); err != nil {
					consecutiveFailures++
					// TODO: if consecutiveFailures exceeds a threshold (e.g. 3),
					// fail the job to avoid a zombie worker holding a claim it can no longer renew.
					log.Printf("lease renewal failed for %s (consecutive failures: %d): %v",
						translationID, consecutiveFailures, err)
				} else {
					consecutiveFailures = 0
				}
			case <-renewCtx.Done():
				return
			}
		}
	}()

	sentences := splitInputSentences(item.InputText)
	if len(sentences) == 0 {
		_ = m.store.Fail(translationID, "No sentences found for segmentation")
		return
	}

	fullTranslation, err := m.provider.TranslateFull(ctx, item.InputText)
	if err != nil {
		_ = m.store.Fail(translationID, "Failed to generate full translation: "+err.Error())
		return
	}
	if err := m.store.SetFullTranslation(translationID, fullTranslation); err != nil {
		_ = m.store.Fail(translationID, "Failed to store full translation: "+err.Error())
		return
	}

	queued, err := m.segmentInputBySentence(ctx, sentences)
	if err != nil {
		msg := err.Error()
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		_ = m.store.Fail(translationID, "Failed to segment: "+msg)
		return
	}
	total := len(queued)
	if total == 0 {
		_ = m.store.Fail(translationID, "No translatable segments found")
		return
	}

	startIndex := item.Progress
	if item.Status == "pending" {
		startIndex = 0
		sentenceInits := make([]translation.SentenceInit, len(sentences))
		for i, s := range sentences {
			sentenceInits[i] = translation.SentenceInit{Indent: s.Indent, Separator: s.Separator}
		}
		if err := m.store.SetProcessing(translationID, total, sentenceInits); err != nil {
			return
		}
	}

	if startIndex >= len(queued) {
		if err := m.store.Complete(translationID); err != nil {
			_ = m.store.Fail(translationID, "Failed to complete translation")
		}
		return
	}

	for idx := startIndex; idx < len(queued); idx++ {
		work := queued[idx]
		translated, err := m.provider.TranslateSegments(ctx, []string{work.Segment}, work.SentenceText)
		if err != nil || len(translated) == 0 {
			_ = m.store.Fail(translationID, "Failed to translate segments")
			return
		}
		segmentResult := translated[0]
		if _, _, err := m.store.AddProgressSegment(translationID, segmentResult, work.SentenceIndex); err != nil {
			_ = m.store.Fail(translationID, "Failed to update translation progress")
			return
		}
		time.Sleep(15 * time.Millisecond)
	}

	if err := m.store.Complete(translationID); err != nil {
		_ = m.store.Fail(translationID, "Failed to complete translation")
	}
}
```

- [ ] **Step 2: Replace the goroutine body in `StartProcessing`**

Find the `go func(item translation.Translation) { ... }(item)` block in `StartProcessing`. This is currently lines ~113–173 and contains the full pipeline (sentence splitting, `TranslateFull`, `segmentInputBySentence`, `SetProcessing`, and the final `m.runJob(...)` call). **Delete the entire body** — everything from `ctx := context.Background()` through `m.runJob(ctx, translationID, queued, startIndex)` — and replace with:

```go
go func(item translation.Translation) {
	m.runJob(context.Background(), translationID, item)
}(item)
```

The `m.removeRunning(translationID)` calls that were scattered through the old goroutine body are eliminated: `runJob` now handles cleanup via `defer m.removeRunning(translationID)`.

- [ ] **Step 3: Run the full existing test suite**

```bash
cd server && go test ./internal/queue/ -v
```

Expected: all 6 existing tests PASS (`TestQueueProgressLifecycle`, `TestQueueProgressSurvivesManagerRestart`, `TestResumeRestartableJobsCompletesPendingTranslation`, `TestTranslateFullFailureFails`, `TestReprocessingPreservesFullTranslation`, `TestReprocessingGeneratesFullTranslationWhenAbsent`).

- [ ] **Step 4: Commit**

```bash
cd server && gofmt -w internal/queue/manager.go
git add server/internal/queue/manager.go
git commit -m "refactor: expand runJob to own full pipeline, add lease renewal goroutine"
```

---

## Task 4: Add renewal goroutine to `StartReprocessing`

**Files:**
- Modify: `server/internal/queue/manager.go`

`StartReprocessing` uses `go func() { ... }()` with no parameters — `translationID` is captured from the enclosing scope (unlike `StartProcessing` which passes `item` as a goroutine argument). The renewal pattern is otherwise identical.

- [ ] **Step 1: Add the renewal goroutine at the top of the `StartReprocessing` goroutine body**

Immediately after `ctx := context.Background()` inside the `go func() { ... }()` block, insert:

```go
// Renewal goroutine: same pattern as runJob.
// defer cancelRenew() handles all exit paths — no per-return call needed.
renewCtx, cancelRenew := context.WithCancel(ctx)
defer cancelRenew()
go func() {
	ticker := time.NewTicker(leaseRenewalInterval)
	defer ticker.Stop()
	consecutiveFailures := 0
	for {
		select {
		case <-ticker.C:
			if err := m.store.RenewLease(translationID, jobLeaseDuration); err != nil {
				consecutiveFailures++
				// TODO: fail job if consecutiveFailures exceeds threshold.
				log.Printf("lease renewal failed for %s (consecutive failures: %d): %v",
					translationID, consecutiveFailures, err)
			} else {
				consecutiveFailures = 0
			}
		case <-renewCtx.Done():
			return
		}
	}
}()
```

Leave the rest of `StartReprocessing` unchanged.

- [ ] **Step 2: Run the reprocessing tests**

```bash
cd server && go test ./internal/queue/ -run 'TestReprocessing' -v
```

Expected: both reprocessing tests PASS.

- [ ] **Step 3: Commit**

```bash
cd server && gofmt -w internal/queue/manager.go
git add server/internal/queue/manager.go
git commit -m "feat: add lease renewal goroutine to StartReprocessing"
```

---

## Task 5: Add `StartBackgroundScanner` and tests

**Files:**
- Modify: `server/internal/queue/manager.go`
- Modify: `server/internal/queue/manager_test.go`

- [ ] **Step 1: Write failing tests in `manager_test.go`**

Add both tests to `server/internal/queue/manager_test.go`. The double-process test uses `store.GetJobAttempts` (added in Task 1) to verify the job was claimed exactly once:

```go
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
```

- [ ] **Step 2: Run to confirm tests fail**

```bash
cd server && go test ./internal/queue/ -run 'TestScanner' -v
```

Expected: compile error — `StartBackgroundScanner undefined`.

- [ ] **Step 3: Implement `StartBackgroundScanner` in `manager.go`**

Add after `ResumeRestartableJobs`:

```go
// StartBackgroundScanner periodically scans for jobs with expired leases and
// re-queues them via ResumeRestartableJobs. The in-memory running map prevents
// double-processing jobs that are still active.
//
// Pass context.Background() for a process-lifetime scanner (killed when the
// process exits). Use a cancellable context for graceful shutdown.
func (m *Manager) StartBackgroundScanner(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(expiredLeaseScanInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.ResumeRestartableJobs()
			case <-ctx.Done():
				return
			}
		}
	}()
}
```

- [ ] **Step 4: Run all queue tests**

```bash
cd server && go test ./internal/queue/ -v
```

Expected: all tests PASS including the two new ones.

- [ ] **Step 5: Commit**

```bash
cd server && gofmt -w internal/queue/manager.go internal/queue/manager_test.go
git add server/internal/queue/manager.go server/internal/queue/manager_test.go
git commit -m "feat: add StartBackgroundScanner and scanner tests"
```

---

## Task 6: Wire scanner into `server.go` and verify full suite

**Files:**
- Modify: `server/internal/http/server.go`

- [ ] **Step 1: Add `"context"` to the import block and call `StartBackgroundScanner`**

`"context"` is not currently imported in `server.go`. Add it to the import block, then add one line after `manager.ResumeRestartableJobs()` in `initDependencies`:

```go
manager.ResumeRestartableJobs()
manager.StartBackgroundScanner(context.Background())
```

- [ ] **Step 2: Run full test suite**

```bash
cd server && go test ./...
```

Expected: all tests PASS.

- [ ] **Step 3: gofmt and final commit**

```bash
cd server && gofmt -w .
git add server/internal/http/server.go
git commit -m "feat: wire StartBackgroundScanner into server startup"
```
