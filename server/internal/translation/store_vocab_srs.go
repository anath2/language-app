package translation

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	reviewEntitySegment   = "segment"
	reviewEntityCharacter = "character"
)

func (s *SRSStore) SaveSegment(headword string, pinyin string, english string, translationID *string, snippet *string, status string) (string, error) {
	if strings.TrimSpace(headword) == "" {
		return "", errors.New("headword is required")
	}
	if status == "" {
		status = "learning"
	}
	if !isValidStatus(status) {
		return "", errors.New("invalid status")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	id, _ := newID()
	if _, err := s.db.Exec(
		`INSERT OR IGNORE INTO saved_segments (id, headword, pinyin, english, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, strings.TrimSpace(headword), strings.TrimSpace(pinyin), strings.TrimSpace(english), status, now, now,
	); err != nil {
		return "", fmt.Errorf("insert segment: %w", err)
	}
	var segmentID string
	if err := s.db.QueryRow(
		`SELECT id FROM saved_segments WHERE headword = ? AND pinyin = ?`,
		strings.TrimSpace(headword), strings.TrimSpace(pinyin),
	).Scan(&segmentID); err != nil {
		return "", fmt.Errorf("resolve segment id: %w", err)
	}

	var translationIDVal any
	var snippetVal string
	if translationID != nil {
		translationIDVal = *translationID
	}
	if snippet != nil {
		snippetVal = *snippet
	}
	if _, err := s.db.Exec(
		`UPDATE saved_segments
		 SET updated_at = ?,
		     english = CASE WHEN ? = '' THEN english ELSE ? END,
		     status = ?,
		     last_seen_translation_id = COALESCE(?, last_seen_translation_id),
		     last_seen_snippet = CASE WHEN ? = '' THEN last_seen_snippet ELSE ? END,
		     last_seen_at = ?,
		     seen_count = seen_count + 1
		 WHERE id = ?`,
		now, strings.TrimSpace(english), strings.TrimSpace(english), status, translationIDVal, snippetVal, snippetVal, now, segmentID,
	); err != nil {
		return "", fmt.Errorf("update segment context: %w", err)
	}
	if err := s.ensureSegmentSRSState(segmentID, now); err != nil {
		return "", err
	}
	return segmentID, nil
}

func (s *SRSStore) UpdateSegmentStatus(segmentID string, status string) error {
	if !isValidStatus(status) {
		return errors.New("invalid status")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`UPDATE saved_segments SET status = ?, updated_at = ? WHERE id = ?`, status, now, segmentID)
	if err != nil {
		return fmt.Errorf("update segment status: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SRSStore) UpdateCharacterStatus(characterID string, status string) error {
	if !isValidStatus(status) {
		return errors.New("invalid status")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`UPDATE saved_characters SET status = ?, updated_at = ? WHERE id = ?`, status, now, characterID)
	if err != nil {
		return fmt.Errorf("update character status: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SRSStore) RecordLookup(segmentID string) (SegmentSRSInfo, bool) {
	row := s.db.QueryRow(`SELECT id, headword, pinyin, english, status FROM saved_segments WHERE id = ?`, segmentID)
	var rec SegmentSRSInfo
	if err := row.Scan(&rec.SegmentID, &rec.Headword, &rec.Pinyin, &rec.English, &rec.Status); err != nil {
		return SegmentSRSInfo{}, false
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	lookupID, _ := newID()
	_, _ = s.db.Exec(`INSERT INTO vocab_lookups (id, segment_id, looked_up_at) VALUES (?, ?, ?)`, lookupID, segmentID, now)
	_ = s.ensureSegmentSRSState(segmentID, now)
	_, _ = s.db.Exec(`UPDATE srs_state SET last_reviewed_at = ? WHERE segment_id = ?`, now, segmentID)
	infoList, _ := s.GetSegmentSRSInfo([]string{rec.Headword})
	if len(infoList) > 0 {
		return infoList[0], true
	}
	rec.Opacity = 1
	return rec, true
}

func (s *SRSStore) GetSegmentSRSInfo(headwords []string) ([]SegmentSRSInfo, error) {
	filtered := make([]string, 0, len(headwords))
	for _, h := range headwords {
		h = strings.TrimSpace(h)
		if h != "" {
			filtered = append(filtered, h)
		}
	}
	if len(filtered) == 0 {
		return []SegmentSRSInfo{}, nil
	}
	placeholders := strings.Repeat("?,", len(filtered))
	placeholders = strings.TrimSuffix(placeholders, ",")
	args := make([]any, len(filtered))
	for i, h := range filtered {
		args[i] = h
	}
	rows, err := s.db.Query(
		fmt.Sprintf(`SELECT ss.id, ss.headword, ss.pinyin, ss.english, ss.status, st.last_reviewed_at, st.interval_days, st.due_at
			FROM saved_segments ss
			LEFT JOIN srs_state st ON ss.id = st.segment_id
			WHERE ss.headword IN (%s)`, placeholders),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("query segment srs info: %w", err)
	}
	defer rows.Close()
	now := time.Now().UTC()
	out := make([]SegmentSRSInfo, 0)
	for rows.Next() {
		var info SegmentSRSInfo
		var lastReviewed sql.NullString
		var intervalDays sql.NullFloat64
		var dueAt sql.NullString
		if err := rows.Scan(&info.SegmentID, &info.Headword, &info.Pinyin, &info.English, &info.Status, &lastReviewed, &intervalDays, &dueAt); err != nil {
			return nil, fmt.Errorf("scan segment srs info: %w", err)
		}
		if intervalDays.Valid {
			info.IntervalDays = intervalDays.Float64
		}
		if dueAt.Valid {
			info.NextDueAt = &dueAt.String
		}
		recentCount := 0
		_ = s.db.QueryRow(
			`SELECT COUNT(*) FROM vocab_lookups WHERE segment_id = ? AND looked_up_at >= ?`,
			info.SegmentID,
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

func (s *SRSStore) GetSegmentReviewQueue(limit int) ([]SegmentReviewCard, error) {
	if limit <= 0 {
		limit = 10
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rows, err := s.db.Query(
		`SELECT ss.id, ss.headword, ss.pinyin, ss.english
		 FROM saved_segments ss
		 JOIN srs_state st ON ss.id = st.segment_id
		 WHERE ss.status = 'learning' AND (st.due_at IS NULL OR st.due_at <= ?)
		 ORDER BY st.due_at ASC
		 LIMIT ?`,
		now,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query review queue: %w", err)
	}
	defer rows.Close()
	out := make([]SegmentReviewCard, 0)
	for rows.Next() {
		var card SegmentReviewCard
		if err := rows.Scan(&card.SegmentID, &card.Headword, &card.Pinyin, &card.English); err != nil {
			return nil, fmt.Errorf("scan review card: %w", err)
		}
		var snippet sql.NullString
		if err := s.db.QueryRow(`SELECT last_seen_snippet FROM saved_segments WHERE id = ?`, card.SegmentID).Scan(&snippet); err == nil && snippet.Valid && snippet.String != "" {
			card.Snippets = []string{snippet.String}
		}
		out = append(out, card)
	}
	return out, nil
}

func (s *SRSStore) GetSegmentDueCount() int {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var cnt int
	_ = s.db.QueryRow(
		`SELECT COUNT(*) FROM saved_segments ss
		 JOIN srs_state st ON ss.id = st.segment_id
		 WHERE ss.status = 'learning' AND (st.due_at IS NULL OR st.due_at <= ?)`,
		now,
	).Scan(&cnt)
	return cnt
}

func (s *SRSStore) RecordReviewAnswer(entityID string, entityType string, grade int) (ReviewAnswerResult, bool, error) {
	if grade < 0 || grade > 2 {
		return ReviewAnswerResult{}, false, errors.New("grade must be 0, 1, or 2")
	}
	entityType = strings.TrimSpace(entityType)
	if entityType == "" {
		entityType = reviewEntitySegment
	}
	if entityType != reviewEntitySegment && entityType != reviewEntityCharacter {
		return ReviewAnswerResult{}, false, errors.New("invalid entity type")
	}

	entityExistsQuery := `SELECT 1 FROM saved_segments WHERE id = ?`
	if entityType == reviewEntityCharacter {
		entityExistsQuery = `SELECT 1 FROM saved_characters WHERE id = ?`
	}
	var exists int
	err := s.db.QueryRow(entityExistsQuery, entityID).Scan(&exists)
	if err != nil {
		return ReviewAnswerResult{}, false, nil
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)
	var dueAt sql.NullString
	var interval, ease float64
	var reps, lapses int
	stateQuery := `SELECT due_at, interval_days, ease, reps, lapses FROM srs_state WHERE segment_id = ?`
	if entityType == reviewEntityCharacter {
		stateQuery = `SELECT due_at, interval_days, ease, reps, lapses FROM srs_state WHERE character_id = ?`
	}
	err = s.db.QueryRow(stateQuery, entityID).
		Scan(&dueAt, &interval, &ease, &reps, &lapses)
	if err != nil {
		if entityType == reviewEntityCharacter {
			_ = s.ensureCharacterSRSState(entityID, nowStr)
		} else {
			_ = s.ensureSegmentSRSState(entityID, nowStr)
		}
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
	nextDue := now.Add(time.Duration(newInterval * 24 * float64(time.Hour))).Format(time.RFC3339Nano)
	updateQuery := `UPDATE srs_state SET due_at = ?, interval_days = ?, ease = ?, reps = ?, lapses = ?, last_reviewed_at = ? WHERE segment_id = ?`
	if entityType == reviewEntityCharacter {
		updateQuery = `UPDATE srs_state SET due_at = ?, interval_days = ?, ease = ?, reps = ?, lapses = ?, last_reviewed_at = ? WHERE character_id = ?`
	}
	_, _ = s.db.Exec(updateQuery, nextDue, newInterval, newEase, newReps, newLapses, nowStr, entityID)
	nextDuePtr := nextDue
	remainingDue := s.GetSegmentDueCount()
	var segmentID *string
	var characterID *string
	if entityType == reviewEntityCharacter {
		remainingDue = s.GetCharacterDueCount()
		characterID = &entityID
	} else {
		segmentID = &entityID
	}
	return ReviewAnswerResult{
		SegmentID:    segmentID,
		CharacterID:  characterID,
		NextDueAt:    &nextDuePtr,
		IntervalDays: newInterval,
		RemainingDue: remainingDue,
	}, true, nil
}

func (s *SRSStore) CountSegmentsByStatus(status string) int {
	var cnt int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM saved_segments WHERE status = ?`, status).Scan(&cnt)
	return cnt
}

func (s *SRSStore) CountTotalSegments() int {
	var cnt int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM saved_segments`).Scan(&cnt)
	return cnt
}

func (s *SRSStore) ExportProgressJSON() (string, error) {
	bundle := map[string]any{
		"schema_version": 2,
		"exported_at":    time.Now().UTC().Format(time.RFC3339Nano),
	}
	type tableDump struct {
		query string
		key   string
	}
	dumps := []tableDump{
		{query: "SELECT id, headword, pinyin, english, status, created_at, updated_at, last_seen_translation_id, last_seen_snippet, last_seen_at, seen_count FROM saved_segments ORDER BY created_at", key: "saved_segments"},
		{query: "SELECT id, character, pinyin, english, status, created_at, updated_at FROM saved_characters ORDER BY created_at", key: "saved_characters"},
		{query: "SELECT id, character_id, segment, segment_pinyin, segment_translation, created_at FROM character_segment_links ORDER BY created_at", key: "character_segment_links"},
		{query: "SELECT id, segment_id, character_id, due_at, interval_days, ease, reps, lapses, last_reviewed_at FROM srs_state", key: "srs_state"},
		{query: "SELECT id, segment_id, character_id, looked_up_at FROM vocab_lookups ORDER BY looked_up_at", key: "vocab_lookups"},
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
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	getArr := func(key string) ([]map[string]any, error) {
		raw, ok := data[key]
		if !ok {
			return nil, fmt.Errorf("missing '%s' field", key)
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
	getArrOptional := func(key string) []map[string]any {
		raw, ok := data[key]
		if !ok {
			return nil
		}
		list, ok := raw.([]any)
		if !ok {
			return nil
		}
		out := make([]map[string]any, 0, len(list))
		for _, it := range list {
			obj, ok := it.(map[string]any)
			if ok {
				out = append(out, obj)
			}
		}
		return out
	}
	segments, err := getArr("saved_segments")
	if err != nil {
		return nil, err
	}
	characters, err := getArr("saved_characters")
	if err != nil {
		return nil, err
	}
	charSegmentLinks := getArrOptional("character_segment_links")
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
		"DELETE FROM character_segment_links",
		"DELETE FROM vocab_lookups",
		"DELETE FROM srs_state",
		"DELETE FROM saved_characters",
		"DELETE FROM saved_segments",
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return nil, err
		}
	}
	for _, item := range segments {
		_, err := tx.Exec(`INSERT INTO saved_segments (id, headword, pinyin, english, status, created_at, updated_at, last_seen_translation_id, last_seen_snippet, last_seen_at, seen_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			toString(item["id"]),
			toString(item["headword"]),
			toString(item["pinyin"]),
			toString(item["english"]),
			toString(item["status"]),
			toString(item["created_at"]),
			toString(item["updated_at"]),
			nullableString(item["last_seen_translation_id"]),
			toString(item["last_seen_snippet"]),
			nullableString(item["last_seen_at"]),
			toInt(item["seen_count"]),
		)
		if err != nil {
			return nil, err
		}
	}
	for _, item := range characters {
		_, err := tx.Exec(`INSERT INTO saved_characters (id, character, pinyin, english, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			toString(item["id"]),
			toString(item["character"]),
			toString(item["pinyin"]),
			toString(item["english"]),
			toString(item["status"]),
			toString(item["created_at"]),
			toString(item["updated_at"]),
		)
		if err != nil {
			return nil, err
		}
	}
	for _, item := range srsState {
		_, err := tx.Exec(`INSERT INTO srs_state (id, segment_id, character_id, due_at, interval_days, ease, reps, lapses, last_reviewed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			toString(item["id"]),
			nullableString(item["segment_id"]),
			nullableString(item["character_id"]),
			nullableString(item["due_at"]),
			toFloat(item["interval_days"]),
			toFloat(item["ease"]),
			toInt(item["reps"]),
			toInt(item["lapses"]),
			nullableString(item["last_reviewed_at"]),
		)
		if err != nil {
			return nil, err
		}
	}
	for _, item := range lookups {
		_, err := tx.Exec(`INSERT INTO vocab_lookups (id, segment_id, character_id, looked_up_at) VALUES (?, ?, ?, ?)`,
			toString(item["id"]),
			nullableString(item["segment_id"]),
			nullableString(item["character_id"]),
			toString(item["looked_up_at"]),
		)
		if err != nil {
			return nil, err
		}
	}
	for _, item := range charSegmentLinks {
		_, err := tx.Exec(`INSERT INTO character_segment_links (id, character_id, segment, segment_pinyin, segment_translation, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			toString(item["id"]),
			toString(item["character_id"]),
			toString(item["segment"]),
			toString(item["segment_pinyin"]),
			toString(item["segment_translation"]),
			toString(item["created_at"]),
		)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	counts := map[string]int{
		"saved_segments":   len(segments),
		"saved_characters": len(characters),
		"srs_state":        len(srsState),
		"vocab_lookups":    len(lookups),
	}
	if len(charSegmentLinks) > 0 {
		counts["character_segment_links"] = len(charSegmentLinks)
	}
	return counts, nil
}

func (s *SRSStore) ExtractAndLinkCharacters(segmentID string, segment string, segmentPinyin string, segmentEnglish string, charData []CharTranslation) error {
	runes := []rune(segment)
	cjkRunes := make([]rune, 0, len(runes))
	for _, r := range runes {
		if isCJKIdeograph(r) {
			cjkRunes = append(cjkRunes, r)
		}
	}
	if len(cjkRunes) == 0 {
		return nil
	}

	charPinyinMap := make(map[string]string, len(charData))
	for _, cd := range charData {
		charPinyinMap[cd.Char] = cd.Pinyin
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	seen := make(map[string]bool)
	for _, r := range cjkRunes {
		char := string(r)
		pinyin := strings.TrimSpace(charPinyinMap[char])
		dedupKey := char + "|" + pinyin
		if seen[dedupKey] {
			continue
		}
		seen[dedupKey] = true

		charEnglish := ""
		if len(cjkRunes) == 1 {
			charEnglish = strings.TrimSpace(segmentEnglish)
		}

		charID, _ := newID()
		_, _ = s.db.Exec(
			`INSERT OR IGNORE INTO saved_characters (id, character, pinyin, english, status, created_at, updated_at)
			 VALUES (?, ?, ?, ?, 'learning', ?, ?)`,
			charID, char, pinyin, charEnglish, now, now,
		)

		var resolvedCharID string
		if err := s.db.QueryRow(
			`SELECT id FROM saved_characters WHERE character = ? AND pinyin = ?`,
			char, pinyin,
		).Scan(&resolvedCharID); err != nil {
			continue
		}
		if err := s.ensureCharacterSRSState(resolvedCharID, now); err != nil {
			return err
		}

		linkID, _ := newID()
		_, _ = s.db.Exec(
			`INSERT OR IGNORE INTO character_segment_links (id, character_id, segment, segment_pinyin, segment_translation, created_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			linkID, resolvedCharID, strings.TrimSpace(segment), strings.TrimSpace(segmentPinyin), strings.TrimSpace(segmentEnglish), now,
		)
	}
	_ = segmentID
	return nil
}

func (s *SRSStore) GetCharacterReviewQueue(limit int) ([]CharacterReviewCard, error) {
	if limit <= 0 {
		limit = 10
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rows, err := s.db.Query(
		`SELECT sc.id, sc.character, sc.pinyin, sc.english
		 FROM saved_characters sc
		 JOIN srs_state st ON sc.id = st.character_id
		 WHERE sc.status = 'learning' AND (st.due_at IS NULL OR st.due_at <= ?)
		 ORDER BY st.due_at ASC
		 LIMIT ?`,
		now,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query character review queue: %w", err)
	}
	defer rows.Close()
	out := make([]CharacterReviewCard, 0)
	for rows.Next() {
		var card CharacterReviewCard
		if err := rows.Scan(&card.CharacterID, &card.Character, &card.Pinyin, &card.English); err != nil {
			return nil, fmt.Errorf("scan character review card: %w", err)
		}
		exRows, err := s.db.Query(
			`SELECT csl.segment, csl.segment_pinyin, csl.segment_translation
			 FROM character_segment_links csl
			 WHERE csl.character_id = ?
			 ORDER BY csl.created_at DESC
			 LIMIT 5`,
			card.CharacterID,
		)
		if err == nil {
			for exRows.Next() {
				var ex CharacterExampleSegment
				_ = exRows.Scan(&ex.Segment, &ex.SegmentPinyin, &ex.SegmentTranslation)
				card.ExampleSegments = append(card.ExampleSegments, ex)
			}
			_ = exRows.Close()
		}
		out = append(out, card)
	}
	return out, nil
}

func (s *SRSStore) GetCharacterDueCount() int {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var cnt int
	_ = s.db.QueryRow(
		`SELECT COUNT(*) FROM saved_characters sc
		 JOIN srs_state st ON sc.id = st.character_id
		 WHERE sc.status = 'learning' AND (st.due_at IS NULL OR st.due_at <= ?)`,
		now,
	).Scan(&cnt)
	return cnt
}

func (s *SRSStore) ensureSegmentSRSState(segmentID string, now string) error {
	id := "seg-" + segmentID
	if _, err := s.db.Exec(
		`INSERT OR IGNORE INTO srs_state (id, segment_id, due_at, interval_days, ease, reps, lapses, last_reviewed_at)
		 VALUES (?, ?, ?, 0, 2.5, 0, 0, ?)`,
		id, segmentID, now, now,
	); err != nil {
		return fmt.Errorf("init segment srs state: %w", err)
	}
	return nil
}

func (s *SRSStore) ensureCharacterSRSState(characterID string, now string) error {
	id := "char-" + characterID
	if _, err := s.db.Exec(
		`INSERT OR IGNORE INTO srs_state (id, character_id, due_at, interval_days, ease, reps, lapses, last_reviewed_at)
		 VALUES (?, ?, ?, 0, 2.5, 0, 0, ?)`,
		id, characterID, now, now,
	); err != nil {
		return fmt.Errorf("init character srs state: %w", err)
	}
	return nil
}

func isValidStatus(status string) bool {
	return status == "unknown" || status == "learning" || status == "known"
}

func isCJKIdeograph(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x20000 && r <= 0x2A6DF) ||
		(r >= 0x2A700 && r <= 0x2CEAF) ||
		(r >= 0x2CEB0 && r <= 0x2EBEF) ||
		(r >= 0x30000 && r <= 0x323AF)
}

func maxFloat(a float64, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
