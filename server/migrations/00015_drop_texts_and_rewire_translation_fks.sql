-- +goose Up
PRAGMA foreign_keys = OFF;

ALTER TABLE translation_chat_messages RENAME TO translation_chat_messages_old;
ALTER TABLE translation_chats RENAME TO translation_chats_old;
ALTER TABLE translation_jobs RENAME TO translation_jobs_old;
ALTER TABLE translation_segments RENAME TO translation_segments_old;
ALTER TABLE translation_sentences RENAME TO translation_sentences_old;
ALTER TABLE translations RENAME TO translations_old;

CREATE TABLE translations (
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  translation_type TEXT NOT NULL DEFAULT 'translation',
  source_type TEXT NOT NULL DEFAULT 'text',
  input_text TEXT NOT NULL,
  full_translation TEXT,
  error_message TEXT,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  progress INTEGER NOT NULL DEFAULT 0,
  total INTEGER NOT NULL DEFAULT 0,
  title TEXT NOT NULL DEFAULT ''
);

INSERT INTO translations (
  id, created_at, updated_at, status, translation_type, source_type, input_text,
  full_translation, error_message, metadata_json, progress, total, title
)
SELECT
  id, created_at, updated_at, status, translation_type, source_type, input_text,
  full_translation, error_message, metadata_json, progress, total, title
FROM translations_old;

CREATE TABLE translation_sentences (
  id TEXT PRIMARY KEY,
  translation_id TEXT NOT NULL,
  sentence_idx INTEGER NOT NULL,
  indent TEXT NOT NULL DEFAULT '',
  separator TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL DEFAULT '',
  FOREIGN KEY(translation_id) REFERENCES translations(id) ON DELETE CASCADE
);

INSERT INTO translation_sentences (id, translation_id, sentence_idx, indent, separator, content_hash)
SELECT id, translation_id, sentence_idx, indent, separator, content_hash
FROM translation_sentences_old;

CREATE TABLE translation_segments (
  id TEXT PRIMARY KEY,
  translation_id TEXT NOT NULL,
  sentence_idx INTEGER NOT NULL,
  seg_idx INTEGER NOT NULL,
  segment_text TEXT NOT NULL,
  pinyin TEXT NOT NULL DEFAULT '',
  english TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  FOREIGN KEY(translation_id) REFERENCES translations(id) ON DELETE CASCADE
);

INSERT INTO translation_segments (id, translation_id, sentence_idx, seg_idx, segment_text, pinyin, english, created_at)
SELECT id, translation_id, sentence_idx, seg_idx, segment_text, pinyin, english, created_at
FROM translation_segments_old;

CREATE TABLE translation_jobs (
  translation_id TEXT PRIMARY KEY,
  state TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  lease_until TEXT,
  last_error TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY(translation_id) REFERENCES translations(id) ON DELETE CASCADE
);

INSERT INTO translation_jobs (translation_id, state, attempts, lease_until, last_error, created_at, updated_at)
SELECT translation_id, state, attempts, lease_until, last_error, created_at, updated_at
FROM translation_jobs_old;

CREATE TABLE translation_chats (
  id TEXT PRIMARY KEY,
  translation_id TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY(translation_id) REFERENCES translations(id) ON DELETE CASCADE
);

INSERT INTO translation_chats (id, translation_id, created_at, updated_at)
SELECT id, translation_id, created_at, updated_at
FROM translation_chats_old;

CREATE TABLE translation_chat_messages (
  id TEXT PRIMARY KEY,
  chat_id TEXT NOT NULL,
  translation_id TEXT NOT NULL,
  message_idx INTEGER NOT NULL,
  role TEXT NOT NULL,
  content TEXT NOT NULL,
  selected_text TEXT,
  created_at TEXT NOT NULL,
  review_card_json TEXT NULL,
  FOREIGN KEY(chat_id) REFERENCES translation_chats(id) ON DELETE CASCADE,
  FOREIGN KEY(translation_id) REFERENCES translations(id) ON DELETE CASCADE
);

INSERT INTO translation_chat_messages (
  id, chat_id, translation_id, message_idx, role, content, selected_text, created_at, review_card_json
)
SELECT id, chat_id, translation_id, message_idx, role, content, selected_text, created_at, review_card_json
FROM translation_chat_messages_old;

DROP TABLE translation_chat_messages_old;
DROP TABLE translation_chats_old;
DROP TABLE translation_jobs_old;
DROP TABLE translation_segments_old;
DROP TABLE translation_sentences_old;
DROP TABLE translations_old;

CREATE INDEX idx_jobs_status ON translations(status);
CREATE INDEX idx_jobs_created_at ON translations(created_at);
CREATE INDEX idx_translation_segments_translation_id ON translation_segments(translation_id);
CREATE INDEX idx_translation_segments_order ON translation_segments(translation_id, sentence_idx, seg_idx);
CREATE UNIQUE INDEX ux_translation_segments_order ON translation_segments(translation_id, sentence_idx, seg_idx);
CREATE INDEX idx_translation_paragraphs_translation_id ON translation_sentences(translation_id);
CREATE UNIQUE INDEX ux_translation_paragraphs_pair ON translation_sentences(translation_id, sentence_idx);
CREATE INDEX idx_translation_chats_translation_id ON translation_chats(translation_id);
CREATE INDEX idx_translation_chat_messages_translation_id ON translation_chat_messages(translation_id);
CREATE UNIQUE INDEX ux_translation_chat_messages_order ON translation_chat_messages(translation_id, message_idx);

DROP TABLE IF EXISTS texts;

PRAGMA foreign_keys = ON;

-- +goose Down
SELECT 1;
