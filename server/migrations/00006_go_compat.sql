-- +goose Up
-- +goose StatementBegin
-- Go runtime compatibility deltas on top of canonical Python schema.

ALTER TABLE translations ADD COLUMN progress INTEGER NOT NULL DEFAULT 0;
ALTER TABLE translations ADD COLUMN total INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS translation_jobs (
  translation_id TEXT PRIMARY KEY,
  state TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  lease_until TEXT,
  last_error TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (translation_id) REFERENCES translations(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_translation_paragraphs_pair
  ON translation_paragraphs(translation_id, paragraph_idx);
CREATE UNIQUE INDEX IF NOT EXISTS ux_translation_segments_order
  ON translation_segments(translation_id, paragraph_idx, seg_idx);

INSERT INTO translation_jobs (translation_id, state, attempts, lease_until, last_error, created_at, updated_at)
SELECT t.id, 'pending', 0, NULL, NULL, t.created_at, COALESCE(t.updated_at, t.created_at)
FROM translations t
WHERE t.status IN ('pending', 'processing')
  AND NOT EXISTS (
    SELECT 1 FROM translation_jobs j WHERE j.translation_id = t.id
  );

INSERT OR IGNORE INTO user_profile (id, name, email, language, created_at, updated_at)
VALUES (1, '', '', 'zh-CN', datetime('now'), datetime('now'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- no-op down for compatibility migration
SELECT 1;
-- +goose StatementEnd
