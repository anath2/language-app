package translation

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("translation not found")

type Translation struct {
	ID              string
	CreatedAt       string
	Status          string
	SourceType      string
	InputText       string
	FullTranslation *string
	ErrorMessage    *string
	Paragraphs      []ParagraphResult
	Progress        int
	Total           int
}

type SegmentResult struct {
	Segment string `json:"segment"`
	Pinyin  string `json:"pinyin"`
	English string `json:"english"`
}

type ParagraphResult struct {
	Translations []SegmentResult `json:"translations"`
	Indent       string          `json:"indent"`
	Separator    string          `json:"separator"`
}

type SegmentProgressEntry struct {
	Segment        string
	Pinyin         string
	English        string
	Index          int
	ParagraphIndex int
}

type ProgressSnapshot struct {
	Status  string
	Current int
	Total   int
	Results []SegmentProgressEntry
	Error   string
}

type TextRecord struct {
	ID             string
	CreatedAt      string
	SourceType     string
	RawText        string
	NormalizedText string
	Metadata       map[string]any
}

type VocabRecord struct {
	ID       string
	Headword string
	Pinyin   string
	English  string
	Status   string
}

type VocabSRSInfo struct {
	VocabItemID  string
	Headword     string
	Pinyin       string
	English      string
	Opacity      float64
	IsStruggling bool
	Status       string
}

type ReviewCard struct {
	VocabItemID string
	Headword    string
	Pinyin      string
	English     string
	Snippets    []string
}

type ReviewAnswerResult struct {
	VocabItemID  string
	NextDueAt    *string
	IntervalDays float64
	RemainingDue int
}

type UserProfile struct {
	Name      string
	Email     string
	Language  string
	CreatedAt string
	UpdatedAt string
}

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("translation db path is required")
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if _, err := db.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set wal mode: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout = 3000;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	s := &Store{db: db}
	if err := s.verifySchema(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) verifySchema() error {
	requiredTables := []string{
		"translations",
		"translation_paragraphs",
		"translation_segments",
		"translation_jobs",
		"texts",
		"segments",
		"events",
		"vocab_items",
		"vocab_occurrences",
		"srs_state",
		"vocab_lookups",
		"user_profile",
	}
	for _, table := range requiredTables {
		var exists int
		if err := s.db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
			table,
		).Scan(&exists); err != nil {
			return fmt.Errorf("verify schema table %s: %w", table, err)
		}
		if exists == 0 {
			return fmt.Errorf("database schema is not migrated: missing table %s", table)
		}
	}
	return nil
}

