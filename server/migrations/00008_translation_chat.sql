-- +goose Up
CREATE TABLE translation_chats (
  id TEXT PRIMARY KEY,
  translation_id TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY(translation_id) REFERENCES translations(id) ON DELETE CASCADE
);
CREATE INDEX idx_translation_chats_translation_id ON translation_chats(translation_id);

CREATE TABLE translation_chat_messages (
  id TEXT PRIMARY KEY,
  chat_id TEXT NOT NULL,
  translation_id TEXT NOT NULL,
  message_idx INTEGER NOT NULL,
  role TEXT NOT NULL,
  content TEXT NOT NULL,
  selected_segment_ids_json TEXT NOT NULL DEFAULT '[]',
  created_at TEXT NOT NULL,
  FOREIGN KEY(chat_id) REFERENCES translation_chats(id) ON DELETE CASCADE,
  FOREIGN KEY(translation_id) REFERENCES translations(id) ON DELETE CASCADE
);
CREATE INDEX idx_translation_chat_messages_translation_id ON translation_chat_messages(translation_id);
CREATE UNIQUE INDEX ux_translation_chat_messages_order ON translation_chat_messages(translation_id, message_idx);

-- +goose Down
DROP INDEX IF EXISTS ux_translation_chat_messages_order;
DROP INDEX IF EXISTS idx_translation_chat_messages_translation_id;
DROP TABLE IF EXISTS translation_chat_messages;
DROP INDEX IF EXISTS idx_translation_chats_translation_id;
DROP TABLE IF EXISTS translation_chats;
