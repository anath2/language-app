-- +goose Up
-- +goose StatementBegin
-- Add user_profile table for storing user settings (single-row table for MAU=1)
CREATE TABLE IF NOT EXISTS user_profile (
  id INTEGER PRIMARY KEY CHECK (id = 1), -- enforce single row
  name TEXT NOT NULL DEFAULT '',
  email TEXT NOT NULL DEFAULT '',
  language TEXT NOT NULL DEFAULT 'zh-CN',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

-- Insert default profile row if not exists
INSERT OR IGNORE INTO user_profile (id, name, email, language, created_at, updated_at)
VALUES (1, '', '', 'zh-CN', datetime('now'), datetime('now'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_profile;
-- +goose StatementEnd
