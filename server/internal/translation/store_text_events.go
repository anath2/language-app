package translation

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *TextEventStore) CreateText(rawText string, sourceType string, metadata map[string]any) (TextRecord, error) {
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

func (s *TextEventStore) GetText(textID string) (TextRecord, bool) {
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

func (s *TextEventStore) CreateEvent(eventType string, textID *string, segmentID *string, payload map[string]any) (string, error) {
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
