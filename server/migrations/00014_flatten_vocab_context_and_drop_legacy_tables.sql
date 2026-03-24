-- +goose Up
ALTER TABLE vocab_items ADD COLUMN last_seen_translation_id TEXT;
ALTER TABLE vocab_items ADD COLUMN last_seen_snippet TEXT NOT NULL DEFAULT '';
ALTER TABLE vocab_items ADD COLUMN last_seen_at TEXT;
ALTER TABLE vocab_items ADD COLUMN seen_count INTEGER NOT NULL DEFAULT 0;

UPDATE vocab_items
SET
  last_seen_translation_id = (
    SELECT vo.text_id
    FROM vocab_occurrences vo
    WHERE vo.vocab_item_id = vocab_items.id
    ORDER BY
      CASE WHEN COALESCE(vo.snippet, '') <> '' THEN 0 ELSE 1 END,
      vo.created_at DESC
    LIMIT 1
  ),
  last_seen_snippet = COALESCE((
    SELECT vo.snippet
    FROM vocab_occurrences vo
    WHERE vo.vocab_item_id = vocab_items.id
    ORDER BY
      CASE WHEN COALESCE(vo.snippet, '') <> '' THEN 0 ELSE 1 END,
      vo.created_at DESC
    LIMIT 1
  ), ''),
  last_seen_at = (
    SELECT vo.created_at
    FROM vocab_occurrences vo
    WHERE vo.vocab_item_id = vocab_items.id
    ORDER BY vo.created_at DESC
    LIMIT 1
  ),
  seen_count = (
    SELECT COUNT(*)
    FROM vocab_occurrences vo
    WHERE vo.vocab_item_id = vocab_items.id
  );

DROP TABLE IF EXISTS vocab_occurrences;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS segments;

-- +goose Down
SELECT 1;
