package translation

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

func computeTitle(inputText string) string {
	runes := []rune(strings.TrimSpace(inputText))
	if len(runes) <= 10 {
		return string(runes)
	}
	return string(runes[:10]) + "…"
}

func (s *TranslationStore) Create(inputText string, sourceType string) (Translation, error) {
	if strings.TrimSpace(inputText) == "" {
		return Translation{}, errors.New("input_text is required")
	}
	if sourceType == "" {
		sourceType = "text"
	}

	id, err := newID()
	if err != nil {
		return Translation{}, err
	}

	tr := Translation{
		ID:         id,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		Status:     "pending",
		SourceType: sourceType,
		InputText:  inputText,
		Title:      computeTitle(inputText),
		Sentences:  nil,
		Progress:   0,
		Total:      0,
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Translation{}, fmt.Errorf("begin create translation tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO translations (
		    id, created_at, updated_at, status, translation_type, source_type, input_text,
		    full_translation, error_message, metadata_json, progress, total, title
		 )
		 VALUES (?, ?, ?, ?, 'translation', ?, ?, NULL, NULL, '{}', 0, 0, ?)`,
		tr.ID,
		tr.CreatedAt,
		tr.CreatedAt,
		tr.Status,
		tr.SourceType,
		tr.InputText,
		tr.Title,
	); err != nil {
		return Translation{}, fmt.Errorf("insert translation: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO translation_jobs (translation_id, state, attempts, lease_until, last_error, created_at, updated_at)
		 VALUES (?, 'pending', 0, NULL, NULL, ?, ?)`,
		tr.ID,
		tr.CreatedAt,
		tr.CreatedAt,
	); err != nil {
		return Translation{}, fmt.Errorf("insert translation job: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Translation{}, fmt.Errorf("commit create translation tx: %w", err)
	}

	return tr, nil
}

func (s *TranslationStore) Get(id string) (Translation, bool) {
	for i := 0; i < 8; i++ {
		tr, err := s.getOnce(id)
		if err == nil {
			return tr, true
		}
		if errors.Is(err, sql.ErrNoRows) {
			return Translation{}, false
		}
		if isDBLocked(err) {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		return Translation{}, false
	}
	return Translation{}, false
}

func (s *TranslationStore) Delete(id string) bool {
	for i := 0; i < 8; i++ {
		res, err := s.db.Exec(`DELETE FROM translations WHERE id = ?`, id)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "database is locked") {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			return false
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return false
		}
		return affected > 0
	}
	return false
}

func (s *TranslationStore) List(limit int, offset int, status string) ([]Translation, int, error) {
	if status != "" && status != "pending" && status != "processing" && status != "completed" && status != "failed" {
		return nil, 0, errors.New("Invalid status filter")
	}
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	for i := 0; i < 40; i++ {
		items, total, err := s.listOnce(limit, offset, status)
		if err == nil {
			return items, total, nil
		}
		if isDBLocked(err) {
			time.Sleep(25 * time.Millisecond)
			continue
		}
		return nil, 0, err
	}

	return nil, 0, fmt.Errorf("list translations: database remained locked")
}

func (s *TranslationStore) SetProcessing(id string, total int, sentences []SentenceInit) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin set processing tx: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.Exec(`UPDATE translations SET status = 'processing', total = ?, progress = 0 WHERE id = ?`, total, id)
	if err != nil {
		return fmt.Errorf("update processing status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil || affected == 0 {
		return ErrNotFound
	}

	for sentenceIdx, sent := range sentences {
		if _, err := tx.Exec(
			`INSERT INTO translation_sentences (id, translation_id, sentence_idx, indent, separator, content_hash)
			 VALUES (?, ?, ?, ?, ?, '')
			 ON CONFLICT (translation_id, sentence_idx) DO NOTHING`,
			fmt.Sprintf("%s:%d", id, sentenceIdx),
			id,
			sentenceIdx,
			sent.Indent,
			sent.Separator,
		); err != nil {
			return fmt.Errorf("ensure sentence row %d: %w", sentenceIdx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit set processing tx: %w", err)
	}
	return nil
}

func (s *TranslationStore) AddProgressSegment(id string, result SegmentResult, sentenceIndex int) (int, int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("begin add progress tx: %w", err)
	}
	defer tx.Rollback()

	var progress int
	var total int
	row := tx.QueryRow(`SELECT progress, total FROM translations WHERE id = ?`, id)
	if err := row.Scan(&progress, &total); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, ErrNotFound
		}
		return 0, 0, fmt.Errorf("load progress state: %w", err)
	}

	segIdx := progress
	if _, err := tx.Exec(
		`INSERT INTO translation_sentences (id, translation_id, sentence_idx, indent, separator)
		 VALUES (?, ?, ?, '', '')
		 ON CONFLICT (translation_id, sentence_idx) DO NOTHING`,
		fmt.Sprintf("%s:%d", id, sentenceIndex),
		id,
		sentenceIndex,
	); err != nil {
		return 0, 0, fmt.Errorf("ensure sentence row: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO translation_segments (id, translation_id, sentence_idx, seg_idx, segment_text, pinyin, english, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		fmt.Sprintf("%s:%d:%d", id, sentenceIndex, segIdx),
		id,
		sentenceIndex,
		segIdx,
		result.Segment,
		result.Pinyin,
		result.English,
		time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return 0, 0, fmt.Errorf("insert translation segment: %w", err)
	}

	progress++
	if total == 0 {
		total = progress
	}
	if _, err := tx.Exec(`UPDATE translations SET progress = ?, total = ? WHERE id = ?`, progress, total, id); err != nil {
		return 0, 0, fmt.Errorf("update translation progress: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit add progress tx: %w", err)
	}

	return progress, total, nil
}

func (s *TranslationStore) SetFullTranslation(id string, fullTranslation string) error {
	if fullTranslation == "" {
		return fmt.Errorf("full_translation must not be empty")
	}
	res, err := s.db.Exec(
		`UPDATE translations SET full_translation = ? WHERE id = ?`,
		fullTranslation,
		id,
	)
	if err != nil {
		return fmt.Errorf("set full translation: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil || affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *TranslationStore) Complete(id string) error {
	var fullTranslation sql.NullString
	if err := s.db.QueryRow(`SELECT full_translation FROM translations WHERE id = ?`, id).Scan(&fullTranslation); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("check full translation: %w", err)
	}
	if !fullTranslation.Valid || fullTranslation.String == "" {
		return fmt.Errorf("complete translation: full_translation is not set")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin complete tx: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`UPDATE translations
		 SET status = 'completed',
		     progress = total,
		     error_message = NULL
		 WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("complete translation: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil || affected == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(
		`UPDATE translation_jobs
		 SET state = 'done', lease_until = NULL, last_error = NULL, updated_at = ?
		 WHERE translation_id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano),
		id,
	); err != nil {
		return fmt.Errorf("mark translation job done: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit complete tx: %w", err)
	}
	return nil
}

func (s *TranslationStore) Fail(id string, message string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin fail tx: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`UPDATE translations SET status = 'failed', error_message = ? WHERE id = ?`,
		message,
		id,
	)
	if err != nil {
		return fmt.Errorf("fail translation: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil || affected == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(
		`UPDATE translation_jobs
		 SET state = 'failed', lease_until = NULL, last_error = ?, updated_at = ?
		 WHERE translation_id = ?`,
		message,
		time.Now().UTC().Format(time.RFC3339Nano),
		id,
	); err != nil {
		return fmt.Errorf("mark translation job failed: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit fail tx: %w", err)
	}
	return nil
}

func (s *TranslationStore) GetProgressSnapshot(id string) (ProgressSnapshot, bool) {
	row := s.db.QueryRow(`SELECT status, progress, total, COALESCE(error_message, '') FROM translations WHERE id = ?`, id)
	var snapshot ProgressSnapshot
	if err := row.Scan(&snapshot.Status, &snapshot.Current, &snapshot.Total, &snapshot.Error); err != nil {
		return ProgressSnapshot{}, false
	}

	rows, err := s.db.Query(
		`SELECT segment_text, pinyin, english, seg_idx, sentence_idx
		 FROM translation_segments
		 WHERE translation_id = ?
		 ORDER BY sentence_idx ASC, seg_idx ASC`,
		id,
	)
	if err != nil {
		return ProgressSnapshot{}, false
	}
	defer rows.Close()

	snapshot.Results = make([]SegmentProgressEntry, 0)
	for rows.Next() {
		var seg SegmentProgressEntry
		if err := rows.Scan(&seg.Segment, &seg.Pinyin, &seg.English, &seg.Index, &seg.SentenceIndex); err != nil {
			return ProgressSnapshot{}, false
		}
		snapshot.Results = append(snapshot.Results, seg)
	}
	if err := rows.Err(); err != nil {
		return ProgressSnapshot{}, false
	}

	return snapshot, true
}

func (s *TranslationStore) getOnce(id string) (Translation, error) {
	row := s.db.QueryRow(
		`SELECT id, created_at, status, source_type, input_text, title, full_translation, error_message, progress, total
		 FROM translations WHERE id = ?`,
		id,
	)

	var tr Translation
	var fullTranslation sql.NullString
	var errorMessage sql.NullString
	if err := row.Scan(
		&tr.ID,
		&tr.CreatedAt,
		&tr.Status,
		&tr.SourceType,
		&tr.InputText,
		&tr.Title,
		&fullTranslation,
		&errorMessage,
		&tr.Progress,
		&tr.Total,
	); err != nil {
		return Translation{}, err
	}
	if fullTranslation.Valid {
		v := fullTranslation.String
		tr.FullTranslation = &v
	}
	if errorMessage.Valid {
		v := errorMessage.String
		tr.ErrorMessage = &v
	}

	tr.Sentences = s.loadSentences(id)
	return tr, nil
}

func (s *TranslationStore) listOnce(limit int, offset int, status string) ([]Translation, int, error) {
	countQuery := `SELECT COUNT(*) FROM translations`
	listQuery := `SELECT id, created_at, status, source_type, input_text, title, full_translation, error_message, progress, total
		FROM translations`
	args := make([]any, 0, 3)
	if status != "" {
		countQuery += ` WHERE status = ?`
		listQuery += ` WHERE status = ?`
		args = append(args, status)
	}
	listQuery += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`

	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count translations: %w", err)
	}

	listArgs := append(args, limit, offset)
	rows, err := s.db.Query(listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list translations: %w", err)
	}
	defer rows.Close()

	items := make([]Translation, 0, limit)
	for rows.Next() {
		var tr Translation
		var fullTranslation sql.NullString
		var errorMessage sql.NullString
		if err := rows.Scan(
			&tr.ID,
			&tr.CreatedAt,
			&tr.Status,
			&tr.SourceType,
			&tr.InputText,
			&tr.Title,
			&fullTranslation,
			&errorMessage,
			&tr.Progress,
			&tr.Total,
		); err != nil {
			return nil, 0, fmt.Errorf("scan translation row: %w", err)
		}
		if fullTranslation.Valid {
			v := fullTranslation.String
			tr.FullTranslation = &v
		}
		if errorMessage.Valid {
			v := errorMessage.String
			tr.ErrorMessage = &v
		}
		items = append(items, tr)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate translation rows: %w", err)
	}

	return items, total, nil
}

func (s *TranslationStore) UpdateTranslationSegments(translationID string, sentenceIdx int, segments []SegmentResult) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO translation_sentences (id, translation_id, sentence_idx, indent, separator)
		 VALUES (?, ?, ?, '', '')
		 ON CONFLICT (translation_id, sentence_idx) DO NOTHING`,
		fmt.Sprintf("%s:%d", translationID, sentenceIdx),
		translationID, sentenceIdx,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM translation_segments WHERE translation_id = ? AND sentence_idx = ?`, translationID, sentenceIdx); err != nil {
		return err
	}
	for idx, seg := range segments {
		if _, err := tx.Exec(
			`INSERT INTO translation_segments (id, translation_id, sentence_idx, seg_idx, segment_text, pinyin, english, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			fmt.Sprintf("%s:%d:%d", translationID, sentenceIdx, idx),
			translationID, sentenceIdx, idx, seg.Segment, seg.Pinyin, seg.English, time.Now().UTC().Format(time.RFC3339Nano),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UpdateInputTextForReprocessing diffs the new text against existing sentence hashes,
// deletes stale segments, updates the translation's input_text + status, and returns
// the map of sentenceIdx → sentence for only changed/new sentences.
// Returns an empty map (no error) when the new text produces no changes.
func (s *TranslationStore) UpdateInputTextForReprocessing(id string, newText string) (map[int]string, error) {
	sentences := splitStoreSentences(newText)

	// Compute hashes for the new sentences.
	newHashes := make([]string, len(sentences))
	for i, si := range sentences {
		h := sha256.Sum256([]byte(si.Text))
		newHashes[i] = fmt.Sprintf("%x", h)
	}

	// Check translation exists.
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM translations WHERE id = ?`, id).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check translation exists: %w", err)
	}
	if exists == 0 {
		return nil, ErrNotFound
	}

	// Load existing sentence hashes (outside transaction — read-only).
	rows, err := s.db.Query(
		`SELECT sentence_idx, content_hash FROM translation_sentences WHERE translation_id = ? ORDER BY sentence_idx ASC`,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("load sentence hashes: %w", err)
	}
	oldHashes := make(map[int]string)
	for rows.Next() {
		var idx int
		var hash string
		if err := rows.Scan(&idx, &hash); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan sentence hash: %w", err)
		}
		oldHashes[idx] = hash
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sentence hashes: %w", err)
	}

	// Determine changed/new sentences.
	changed := make(map[int]string)
	for i, si := range sentences {
		if oldHash, ok := oldHashes[i]; ok && oldHash == newHashes[i] {
			continue
		}
		changed[i] = si.Text
	}
	// Determine removed sentences (old indices beyond new count).
	removedIdxs := make([]int, 0)
	for oldIdx := range oldHashes {
		if oldIdx >= len(sentences) {
			removedIdxs = append(removedIdxs, oldIdx)
		}
	}

	// If nothing changed and nothing removed, return early without touching DB.
	if len(changed) == 0 && len(removedIdxs) == 0 {
		return changed, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin update input text tx: %w", err)
	}
	defer tx.Rollback()

	// Delete stale segments and upsert hashes for changed/new sentences.
	for i, si := range sentences {
		if _, isChanged := changed[i]; !isChanged {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM translation_segments WHERE translation_id = ? AND sentence_idx = ?`, id, i); err != nil {
			return nil, fmt.Errorf("delete stale segments for sentence %d: %w", i, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO translation_sentences (id, translation_id, sentence_idx, indent, separator, content_hash)
			 VALUES (?, ?, ?, ?, ?, ?)
			 ON CONFLICT (translation_id, sentence_idx) DO UPDATE SET content_hash = excluded.content_hash, indent = excluded.indent, separator = excluded.separator`,
			fmt.Sprintf("%s:%d", id, i),
			id,
			i,
			si.Indent,
			si.Separator,
			newHashes[i],
		); err != nil {
			return nil, fmt.Errorf("upsert sentence %d: %w", i, err)
		}
	}

	// Remove sentences beyond new sentence count.
	for _, oldIdx := range removedIdxs {
		if _, err := tx.Exec(`DELETE FROM translation_segments WHERE translation_id = ? AND sentence_idx = ?`, id, oldIdx); err != nil {
			return nil, fmt.Errorf("delete segments for removed sentence %d: %w", oldIdx, err)
		}
		if _, err := tx.Exec(`DELETE FROM translation_sentences WHERE translation_id = ? AND sentence_idx = ?`, id, oldIdx); err != nil {
			return nil, fmt.Errorf("delete removed sentence %d: %w", oldIdx, err)
		}
	}

	if _, err := tx.Exec(
		`UPDATE translations SET input_text = ?, status = 'pending', progress = 0, total = 0 WHERE id = ?`,
		newText,
		id,
	); err != nil {
		return nil, fmt.Errorf("update input text: %w", err)
	}

	// Reset translation_jobs row to pending so the queue can claim it again.
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.Exec(
		`UPDATE translation_jobs SET state = 'pending', lease_until = NULL, last_error = NULL, updated_at = ? WHERE translation_id = ?`,
		now,
		id,
	); err != nil {
		return nil, fmt.Errorf("reset translation job: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit update input text tx: %w", err)
	}

	return changed, nil
}

// SetReprocessing marks the translation as processing with the given total segment count.
// Unlike SetProcessing it does not touch sentence rows (they are already set up by UpdateInputTextForReprocessing).
func (s *TranslationStore) SetReprocessing(id string, total int) error {
	res, err := s.db.Exec(
		`UPDATE translations SET status = 'processing', total = ?, progress = 0 WHERE id = ?`,
		total,
		id,
	)
	if err != nil {
		return fmt.Errorf("set reprocessing: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil || affected == 0 {
		return ErrNotFound
	}
	return nil
}

// AddReprocessedSegment inserts a segment at an explicit (sentenceIdx, segIdx) position and
// increments the global progress counter.
func (s *TranslationStore) AddReprocessedSegment(id string, result SegmentResult, sentenceIdx int, segIdx int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin add reprocessed segment tx: %w", err)
	}
	defer tx.Rollback()

	var progress, total int
	if err := tx.QueryRow(`SELECT progress, total FROM translations WHERE id = ?`, id).Scan(&progress, &total); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("load progress: %w", err)
	}

	if _, err := tx.Exec(
		`INSERT INTO translation_segments (id, translation_id, sentence_idx, seg_idx, segment_text, pinyin, english, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		fmt.Sprintf("%s:%d:%d", id, sentenceIdx, segIdx),
		id,
		sentenceIdx,
		segIdx,
		result.Segment,
		result.Pinyin,
		result.English,
		time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("insert reprocessed segment: %w", err)
	}

	progress++
	if _, err := tx.Exec(`UPDATE translations SET progress = ? WHERE id = ?`, progress, id); err != nil {
		return fmt.Errorf("update progress: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit add reprocessed segment tx: %w", err)
	}
	return nil
}

func (s *TranslationStore) UpdateTitle(id string, title string) error {
	res, err := s.db.Exec(`UPDATE translations SET title = ? WHERE id = ?`, title, id)
	if err != nil {
		return fmt.Errorf("update title: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil || affected == 0 {
		return ErrNotFound
	}
	return nil
}

type storeSentenceInfo struct {
	Text      string
	Indent    string
	Separator string
}

// splitStoreSentences is a copy of the queue package's splitInputSentences logic,
// kept here so the store package stays decoupled from queue.
// It now also captures indent and separator for each sentence.
func splitStoreSentences(text string) []storeSentenceInfo {
	var out []storeSentenceInfo
	var sentence strings.Builder
	var lineIndent strings.Builder
	atLineStart := true

	addSeparatorChar := func(r rune) {
		if len(out) > 0 {
			out[len(out)-1].Separator += string(r)
		}
	}

	for len(text) > 0 {
		r, size := utf8.DecodeRuneInString(text)
		text = text[size:]

		if atLineStart {
			if r == ' ' || r == '\t' {
				lineIndent.WriteRune(r)
				continue
			}
			if r == '\n' || r == '\r' {
				addSeparatorChar(r)
				lineIndent.Reset()
				continue
			}
			atLineStart = false
		}

		if r == '\n' || r == '\r' {
			s := strings.TrimSpace(sentence.String())
			if s != "" {
				out = append(out, storeSentenceInfo{
					Text:   s,
					Indent: lineIndent.String(),
				})
			}
			addSeparatorChar(r)
			sentence.Reset()
			lineIndent.Reset()
			atLineStart = true
			continue
		}

		sentence.WriteRune(r)
		if isStoreSentenceDelimiter(r) {
			s := strings.TrimSpace(sentence.String())
			if s != "" {
				out = append(out, storeSentenceInfo{
					Text:   s,
					Indent: lineIndent.String(),
				})
				sentence.Reset()
				lineIndent.Reset()
			}
		}
	}

	if s := strings.TrimSpace(sentence.String()); s != "" {
		out = append(out, storeSentenceInfo{
			Text:   s,
			Indent: lineIndent.String(),
		})
	}

	return out
}

func isStoreSentenceDelimiter(r rune) bool {
	switch r {
	case '。', '！', '？', '!', '?', ';', '；':
		return true
	default:
		return false
	}
}

func (s *TranslationStore) loadSentences(translationID string) []SentenceResult {
	rows, err := s.db.Query(
		`SELECT sentence_idx, indent, separator
		 FROM translation_sentences
		 WHERE translation_id = ?
		 ORDER BY sentence_idx ASC`,
		translationID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	sentences := make([]SentenceResult, 0)
	indices := make([]int, 0)
	for rows.Next() {
		var idx int
		var indent string
		var separator string
		if err := rows.Scan(&idx, &indent, &separator); err != nil {
			return nil
		}
		sentences = append(sentences, SentenceResult{
			Translations: []SegmentResult{},
			Indent:       indent,
			Separator:    separator,
		})
		indices = append(indices, idx)
	}
	if err := rows.Err(); err != nil {
		return nil
	}

	for i, sentenceIdx := range indices {
		segRows, err := s.db.Query(
			`SELECT segment_text, pinyin, english
			 FROM translation_segments
			 WHERE translation_id = ? AND sentence_idx = ?
			 ORDER BY seg_idx ASC`,
			translationID,
			sentenceIdx,
		)
		if err != nil {
			return nil
		}
		segments := make([]SegmentResult, 0)
		for segRows.Next() {
			var seg SegmentResult
			if err := segRows.Scan(&seg.Segment, &seg.Pinyin, &seg.English); err != nil {
				_ = segRows.Close()
				return nil
			}
			segments = append(segments, seg)
		}
		_ = segRows.Close()
		sentences[i].Translations = segments
	}

	return sentences
}
