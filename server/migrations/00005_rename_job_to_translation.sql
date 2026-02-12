-- +goose Up
-- +goose StatementBegin
-- Migration 005: Rename job tables to translation tables
ALTER TABLE jobs RENAME TO translations;
ALTER TABLE translations RENAME COLUMN job_type TO translation_type;

CREATE TABLE translation_segments (
  id TEXT PRIMARY KEY,
  translation_id TEXT NOT NULL,
  paragraph_idx INTEGER NOT NULL,
  seg_idx INTEGER NOT NULL,
  segment_text TEXT NOT NULL,
  pinyin TEXT NOT NULL DEFAULT '',
  english TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  FOREIGN KEY(translation_id) REFERENCES translations(id) ON DELETE CASCADE
);

INSERT INTO translation_segments (id, translation_id, paragraph_idx, seg_idx, segment_text, pinyin, english, created_at)
SELECT id, job_id, paragraph_idx, seg_idx, segment_text, pinyin, english, created_at
FROM job_segments;

DROP TABLE job_segments;

CREATE INDEX idx_translation_segments_translation_id ON translation_segments(translation_id);
CREATE INDEX idx_translation_segments_order ON translation_segments(translation_id, paragraph_idx, seg_idx);

CREATE TABLE translation_paragraphs (
  id TEXT PRIMARY KEY,
  translation_id TEXT NOT NULL,
  paragraph_idx INTEGER NOT NULL,
  indent TEXT NOT NULL DEFAULT '',
  separator TEXT NOT NULL DEFAULT '',
  FOREIGN KEY(translation_id) REFERENCES translations(id) ON DELETE CASCADE
);

INSERT INTO translation_paragraphs (id, translation_id, paragraph_idx, indent, separator)
SELECT id, job_id, paragraph_idx, indent, separator
FROM job_paragraphs;

DROP TABLE job_paragraphs;

CREATE INDEX idx_translation_paragraphs_translation_id ON translation_paragraphs(translation_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- no-op down for table rename/data-move migration
SELECT 1;
-- +goose StatementEnd
