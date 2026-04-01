package translation

import (
	"fmt"
	"time"
)

func (s *TranslationStore) ListRestartableTranslationIDs() ([]string, error) {
	nowStr := time.Now().UTC().Format(time.RFC3339Nano)
	rows, err := s.db.Query(
		`SELECT translation_id FROM translation_jobs
		 WHERE state = 'pending'
		    OR (state = 'leased' AND (lease_until IS NULL OR lease_until < ?))
		 ORDER BY created_at ASC`,
		nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("list restartable translations: %w", err)
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan restartable translation id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate restartable translations: %w", err)
	}

	return ids, nil
}

func (s *TranslationStore) ClaimTranslationJob(translationID string, leaseDuration time.Duration) (bool, error) {
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)
	leaseUntil := now.Add(leaseDuration).Format(time.RFC3339Nano)

	for i := 0; i < 8; i++ {
		res, err := s.db.Exec(
			`UPDATE translation_jobs
			 SET state = 'leased',
			     attempts = attempts + 1,
			     lease_until = ?,
			     updated_at = ?,
			     last_error = NULL
			 WHERE translation_id = ?
			   AND (
			     state = 'pending'
			     OR (state = 'leased' AND (lease_until IS NULL OR lease_until < ?))
			   )`,
			leaseUntil,
			nowStr,
			translationID,
			nowStr,
		)
		if err != nil {
			if isDBLocked(err) {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			return false, fmt.Errorf("claim translation job: %w", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return false, fmt.Errorf("claim translation job rows affected: %w", err)
		}
		return affected > 0, nil
	}

	return false, nil
}

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
