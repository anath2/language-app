# Vocab Data Model Split: Segments and Characters

## Problem

`vocab_items` is a polymorphic table discriminated by a `type` column ('word' vs 'character'). This leads to:
- Columns that don't apply to both types (`english` is meaningless for characters)
- Queries that filter on `type` everywhere
- `character_word_links` naming is inconsistent with the codebase's segment terminology

Splitting into dedicated tables separates concerns and makes each table's schema self-documenting.

## Solution

Replace `vocab_items` with two tables: `saved_segments` (user-saved segments with required English) and `saved_characters` (individual characters with optional English). Replace `character_word_links` with `character_segment_links` carrying segment context.

## New Tables

### `saved_segments`

User-saved segments with required English.

```sql
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
```

### `saved_characters`

Individual characters with optional English — populated only when the segment itself is a single character (the segment's English applies). Multi-character segment decomposition produces characters with empty English; their meaning is conveyed through `character_segment_links.segment_translation`.

```sql
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
```

### `character_segment_links`

Context for each character — which segment it appeared in. Replaces `character_word_links`.

```sql
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
```

### `srs_state`

Stays as one table with two nullable FKs. A CHECK constraint ensures exactly one is set.

```sql
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
  CHECK ((segment_id IS NOT NULL AND character_id IS NULL) OR (segment_id IS NULL AND character_id IS NOT NULL))
);
```

### `vocab_lookups`

Same two-nullable-FK pattern:

```sql
CREATE TABLE vocab_lookups (
  id TEXT PRIMARY KEY,
  segment_id TEXT,
  character_id TEXT,
  looked_up_at TEXT NOT NULL,
  FOREIGN KEY(segment_id) REFERENCES saved_segments(id) ON DELETE CASCADE,
  FOREIGN KEY(character_id) REFERENCES saved_characters(id) ON DELETE CASCADE,
  CHECK ((segment_id IS NOT NULL AND character_id IS NULL) OR (segment_id IS NULL AND character_id IS NOT NULL))
);
```

## Migration Strategy

Single migration (00016 or 00017 depending on ordering with the pipeline spec) that:

1. Creates `saved_segments`, `saved_characters`, `character_segment_links`
2. Migrates data from `vocab_items` where `type='word'` → `saved_segments`
3. Migrates data from `vocab_items` where `type='character'` → `saved_characters`
4. Migrates `character_word_links` → `character_segment_links` (segment context fields will be empty for existing data — backfill is a separate concern)
5. Recreates `srs_state` and `vocab_lookups` with the new FK structure, migrating existing rows
6. Drops `vocab_items` and `character_word_links`

## Impact on Go Code

| Current | New |
|---|---|
| `SaveVocabItem` → `vocab_items` | `SaveSegment` → `saved_segments` |
| `ExtractAndLinkCharacters` → `vocab_items` type='character' + `character_word_links` | `ExtractAndLinkCharacters` → `saved_characters` + `character_segment_links` |
| `GetReviewQueue` filters `type='word'` | `GetSegmentReviewQueue` queries `saved_segments` directly |
| `GetCharacterReviewQueue` filters `type='character'` + joins `character_word_links` | `GetCharacterReviewQueue` queries `saved_characters` + joins `character_segment_links` |
| `GetDueCount` / `GetCharacterDueCount` filter on `type` | Each queries its own table directly |
| `RecordReviewAnswer` reads `type` to branch due count | Caller passes entity type, or separate methods per table |
| `CountVocabByStatus` / `CountTotalVocab` filter `type='word'` | Query `saved_segments` directly |
| `GetVocabSRSInfo` queries `vocab_items` | Queries `saved_segments` (segment-level SRS info only) |
| `RecordLookup` queries `vocab_items` | Queries whichever table matches the ID |
| `UpdateVocabStatus` updates `vocab_items` | `UpdateSegmentStatus` / `UpdateCharacterStatus` on respective tables |
| `ExportProgressJSON` dumps `vocab_items` | Dumps `saved_segments` + `saved_characters` separately |
| `ImportProgressJSON` loads `vocab_items` | Loads both tables separately |

## Go Type Changes

| Current | New |
|---|---|
| `VocabRecord` | `SegmentRecord` |
| `VocabSRSInfo` | `SegmentSRSInfo` |
| `ReviewCard` | `SegmentReviewCard` |
| `CharacterReviewCard` (keeps `English`) | `CharacterReviewCard` — `English` optional, `ExampleSegments` replaces `ExampleWords` |
| `CharacterExampleWord` | `CharacterExampleSegment` with segment + segment_pinyin + segment_translation |

## Character English Logic

When `ExtractAndLinkCharacters` processes a saved segment:
- If the segment is a single character (e.g., user saves 行 directly): the character gets the segment's English
- If the segment is multi-character (e.g., 银行): decomposed characters (银, 行) get empty English — context comes from `character_segment_links.segment_translation`

## Dedup Rules

- **Segments**: `(headword, pinyin)` — same segment text with same reading is one row. English may be updated on subsequent saves.
- **Characters**: `(character, pinyin)` — same character with same reading is one row. Different readings (行 háng vs xíng) are separate.
- **Character-segment links**: `(character_id, segment, segment_pinyin)` — same character in same segment with same reading deduplicates.

## Handler/Interface Changes

- `handlers/deps.go`: `srsStore` interface updated — `ExtractAndLinkCharacters` loses `cedictLookup` param, gains segment data param
- `handlers/vocab.go`: `SaveVocab` handler calls `SaveSegment` + `ExtractAndLinkCharacters` with segment data from DB
- Review endpoints return data from the new tables
- Export/import endpoints serialize/deserialize both tables
