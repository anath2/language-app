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
