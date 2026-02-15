package translation

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *SRSStore) SaveVocabItem(headword string, pinyin string, english string, textID *string, segmentID *string, snippet *string, status string) (string, error) {
	if strings.TrimSpace(headword) == "" {
		return "", errors.New("headword is required")
	}
	if status == "" {
		status = "learning"
	}
	if status != "unknown" && status != "learning" && status != "known" {
		return "", errors.New("Invalid status")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	id, _ := newID()
	if _, err := s.db.Exec(
		`INSERT OR IGNORE INTO vocab_items (id, headword, pinyin, english, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, strings.TrimSpace(headword), strings.TrimSpace(pinyin), strings.TrimSpace(english), status, now, now,
	); err != nil {
		return "", fmt.Errorf("insert vocab item: %w", err)
	}
	var resolvedID string
	if err := s.db.QueryRow(
		`SELECT id FROM vocab_items WHERE headword = ? AND pinyin = ? AND english = ?`,
		strings.TrimSpace(headword), strings.TrimSpace(pinyin), strings.TrimSpace(english),
	).Scan(&resolvedID); err != nil {
		return "", fmt.Errorf("resolve vocab item id: %w", err)
	}
	if _, err := s.db.Exec(`UPDATE vocab_items SET updated_at = ? WHERE id = ?`, now, resolvedID); err != nil {
		return "", fmt.Errorf("touch vocab item: %w", err)
	}
	occID, _ := newID()
	var textIDVal any
	var segmentIDVal any
	var snippetVal string
	if textID != nil {
		textIDVal = *textID
	}
	if segmentID != nil {
		segmentIDVal = *segmentID
	}
	if snippet != nil {
		snippetVal = *snippet
	}
	if _, err := s.db.Exec(
		`INSERT INTO vocab_occurrences (id, vocab_item_id, text_id, segment_id, snippet, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		occID, resolvedID, textIDVal, segmentIDVal, snippetVal, now,
	); err != nil {
		return "", fmt.Errorf("insert vocab occurrence: %w", err)
	}
	if _, err := s.db.Exec(
		`INSERT OR IGNORE INTO srs_state (vocab_item_id, due_at, interval_days, ease, reps, lapses, last_reviewed_at)
		 VALUES (?, ?, 0, 2.5, 0, 0, ?)`,
		resolvedID, now, now,
	); err != nil {
		return "", fmt.Errorf("init srs state: %w", err)
	}
	return resolvedID, nil
}

func (s *SRSStore) UpdateVocabStatus(vocabItemID string, status string) error {
	if status != "unknown" && status != "learning" && status != "known" {
		return errors.New("Invalid status")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`UPDATE vocab_items SET status = ?, updated_at = ? WHERE id = ?`, status, now, vocabItemID)
	if err != nil {
		return fmt.Errorf("update vocab status: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SRSStore) RecordLookup(vocabItemID string) (VocabSRSInfo, bool) {
	row := s.db.QueryRow(`SELECT id, headword, pinyin, english, status FROM vocab_items WHERE id = ?`, vocabItemID)
	var rec VocabSRSInfo
	if err := row.Scan(&rec.VocabItemID, &rec.Headword, &rec.Pinyin, &rec.English, &rec.Status); err != nil {
		return VocabSRSInfo{}, false
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	lookupID, _ := newID()
	_, _ = s.db.Exec(`INSERT INTO vocab_lookups (id, vocab_item_id, looked_up_at) VALUES (?, ?, ?)`, lookupID, vocabItemID, now)
	_, _ = s.db.Exec(`INSERT OR IGNORE INTO srs_state (vocab_item_id, due_at, interval_days, ease, reps, lapses, last_reviewed_at)
		VALUES (?, ?, 0, 2.5, 0, 0, ?)`, vocabItemID, now, now)
	_, _ = s.db.Exec(`UPDATE srs_state SET last_reviewed_at = ? WHERE vocab_item_id = ?`, now, vocabItemID)
	infoList, _ := s.GetVocabSRSInfo([]string{rec.Headword})
	if len(infoList) > 0 {
		return infoList[0], true
	}
	rec.Opacity = 1
	return rec, true
}

func (s *SRSStore) GetVocabSRSInfo(headwords []string) ([]VocabSRSInfo, error) {
	filtered := make([]string, 0, len(headwords))
	for _, h := range headwords {
		h = strings.TrimSpace(h)
		if h != "" {
			filtered = append(filtered, h)
		}
	}
	if len(filtered) == 0 {
		return []VocabSRSInfo{}, nil
	}
	placeholders := strings.Repeat("?,", len(filtered))
	placeholders = strings.TrimSuffix(placeholders, ",")
	args := make([]any, len(filtered))
	for i, h := range filtered {
		args[i] = h
	}
	rows, err := s.db.Query(
		fmt.Sprintf(`SELECT vi.id, vi.headword, vi.pinyin, vi.english, vi.status, ss.last_reviewed_at
			FROM vocab_items vi
			LEFT JOIN srs_state ss ON vi.id = ss.vocab_item_id
			WHERE vi.headword IN (%s)`, placeholders),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("query vocab srs info: %w", err)
	}
	defer rows.Close()
	now := time.Now().UTC()
	out := make([]VocabSRSInfo, 0)
	for rows.Next() {
		var info VocabSRSInfo
		var lastReviewed sql.NullString
		if err := rows.Scan(&info.VocabItemID, &info.Headword, &info.Pinyin, &info.English, &info.Status, &lastReviewed); err != nil {
			return nil, fmt.Errorf("scan vocab srs info: %w", err)
		}
		recentCount := 0
		_ = s.db.QueryRow(
			`SELECT COUNT(*) FROM vocab_lookups WHERE vocab_item_id = ? AND looked_up_at >= ?`,
			info.VocabItemID,
			now.Add(-7*24*time.Hour).Format(time.RFC3339Nano),
		).Scan(&recentCount)
		info.IsStruggling = recentCount >= 3
		if !lastReviewed.Valid {
			info.Opacity = 0
		} else {
			lastDt, parseErr := time.Parse(time.RFC3339Nano, lastReviewed.String)
			if parseErr != nil {
				info.Opacity = 1
			} else {
				days := now.Sub(lastDt).Hours() / 24
				base := 1 - days/30
				if base < 0 {
					base = 0
				}
				if info.IsStruggling && base < 0.3 {
					base = 0.3
				}
				info.Opacity = base
			}
		}
		out = append(out, info)
	}
	return out, nil
}

func (s *SRSStore) GetReviewQueue(limit int) ([]ReviewCard, error) {
	if limit <= 0 {
		limit = 10
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rows, err := s.db.Query(
		`SELECT vi.id, vi.headword, vi.pinyin, vi.english
		 FROM vocab_items vi
		 JOIN srs_state ss ON vi.id = ss.vocab_item_id
		 WHERE vi.status = 'learning' AND (ss.due_at IS NULL OR ss.due_at <= ?)
		 ORDER BY ss.due_at ASC
		 LIMIT ?`,
		now,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query review queue: %w", err)
	}
	defer rows.Close()
	out := make([]ReviewCard, 0)
	for rows.Next() {
		var card ReviewCard
		if err := rows.Scan(&card.VocabItemID, &card.Headword, &card.Pinyin, &card.English); err != nil {
			return nil, fmt.Errorf("scan review card: %w", err)
		}
		snippetRows, err := s.db.Query(`SELECT snippet FROM vocab_occurrences WHERE vocab_item_id = ? AND snippet != '' ORDER BY created_at DESC LIMIT 3`, card.VocabItemID)
		if err == nil {
			for snippetRows.Next() {
				var snip string
				_ = snippetRows.Scan(&snip)
				card.Snippets = append(card.Snippets, snip)
			}
			_ = snippetRows.Close()
		}
		out = append(out, card)
	}
	return out, nil
}

func (s *SRSStore) GetDueCount() int {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var cnt int
	_ = s.db.QueryRow(
		`SELECT COUNT(*) FROM vocab_items vi
		 JOIN srs_state ss ON vi.id = ss.vocab_item_id
		 WHERE vi.status = 'learning' AND (ss.due_at IS NULL OR ss.due_at <= ?)`,
		now,
	).Scan(&cnt)
	return cnt
}

func (s *SRSStore) RecordReviewAnswer(vocabItemID string, grade int) (ReviewAnswerResult, bool, error) {
	if grade < 0 || grade > 2 {
		return ReviewAnswerResult{}, false, errors.New("Grade must be 0, 1, or 2")
	}
	var exists int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM vocab_items WHERE id = ?`, vocabItemID).Scan(&exists)
	if exists == 0 {
		return ReviewAnswerResult{}, false, nil
	}
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)
	var dueAt sql.NullString
	var interval, ease float64
	var reps, lapses int
	err := s.db.QueryRow(`SELECT due_at, interval_days, ease, reps, lapses FROM srs_state WHERE vocab_item_id = ?`, vocabItemID).
		Scan(&dueAt, &interval, &ease, &reps, &lapses)
	if err != nil {
		_, _ = s.db.Exec(`INSERT OR IGNORE INTO srs_state (vocab_item_id, due_at, interval_days, ease, reps, lapses, last_reviewed_at) VALUES (?, ?, 0, 2.5, 0, 0, ?)`, vocabItemID, nowStr, nowStr)
		dueAt = sql.NullString{String: nowStr, Valid: true}
		interval = 0
		ease = 2.5
		reps = 0
		lapses = 0
	}
	newInterval := interval
	newEase := ease
	newReps := reps
	newLapses := lapses
	switch grade {
	case 0:
		newInterval = 0
		newEase = maxFloat(1.3, ease-0.2)
		newReps = 0
		newLapses++
	case 1:
		if reps == 0 {
			newInterval = 0.5
		} else {
			newInterval = interval * 1.2
		}
		newEase = maxFloat(1.3, ease-0.15)
		newReps++
	case 2:
		if reps == 0 {
			newInterval = 1
		} else if reps == 1 {
			newInterval = 6
		} else {
			newInterval = interval * ease
		}
		newReps++
	}
	nextDue := now.Add(time.Duration(newInterval*24) * time.Hour).Format(time.RFC3339Nano)
	_, _ = s.db.Exec(`UPDATE srs_state SET due_at = ?, interval_days = ?, ease = ?, reps = ?, lapses = ?, last_reviewed_at = ? WHERE vocab_item_id = ?`,
		nextDue, newInterval, newEase, newReps, newLapses, nowStr, vocabItemID)
	nextDuePtr := nextDue
	return ReviewAnswerResult{
		VocabItemID:  vocabItemID,
		NextDueAt:    &nextDuePtr,
		IntervalDays: newInterval,
		RemainingDue: s.GetDueCount(),
	}, true, nil
}

func (s *SRSStore) CountVocabByStatus(status string) int {
	var cnt int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM vocab_items WHERE status = ?`, status).Scan(&cnt)
	return cnt
}

func (s *SRSStore) CountTotalVocab() int {
	var cnt int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM vocab_items`).Scan(&cnt)
	return cnt
}

func (s *SRSStore) ExportProgressJSON() (string, error) {
	bundle := map[string]any{
		"schema_version": 1,
		"exported_at":    time.Now().UTC().Format(time.RFC3339Nano),
	}
	type tableDump struct {
		query string
		key   string
	}
	dumps := []tableDump{
		{query: "SELECT id, headword, pinyin, english, status, created_at, updated_at FROM vocab_items ORDER BY created_at", key: "vocab_items"},
		{query: "SELECT vocab_item_id, due_at, interval_days, ease, reps, lapses, last_reviewed_at FROM srs_state", key: "srs_state"},
		{query: "SELECT id, vocab_item_id, looked_up_at FROM vocab_lookups ORDER BY looked_up_at", key: "vocab_lookups"},
	}
	for _, d := range dumps {
		rows, err := s.db.Query(d.query)
		if err != nil {
			return "", err
		}
		arr, err := rowsToMaps(rows)
		_ = rows.Close()
		if err != nil {
			return "", err
		}
		bundle[d.key] = arr
	}
	b, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *SRSStore) ImportProgressJSON(input string) (map[string]int, error) {
	var data map[string]any
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return nil, fmt.Errorf("Invalid JSON: %w", err)
	}
	getArr := func(key string) ([]map[string]any, error) {
		raw, ok := data[key]
		if !ok {
			return nil, fmt.Errorf("Missing '%s' field", key)
		}
		list, ok := raw.([]any)
		if !ok {
			return nil, fmt.Errorf("'%s' must be a list", key)
		}
		out := make([]map[string]any, 0, len(list))
		for _, it := range list {
			obj, ok := it.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%s entry must be object", key)
			}
			out = append(out, obj)
		}
		return out, nil
	}
	vocabItems, err := getArr("vocab_items")
	if err != nil {
		return nil, err
	}
	srsState, err := getArr("srs_state")
	if err != nil {
		return nil, err
	}
	lookups, err := getArr("vocab_lookups")
	if err != nil {
		return nil, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		"DELETE FROM vocab_lookups",
		"DELETE FROM srs_state",
		"DELETE FROM vocab_occurrences",
		"DELETE FROM vocab_items",
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return nil, err
		}
	}
	for _, item := range vocabItems {
		_, err := tx.Exec(`INSERT INTO vocab_items (id, headword, pinyin, english, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			toString(item["id"]), toString(item["headword"]), toString(item["pinyin"]), toString(item["english"]), toString(item["status"]), toString(item["created_at"]), toString(item["updated_at"]))
		if err != nil {
			return nil, err
		}
	}
	for _, item := range srsState {
		_, err := tx.Exec(`INSERT INTO srs_state (vocab_item_id, due_at, interval_days, ease, reps, lapses, last_reviewed_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			toString(item["vocab_item_id"]), nullableString(item["due_at"]), toFloat(item["interval_days"]), toFloat(item["ease"]), toInt(item["reps"]), toInt(item["lapses"]), nullableString(item["last_reviewed_at"]))
		if err != nil {
			return nil, err
		}
	}
	for _, item := range lookups {
		_, err := tx.Exec(`INSERT INTO vocab_lookups (id, vocab_item_id, looked_up_at) VALUES (?, ?, ?)`,
			toString(item["id"]), toString(item["vocab_item_id"]), toString(item["looked_up_at"]))
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]int{
		"vocab_items":   len(vocabItems),
		"srs_state":     len(srsState),
		"vocab_lookups": len(lookups),
	}, nil
}

func maxFloat(a float64, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
