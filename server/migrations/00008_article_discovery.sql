-- +goose Up
-- +goose StatementBegin

-- User topic preferences that guide article discovery
CREATE TABLE IF NOT EXISTS discovery_preferences (
  id TEXT PRIMARY KEY,
  topic TEXT NOT NULL,
  weight REAL NOT NULL DEFAULT 1.0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_discovery_preferences_topic ON discovery_preferences(topic);

-- Log of each discovery pipeline run
CREATE TABLE IF NOT EXISTS discovery_runs (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL DEFAULT 'pending', -- pending, running, completed, failed
  trigger_type TEXT NOT NULL DEFAULT 'scheduled', -- scheduled, manual
  articles_found INTEGER NOT NULL DEFAULT 0,
  error_message TEXT,
  started_at TEXT NOT NULL,
  completed_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_discovery_runs_status ON discovery_runs(status);
CREATE INDEX IF NOT EXISTS idx_discovery_runs_started_at ON discovery_runs(started_at);

-- Discovered article recommendations
CREATE TABLE IF NOT EXISTS article_recommendations (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  url TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  source_name TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  difficulty_score REAL NOT NULL DEFAULT 0.0, -- 0-1 scale
  total_words INTEGER NOT NULL DEFAULT 0,
  unknown_words INTEGER NOT NULL DEFAULT 0,
  learning_words INTEGER NOT NULL DEFAULT 0,
  known_words INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'new', -- new, dismissed, imported
  translation_id TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY(run_id) REFERENCES discovery_runs(id) ON DELETE CASCADE,
  FOREIGN KEY(translation_id) REFERENCES translations(id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_article_recommendations_url ON article_recommendations(url);
CREATE INDEX IF NOT EXISTS idx_article_recommendations_status ON article_recommendations(status);
CREATE INDEX IF NOT EXISTS idx_article_recommendations_difficulty ON article_recommendations(difficulty_score);
CREATE INDEX IF NOT EXISTS idx_article_recommendations_created_at ON article_recommendations(created_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS article_recommendations;
DROP TABLE IF EXISTS discovery_runs;
DROP TABLE IF EXISTS discovery_preferences;
-- +goose StatementEnd