func (s *Store) Create(inputText string, sourceType string) (Translation, error) {
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

func (s *Store) Get(id string) (Translation, bool) {
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

func (s *Store) Delete(id string) bool {
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

func (s *Store) List(limit int, offset int, status string) ([]Translation, int, error) {
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

func (s *Store) SetProcessing(id string, total int) error {
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

	if _, err := tx.Exec(
		`INSERT INTO translation_paragraphs (id, translation_id, paragraph_idx, indent, separator)
		 VALUES (?, ?, 0, '', '')
		 ON CONFLICT (translation_id, paragraph_idx) DO NOTHING`,
		fmt.Sprintf("%s:%d", id, 0),
		id,
	); err != nil {
		return fmt.Errorf("ensure default paragraph: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit set processing tx: %w", err)
	}
	return nil
}

func (s *Store) AddProgressSegment(id string, result SegmentResult) (int, int, error) {
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
		`INSERT INTO translation_segments (id, translation_id, paragraph_idx, seg_idx, segment_text, pinyin, english, created_at)
		 VALUES (?, ?, 0, ?, ?, ?, ?, ?)`,
		fmt.Sprintf("%s:%d:%d", id, 0, segIdx),
		id,
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

func (s *Store) Complete(id string) error {
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
		`UPDATE translations SET status = 'completed', progress = total, full_translation = ?, error_message = NULL WHERE id = ?`,
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

func (s *Store) Fail(id string, message string) error {
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

func (s *Store) GetProgressSnapshot(id string) (ProgressSnapshot, bool) {
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
		if err := rows.Scan(&seg.Segment, &seg.Pinyin, &seg.English, &seg.Index, &seg.ParagraphIndex); err != nil {
			return ProgressSnapshot{}, false
		}
		snapshot.Results = append(snapshot.Results, seg)
	}
	if err := rows.Err(); err != nil {
		return ProgressSnapshot{}, false
	}

	return snapshot, true
}

func (s *Store) ListRestartableTranslationIDs() ([]string, error) {
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

func (s *Store) ClaimTranslationJob(translationID string, leaseDuration time.Duration) (bool, error) {
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

func newID() (string, error) {
	return fmt.Sprintf("%d", time.Now().UTC().UnixNano()), nil
}

func (s *Store) getOnce(id string) (Translation, error) {
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

func (s *Store) listOnce(limit int, offset int, status string) ([]Translation, int, error) {
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

func isDBLocked(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "database is locked")
}

func (s *Store) CreateText(rawText string, sourceType string, metadata map[string]any) (TextRecord, error) {
	if strings.TrimSpace(rawText) == "" {
		return TextRecord{}, errors.New("raw_text is required")
	}
	if sourceType == "" {
		sourceType = "text"
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metaBytes, err := json.Marshal(metadata)
	if err != nil {
		return TextRecord{}, fmt.Errorf("encode metadata: %w", err)
	}
	id, _ := newID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	normalized := strings.TrimSpace(rawText)
	if _, err := s.db.Exec(
		`INSERT INTO texts (id, created_at, source_type, raw_text, normalized_text, metadata_json)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, now, sourceType, rawText, normalized, string(metaBytes),
	); err != nil {
		return TextRecord{}, fmt.Errorf("insert text: %w", err)
	}
	return TextRecord{
		ID:             id,
		CreatedAt:      now,
		SourceType:     sourceType,
		RawText:        rawText,
		NormalizedText: normalized,
		Metadata:       metadata,
	}, nil
}

func (s *Store) GetText(textID string) (TextRecord, bool) {
	row := s.db.QueryRow(`SELECT id, created_at, source_type, raw_text, normalized_text, metadata_json FROM texts WHERE id = ?`, textID)
	var rec TextRecord
	var metaJSON string
	if err := row.Scan(&rec.ID, &rec.CreatedAt, &rec.SourceType, &rec.RawText, &rec.NormalizedText, &metaJSON); err != nil {
		return TextRecord{}, false
	}
	rec.Metadata = map[string]any{}
	_ = json.Unmarshal([]byte(metaJSON), &rec.Metadata)
	return rec, true
}

func (s *Store) CreateEvent(eventType string, textID *string, segmentID *string, payload map[string]any) (string, error) {
	if strings.TrimSpace(eventType) == "" {
		return "", errors.New("event_type is required")
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode payload: %w", err)
	}
	id, _ := newID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var textIDVal any
	var segmentIDVal any
	if textID != nil {
		textIDVal = *textID
	}
	if segmentID != nil {
		segmentIDVal = *segmentID
	}
	if _, err := s.db.Exec(
		`INSERT INTO events (id, ts, text_id, segment_id, event_type, payload_json)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, now, textIDVal, segmentIDVal, eventType, string(payloadBytes),
	); err != nil {
		return "", fmt.Errorf("insert event: %w", err)
	}
	return id, nil
}

func (s *Store) SaveVocabItem(headword string, pinyin string, english string, textID *string, segmentID *string, snippet *string, status string) (string, error) {
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

func (s *Store) UpdateVocabStatus(vocabItemID string, status string) error {
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

func (s *Store) RecordLookup(vocabItemID string) (VocabSRSInfo, bool) {
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

func (s *Store) GetVocabSRSInfo(headwords []string) ([]VocabSRSInfo, error) {
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

func (s *Store) GetReviewQueue(limit int) ([]ReviewCard, error) {
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

func (s *Store) GetDueCount() int {
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

func (s *Store) RecordReviewAnswer(vocabItemID string, grade int) (ReviewAnswerResult, bool, error) {
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

func (s *Store) UpsertUserProfile(name string, email string, language string) (UserProfile, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`UPDATE user_profile SET name = ?, email = ?, language = ?, updated_at = ? WHERE id = 1`,
		name, email, language, now)
	if err != nil {
		return UserProfile{}, fmt.Errorf("update user profile: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		if _, err := s.db.Exec(`INSERT INTO user_profile (id, name, email, language, created_at, updated_at) VALUES (1, ?, ?, ?, ?, ?)`,
			name, email, language, now, now); err != nil {
			return UserProfile{}, fmt.Errorf("insert user profile: %w", err)
		}
	}
	return UserProfile{Name: name, Email: email, Language: language, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *Store) GetUserProfile() (UserProfile, bool) {
	row := s.db.QueryRow(`SELECT name, email, language, created_at, updated_at FROM user_profile WHERE id = 1`)
	var p UserProfile
	if err := row.Scan(&p.Name, &p.Email, &p.Language, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return UserProfile{}, false
	}
	return p, true
}

func (s *Store) CountVocabByStatus(status string) int {
	var cnt int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM vocab_items WHERE status = ?`, status).Scan(&cnt)
	return cnt
}

func (s *Store) CountTotalVocab() int {
	var cnt int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM vocab_items`).Scan(&cnt)
	return cnt
}

func (s *Store) ExportProgressJSON() (string, error) {
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

func (s *Store) ImportProgressJSON(input string) (map[string]int, error) {
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

func (s *Store) UpdateTranslationSegments(translationID string, paragraphIdx int, segments []SegmentResult) error {
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

func (s *Store) loadParagraphs(translationID string) []ParagraphResult {
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

func rowsToMaps(rows *sql.Rows) ([]map[string]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		obj := make(map[string]any, len(columns))
		for i, col := range columns {
			switch v := values[i].(type) {
			case []byte:
				obj[col] = string(v)
			default:
				obj[col] = v
			}
		}
		out = append(out, obj)
	}
	return out, rows.Err()
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", x)
	}
}

func nullableString(v any) any {
	s := toString(v)
	if s == "" {
		return nil
	}
	return s
}

func toInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	default:
		return 0
	}
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		n, _ := strconv.ParseFloat(x, 64)
		return n
	default:
		return 0
	}
}

func maxFloat(a float64, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
