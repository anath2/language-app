-- +goose Up
-- +goose StatementBegin
-- Job queue tables for translation tasks
CREATE TABLE IF NOT EXISTS jobs (
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending', -- pending, processing, completed, failed
  job_type TEXT NOT NULL DEFAULT 'translation',
  source_type TEXT NOT NULL DEFAULT 'text', -- text, ocr
  input_text TEXT NOT NULL,
  full_translation TEXT,
  error_message TEXT,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  text_id TEXT,
  FOREIGN KEY(text_id) REFERENCES texts(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at);

CREATE TABLE IF NOT EXISTS job_segments (
  id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL,
  paragraph_idx INTEGER NOT NULL,
  seg_idx INTEGER NOT NULL,
  segment_text TEXT NOT NULL,
  pinyin TEXT NOT NULL DEFAULT '',
  english TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  FOREIGN KEY(job_id) REFERENCES jobs(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_job_segments_job_id ON job_segments(job_id);
CREATE INDEX IF NOT EXISTS idx_job_segments_order ON job_segments(job_id, paragraph_idx, seg_idx);

CREATE TABLE IF NOT EXISTS job_paragraphs (
  id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL,
  paragraph_idx INTEGER NOT NULL,
  indent TEXT NOT NULL DEFAULT '',
  separator TEXT NOT NULL DEFAULT '',
  FOREIGN KEY(job_id) REFERENCES jobs(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_job_paragraphs_job_id ON job_paragraphs(job_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS job_paragraphs;
DROP TABLE IF EXISTS job_segments;
DROP TABLE IF EXISTS jobs;
-- +goose StatementEnd
