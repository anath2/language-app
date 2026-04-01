# Lease Renewal and Periodic Expired-Lease Scanner

**Date:** 2026-04-01
**Branch:** fix/translation-page
**Status:** Approved

## Problem

The translation job queue uses a lease-based ownership model to prevent duplicate processing and enable crash recovery. Two gaps exist in the current implementation:

1. **No lease renewal.** The lease is set once (30s) when a job is claimed and never extended. A document large enough to take longer than 30s to process will have an expired lease while its goroutine is still running. On the next server startup, `ListRestartableTranslationIDs` will surface it as restartable, causing duplicate processing.

2. **No periodic expired-lease scan.** `ResumeRestartableJobs` is called only at server startup. If a goroutine panics mid-job while the server is running, the job is stuck in `leased` state with an expired lease until the next restart. Users see the translation perpetually "processing" with no recovery.

## Design

### Approach

Option A: renewal goroutine per job + background scanner on Manager.

- Renewal is co-located with job execution. The renewal goroutine is cancelled entirely by `defer cancelRenew()` on the derived `renewCtx` — not by the parent `ctx`, which is `context.Background()` and never closes. This is correct and intentional: the only signal that stops renewal is the job worker exiting.
- The scanner is a background goroutine on `Manager` that periodically calls the existing `ResumeRestartableJobs`. The in-memory `running` map already prevents double-processing with no new coordination required.

### Constants

In `server/internal/queue/manager.go`:

```go
const jobLeaseDuration        = 5 * time.Minute   // was 30s
const leaseRenewalInterval    = 100 * time.Second  // how often active workers renew (~1/3 of lease)
const expiredLeaseScanInterval = 30 * time.Second  // how often the background scanner polls for expired leases
```

### Store Layer

New method on `TranslationStore`, implemented in `server/internal/translation/store_jobs.go` alongside `ClaimTranslationJob` and `ListRestartableTranslationIDs`:

```go
func (s *TranslationStore) RenewLease(translationID string, d time.Duration) error
```

- `UPDATE translation_jobs SET lease_until = ?, updated_at = ? WHERE translation_id = ? AND state = 'leased'`
- No retry loop — intentionally asymmetric with `ClaimTranslationJob`. `ClaimTranslationJob` retries because a lost claim means the job isn't processed; a missed renewal is harmless because the 5-minute lease provides ample headroom and the next tick retries. Add a comment in the implementation noting this.
- Added to the `translationStore` interface in `manager.go`

### Renewal Goroutine — Placement

`runJob` is expanded to encompass the full job pipeline — `TranslateFull`, `segmentInputBySentence`, `SetProcessing`, and the segment loop — so that all work for a job lives inside one function. The renewal goroutine is placed at the top of `runJob`, before any slow work begins. This is the natural home for renewal: it covers the entire job lifetime from the first LLM call to `Complete()`.

The anonymous goroutine in `StartProcessing` becomes thin: claim the job, launch `go runJob(...)`. Same for `StartReprocessing`.

**`runJob` renewal goroutine** (top of function, before `TranslateFull`):

```go
func (m *Manager) runJob(ctx context.Context, translationID string, item translation.Translation) {
    defer m.removeRunning(translationID)

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
                    // TODO: if consecutiveFailures exceeds a threshold (e.g. 3), fail the job
                    // to avoid a zombie worker holding a claim it can no longer renew.
                } else {
                    consecutiveFailures = 0
                }
            case <-renewCtx.Done():
                return
            }
        }
    }()

    // TranslateFull, segmentInputBySentence, SetProcessing, segment loop, Complete ...
}
```

**`StartReprocessing`**: same renewal pattern at the top of the goroutine body. `StartReprocessing`'s goroutine uses `go func() { ... }()` (no parameters; `translationID` captured from enclosing scope).

`defer cancelRenew()` fires on all exit paths. There is a brief window between `ClaimTranslationJob` (called before the goroutine launches) and goroutine start; this is acceptable given the 5-minute lease.

### Background Scanner

New method on `Manager`:

```go
func (m *Manager) StartBackgroundScanner(ctx context.Context)
```

- Spawns a goroutine that ticks every `expiredLeaseScanInterval` (30s)
- Each tick calls `m.ResumeRestartableJobs()`
- Shuts down when `ctx` is cancelled

Called from `initDependencies` in `server/internal/http/server.go`, immediately after the existing `manager.ResumeRestartableJobs()` call:

```go
manager.StartBackgroundScanner(context.Background())
```

`context.Background()` is used intentionally — it never cancels, so the scanner runs for the lifetime of the process. The server currently has no graceful shutdown mechanism; this can be upgraded to a signal-anchored context when that is added.

## Data Flow

```
Server startup:
  manager.ResumeRestartableJobs()           ← immediate recovery of pre-existing expired leases
  manager.StartBackgroundScanner(ctx)       ← ongoing recovery every 30s

POST /api/translations → StartProcessing(id):
  ClaimTranslationJob(id, 5min)
  go runJob(ctx, translationID, item):
    defer cancelRenew()
    go renewalLoop                          ← ticks every 100s until runJob exits
    TranslateFull (LLM call)
    segmentInputBySentence (LLM call)
    SetProcessing
    segment loop → Complete()

Background scanner tick (every 30s):
  ListRestartableTranslationIDs()
    → state='pending' OR (state='leased' AND lease_until < now)
  StartProcessing(id) for each
    → running map check skips jobs already active in this process
```

## Error Handling

- Renewal errors are silently ignored (`_ = m.store.RenewLease(...)`). A single failed renewal is not fatal; the lease has 5 minutes of headroom and the next tick retries.
- If all renewals fail (e.g. DB unavailable), the lease expires after 5 minutes and the scanner re-queues the job.
- Scanner errors are logged (existing `log.Printf` in `ResumeRestartableJobs`) but do not crash the server.
- Note: the `attempts` column in `translation_jobs` is incremented on each `ClaimTranslationJob` call but no max-attempts guard is enforced. A repeatedly-panicking job is re-queued indefinitely. A max-attempts limit is a future enhancement and is out of scope for this change.

## Testing

All tests in `package queue` (white-box, consistent with existing `manager_test.go`).

- **`RenewLease` store method**: unit test in `store_jobs_test.go` — verify `lease_until` is updated for a leased job; verify the update is a no-op when `state` is `completed` or `failed`.
- **Scanner does not double-process active jobs**: add `translationID` to `m.running` directly, call `ResumeRestartableJobs`, verify `ClaimTranslationJob` is not called a second time (assert via store mock call log or `running` map state).
- **Scanner recovers a stuck job**: insert a job with `state='leased'` and `lease_until` in the past, call `ResumeRestartableJobs`, verify the job is re-claimed and processing begins.
- **Renewal keeps lease alive**: use short test-only lease duration and renewal interval constants, start a job with a mock provider that stalls briefly, verify `lease_until` in the DB advances during processing.
