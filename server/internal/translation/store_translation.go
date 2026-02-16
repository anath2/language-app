package translation

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

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
		Paragraphs: nil,
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
		    full_translation, error_message, metadata_json, text_id, progress, total
		 )
		 VALUES (?, ?, ?, ?, 'translation', ?, ?, NULL, NULL, '{}', NULL, 0, 0)`,
		tr.ID,
		tr.CreatedAt,
		tr.CreatedAt,
		tr.Status,
		tr.SourceType,
		tr.InputText,
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

func (s *TranslationStore) SetProcessing(id string, total int, sentenceCount int) error {
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

	if sentenceCount < 0 {
		sentenceCount = 0
	}
	for sentenceIdx := 0; sentenceIdx < sentenceCount; sentenceIdx++ {
		if _, err := tx.Exec(
			`INSERT INTO translation_paragraphs (id, translation_id, paragraph_idx, indent, separator)
			 VALUES (?, ?, ?, '', '')
			 ON CONFLICT (translation_id, paragraph_idx) DO NOTHING`,
			fmt.Sprintf("%s:%d", id, sentenceIdx),
			id,
			sentenceIdx,
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
		`INSERT INTO translation_paragraphs (id, translation_id, paragraph_idx, indent, separator)
		 VALUES (?, ?, ?, '', '')
		 ON CONFLICT (translation_id, paragraph_idx) DO NOTHING`,
		fmt.Sprintf("%s:%d", id, sentenceIndex),
		id,
		sentenceIndex,
	); err != nil {
		return 0, 0, fmt.Errorf("ensure sentence row: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO translation_segments (id, translation_id, paragraph_idx, seg_idx, segment_text, pinyin, english, created_at)
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
	rows, err := s.db.Query(
		`SELECT english FROM translation_segments
		 WHERE translation_id = ?
		 ORDER BY paragraph_idx ASC, seg_idx ASC`,
		id,
	)
	if err != nil {
		return fmt.Errorf("query english segments: %w", err)
	}
	defer rows.Close()

	parts := make([]string, 0)
	for rows.Next() {
		var english string
		if err := rows.Scan(&english); err != nil {
			return fmt.Errorf("scan english segment: %w", err)
		}
		if english != "" {
			parts = append(parts, english)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate english segments: %w", err)
	}

	full := strings.Join(parts, " ")
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin complete tx: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`UPDATE translations
		 SET status = 'completed',
		     progress = total,
		     full_translation = CASE WHEN full_translation IS NULL OR full_translation = '' THEN ? ELSE full_translation END,
		     error_message = NULL
		 WHERE id = ?`,
		full,
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
		`SELECT segment_text, pinyin, english, seg_idx, paragraph_idx
		 FROM translation_segments
		 WHERE translation_id = ?
		 ORDER BY paragraph_idx ASC, seg_idx ASC`,
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
		`SELECT id, created_at, status, source_type, input_text, full_translation, error_message, progress, total
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

	tr.Paragraphs = s.loadParagraphs(id)
	return tr, nil
}

func (s *TranslationStore) listOnce(limit int, offset int, status string) ([]Translation, int, error) {
	countQuery := `SELECT COUNT(*) FROM translations`
	listQuery := `SELECT id, created_at, status, source_type, input_text, full_translation, error_message, progress, total
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

func (s *TranslationStore) UpdateTranslationSegments(translationID string, paragraphIdx int, segments []SegmentResult) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO translation_paragraphs (id, translation_id, paragraph_idx, indent, separator)
		 VALUES (?, ?, ?, '', '')
		 ON CONFLICT (translation_id, paragraph_idx) DO NOTHING`,
		fmt.Sprintf("%s:%d", translationID, paragraphIdx),
		translationID, paragraphIdx,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM translation_segments WHERE translation_id = ? AND paragraph_idx = ?`, translationID, paragraphIdx); err != nil {
		return err
	}
	for idx, seg := range segments {
		if _, err := tx.Exec(
			`INSERT INTO translation_segments (id, translation_id, paragraph_idx, seg_idx, segment_text, pinyin, english, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			fmt.Sprintf("%s:%d:%d", translationID, paragraphIdx, idx),
			translationID, paragraphIdx, idx, seg.Segment, seg.Pinyin, seg.English, time.Now().UTC().Format(time.RFC3339Nano),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *TranslationStore) loadParagraphs(translationID string) []ParagraphResult {
	rows, err := s.db.Query(
		`SELECT paragraph_idx, indent, separator
		 FROM translation_paragraphs
		 WHERE translation_id = ?
		 ORDER BY paragraph_idx ASC`,
		translationID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	paragraphs := make([]ParagraphResult, 0)
	indices := make([]int, 0)
	for rows.Next() {
		var idx int
		var indent string
		var separator string
		if err := rows.Scan(&idx, &indent, &separator); err != nil {
			return nil
		}
		paragraphs = append(paragraphs, ParagraphResult{
			Translations: []SegmentResult{},
			Indent:       indent,
			Separator:    separator,
		})
		indices = append(indices, idx)
	}
	if err := rows.Err(); err != nil {
		return nil
	}

	for i, paraIdx := range indices {
		segRows, err := s.db.Query(
			`SELECT segment_text, pinyin, english
			 FROM translation_segments
			 WHERE translation_id = ? AND paragraph_idx = ?
			 ORDER BY seg_idx ASC`,
			translationID,
			paraIdx,
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
		paragraphs[i].Translations = segments
	}

	return paragraphs
}
