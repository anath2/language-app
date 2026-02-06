-- Add vocab_lookups table for tracking lookup history (struggle detection)

CREATE TABLE IF NOT EXISTS vocab_lookups (
  id TEXT PRIMARY KEY,
  vocab_item_id TEXT NOT NULL,
  looked_up_at TEXT NOT NULL,
  FOREIGN KEY(vocab_item_id) REFERENCES vocab_items(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_vocab_lookups_vocab_item_id ON vocab_lookups(vocab_item_id);
CREATE INDEX IF NOT EXISTS idx_vocab_lookups_looked_up_at ON vocab_lookups(looked_up_at);
