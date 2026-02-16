-- +goose Up
ALTER TABLE vocab_items ADD COLUMN type TEXT NOT NULL DEFAULT 'word';

-- Recreate unique index to include type (allows same headword as both word and character)
DROP INDEX IF EXISTS ux_vocab_items_key;
CREATE UNIQUE INDEX ux_vocab_items_key ON vocab_items(headword, pinyin, english, type);

CREATE TABLE character_word_links (
  id TEXT PRIMARY KEY,
  character_item_id TEXT NOT NULL,
  word_item_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(character_item_id) REFERENCES vocab_items(id) ON DELETE CASCADE,
  FOREIGN KEY(word_item_id) REFERENCES vocab_items(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX ux_char_word_link ON character_word_links(character_item_id, word_item_id);
CREATE INDEX idx_char_word_links_char ON character_word_links(character_item_id);
CREATE INDEX idx_char_word_links_word ON character_word_links(word_item_id);

-- +goose Down
DROP INDEX IF EXISTS idx_char_word_links_word;
DROP INDEX IF EXISTS idx_char_word_links_char;
DROP INDEX IF EXISTS ux_char_word_link;
DROP TABLE IF EXISTS character_word_links;
-- SQLite cannot drop columns; type column stays on rollback
