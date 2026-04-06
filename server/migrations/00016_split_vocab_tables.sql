-- +goose Up
PRAGMA foreign_keys = OFF;

CREATE TABLE saved_segments (
  id TEXT PRIMARY KEY,
  headword TEXT NOT NULL,
  pinyin TEXT NOT NULL DEFAULT '',
  english TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'learning',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  last_seen_translation_id TEXT,
  last_seen_snippet TEXT NOT NULL DEFAULT '',
  last_seen_at TEXT,
  seen_count INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX ux_saved_segments_key ON saved_segments(headword, pinyin);

CREATE TABLE saved_characters (
  id TEXT PRIMARY KEY,
  character TEXT NOT NULL,
  pinyin TEXT NOT NULL DEFAULT '',
  english TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'learning',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX ux_saved_characters_key ON saved_characters(character, pinyin);

CREATE TABLE character_segment_links (
  id TEXT PRIMARY KEY,
  character_id TEXT NOT NULL,
  segment TEXT NOT NULL,
  segment_pinyin TEXT NOT NULL DEFAULT '',
  segment_translation TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  FOREIGN KEY(character_id) REFERENCES saved_characters(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX ux_char_segment_link ON character_segment_links(character_id, segment, segment_pinyin);
CREATE INDEX idx_char_segment_links_char ON character_segment_links(character_id);

-- Map old segment rows to a canonical saved_segments row by (headword, pinyin),
-- keeping the most recently updated row as the canonical source of english/status.
CREATE TEMP TABLE temp_segment_map AS
WITH ranked AS (
  SELECT
    v.id AS old_id,
    v.headword,
    v.pinyin,
    ROW_NUMBER() OVER (
      PARTITION BY v.headword, v.pinyin
      ORDER BY v.updated_at DESC, v.created_at DESC, v.id DESC
    ) AS rn
  FROM vocab_items v
  WHERE v.type = 'word'
),
winners AS (
  SELECT
    v.id,
    v.headword,
    v.pinyin,
    v.english,
    v.status,
    v.created_at,
    v.updated_at,
    v.last_seen_translation_id,
    v.last_seen_snippet,
    v.last_seen_at,
    v.seen_count
  FROM ranked r
  JOIN vocab_items v ON v.id = r.old_id
  WHERE r.rn = 1
),
inserted AS (
  SELECT
    w.id AS new_id,
    w.headword,
    w.pinyin,
    w.english,
    w.status,
    w.created_at,
    w.updated_at,
    w.last_seen_translation_id,
    w.last_seen_snippet,
    w.last_seen_at,
    w.seen_count
  FROM winners w
)
SELECT r.old_id, i.new_id
FROM ranked r
JOIN inserted i
  ON i.headword = r.headword
 AND i.pinyin = r.pinyin;

INSERT INTO saved_segments (
  id, headword, pinyin, english, status, created_at, updated_at,
  last_seen_translation_id, last_seen_snippet, last_seen_at, seen_count
)
WITH ranked AS (
  SELECT
    v.*,
    ROW_NUMBER() OVER (
      PARTITION BY v.headword, v.pinyin
      ORDER BY v.updated_at DESC, v.created_at DESC, v.id DESC
    ) AS rn
  FROM vocab_items v
  WHERE v.type = 'word'
)
SELECT
  id,
  headword,
  pinyin,
  english,
  status,
  created_at,
  updated_at,
  last_seen_translation_id,
  last_seen_snippet,
  last_seen_at,
  seen_count
FROM ranked
WHERE rn = 1;

CREATE TEMP TABLE temp_character_map AS
WITH ranked AS (
  SELECT
    v.id AS old_id,
    v.headword AS character,
    v.pinyin,
    ROW_NUMBER() OVER (
      PARTITION BY v.headword, v.pinyin
      ORDER BY v.updated_at DESC, v.created_at DESC, v.id DESC
    ) AS rn
  FROM vocab_items v
  WHERE v.type = 'character'
),
winners AS (
  SELECT
    v.id,
    v.headword AS character,
    v.pinyin,
    v.english,
    v.status,
    v.created_at,
    v.updated_at
  FROM ranked r
  JOIN vocab_items v ON v.id = r.old_id
  WHERE r.rn = 1
),
inserted AS (
  SELECT
    w.id AS new_id,
    w.character,
    w.pinyin,
    w.english,
    w.status,
    w.created_at,
    w.updated_at
  FROM winners w
)
SELECT r.old_id, i.new_id
FROM ranked r
JOIN inserted i
  ON i.character = r.character
 AND i.pinyin = r.pinyin;

INSERT INTO saved_characters (
  id, character, pinyin, english, status, created_at, updated_at
)
WITH ranked AS (
  SELECT
    v.*,
    ROW_NUMBER() OVER (
      PARTITION BY v.headword, v.pinyin
      ORDER BY v.updated_at DESC, v.created_at DESC, v.id DESC
    ) AS rn
  FROM vocab_items v
  WHERE v.type = 'character'
)
SELECT
  id,
  headword AS character,
  pinyin,
  english,
  status,
  created_at,
  updated_at
FROM ranked
WHERE rn = 1;

INSERT OR IGNORE INTO character_segment_links (
  id, character_id, segment, segment_pinyin, segment_translation, created_at
)
SELECT
  cwl.id,
  cm.new_id AS character_id,
  COALESCE(ss.headword, '') AS segment,
  COALESCE(ss.pinyin, '') AS segment_pinyin,
  COALESCE(ss.english, '') AS segment_translation,
  cwl.created_at
FROM character_word_links cwl
JOIN temp_character_map cm ON cm.old_id = cwl.character_item_id
LEFT JOIN temp_segment_map sm ON sm.old_id = cwl.word_item_id
LEFT JOIN saved_segments ss ON ss.id = sm.new_id;

ALTER TABLE srs_state RENAME TO srs_state_old;
CREATE TABLE srs_state (
  id TEXT PRIMARY KEY,
  segment_id TEXT,
  character_id TEXT,
  due_at TEXT,
  interval_days REAL NOT NULL DEFAULT 0,
  ease REAL NOT NULL DEFAULT 2.5,
  reps INTEGER NOT NULL DEFAULT 0,
  lapses INTEGER NOT NULL DEFAULT 0,
  last_reviewed_at TEXT,
  FOREIGN KEY(segment_id) REFERENCES saved_segments(id) ON DELETE CASCADE,
  FOREIGN KEY(character_id) REFERENCES saved_characters(id) ON DELETE CASCADE,
  CHECK (
    (segment_id IS NOT NULL AND character_id IS NULL) OR
    (segment_id IS NULL AND character_id IS NOT NULL)
  )
);
CREATE INDEX idx_srs_state_segment_id ON srs_state(segment_id);
CREATE INDEX idx_srs_state_character_id ON srs_state(character_id);
CREATE INDEX idx_srs_state_due_at ON srs_state(due_at);

INSERT INTO srs_state (
  id, segment_id, character_id, due_at, interval_days, ease, reps, lapses, last_reviewed_at
)
WITH segment_rows AS (
  SELECT
    sm.new_id AS entity_id,
    ss.due_at,
    ss.interval_days,
    ss.ease,
    ss.reps,
    ss.lapses,
    ss.last_reviewed_at,
    ROW_NUMBER() OVER (
      PARTITION BY sm.new_id
      ORDER BY COALESCE(ss.last_reviewed_at, ss.due_at, '') DESC, ss.vocab_item_id DESC
    ) AS rn
  FROM srs_state_old ss
  JOIN temp_segment_map sm ON sm.old_id = ss.vocab_item_id
),
character_rows AS (
  SELECT
    cm.new_id AS entity_id,
    ss.due_at,
    ss.interval_days,
    ss.ease,
    ss.reps,
    ss.lapses,
    ss.last_reviewed_at,
    ROW_NUMBER() OVER (
      PARTITION BY cm.new_id
      ORDER BY COALESCE(ss.last_reviewed_at, ss.due_at, '') DESC, ss.vocab_item_id DESC
    ) AS rn
  FROM srs_state_old ss
  JOIN temp_character_map cm ON cm.old_id = ss.vocab_item_id
)
SELECT
  'seg-' || entity_id AS id,
  entity_id AS segment_id,
  NULL AS character_id,
  due_at,
  interval_days,
  ease,
  reps,
  lapses,
  last_reviewed_at
FROM segment_rows
WHERE rn = 1
UNION ALL
SELECT
  'char-' || entity_id AS id,
  NULL AS segment_id,
  entity_id AS character_id,
  due_at,
  interval_days,
  ease,
  reps,
  lapses,
  last_reviewed_at
FROM character_rows
WHERE rn = 1;

ALTER TABLE vocab_lookups RENAME TO vocab_lookups_old;
CREATE TABLE vocab_lookups (
  id TEXT PRIMARY KEY,
  segment_id TEXT,
  character_id TEXT,
  looked_up_at TEXT NOT NULL,
  FOREIGN KEY(segment_id) REFERENCES saved_segments(id) ON DELETE CASCADE,
  FOREIGN KEY(character_id) REFERENCES saved_characters(id) ON DELETE CASCADE,
  CHECK (
    (segment_id IS NOT NULL AND character_id IS NULL) OR
    (segment_id IS NULL AND character_id IS NOT NULL)
  )
);
DROP INDEX IF EXISTS idx_vocab_lookups_vocab_item_id;
DROP INDEX IF EXISTS idx_vocab_lookups_looked_up_at;
CREATE INDEX idx_vocab_lookups_segment_id ON vocab_lookups(segment_id);
CREATE INDEX idx_vocab_lookups_character_id ON vocab_lookups(character_id);
CREATE INDEX idx_vocab_lookups_looked_up_at ON vocab_lookups(looked_up_at);

INSERT INTO vocab_lookups (id, segment_id, character_id, looked_up_at)
SELECT
  vl.id,
  sm.new_id AS segment_id,
  NULL AS character_id,
  vl.looked_up_at
FROM vocab_lookups_old vl
JOIN temp_segment_map sm ON sm.old_id = vl.vocab_item_id
UNION ALL
SELECT
  vl.id || '-char' AS id,
  NULL AS segment_id,
  cm.new_id AS character_id,
  vl.looked_up_at
FROM vocab_lookups_old vl
JOIN temp_character_map cm ON cm.old_id = vl.vocab_item_id;

DROP TABLE vocab_lookups_old;
DROP TABLE srs_state_old;
DROP TABLE character_word_links;
DROP TABLE vocab_items;

DROP TABLE temp_segment_map;
DROP TABLE temp_character_map;

PRAGMA foreign_keys = ON;

-- +goose Down
SELECT 1;
