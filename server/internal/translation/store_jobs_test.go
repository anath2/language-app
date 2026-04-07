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
