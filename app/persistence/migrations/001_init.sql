-- Initial schema: texts, segments, events, vocab_items, vocab_occurrences, srs_state

CREATE TABLE IF NOT EXISTS texts (
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,
  source_type TEXT NOT NULL, -- 'text' | 'ocr' | future
  raw_text TEXT NOT NULL,
  normalized_text TEXT NOT NULL,
  metadata_json TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS segments (
  id TEXT PRIMARY KEY,
  text_id TEXT NOT NULL,
  paragraph_idx INTEGER NOT NULL,
  seg_idx INTEGER NOT NULL,
  segment_text TEXT NOT NULL,
  pinyin TEXT NOT NULL DEFAULT '',
  english TEXT NOT NULL DEFAULT '',
  provider_meta_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  FOREIGN KEY(text_id) REFERENCES texts(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_segments_text_id ON segments(text_id);

CREATE TABLE IF NOT EXISTS events (
  id TEXT PRIMARY KEY,
  ts TEXT NOT NULL,
  text_id TEXT,
  segment_id TEXT,
  event_type TEXT NOT NULL,
  payload_json TEXT NOT NULL DEFAULT '{}',
  FOREIGN KEY(text_id) REFERENCES texts(id) ON DELETE SET NULL,
  FOREIGN KEY(segment_id) REFERENCES segments(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_events_ts ON events(ts);
CREATE INDEX IF NOT EXISTS idx_events_text_id ON events(text_id);

CREATE TABLE IF NOT EXISTS vocab_items (
  id TEXT PRIMARY KEY,
  headword TEXT NOT NULL,
  pinyin TEXT NOT NULL DEFAULT '',
  english TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'unknown', -- unknown|learning|known
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_vocab_items_key
  ON vocab_items(headword, pinyin, english);

CREATE TABLE IF NOT EXISTS vocab_occurrences (
  id TEXT PRIMARY KEY,
  vocab_item_id TEXT NOT NULL,
  text_id TEXT,
  segment_id TEXT,
  snippet TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  FOREIGN KEY(vocab_item_id) REFERENCES vocab_items(id) ON DELETE CASCADE,
  FOREIGN KEY(text_id) REFERENCES texts(id) ON DELETE SET NULL,
  FOREIGN KEY(segment_id) REFERENCES segments(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_vocab_occ_vocab_item_id ON vocab_occurrences(vocab_item_id);
CREATE INDEX IF NOT EXISTS idx_vocab_occ_text_id ON vocab_occurrences(text_id);

CREATE TABLE IF NOT EXISTS srs_state (
  vocab_item_id TEXT PRIMARY KEY,
  due_at TEXT,
  interval_days REAL NOT NULL DEFAULT 0,
  ease REAL NOT NULL DEFAULT 2.5,
  reps INTEGER NOT NULL DEFAULT 0,
  lapses INTEGER NOT NULL DEFAULT 0,
  last_reviewed_at TEXT,
  FOREIGN KEY(vocab_item_id) REFERENCES vocab_items(id) ON DELETE CASCADE
);
