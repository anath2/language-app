"""
Database connection management and migrations.

This module handles:
- SQLite connection management with WAL mode
- Database path configuration
- Schema migrations
"""

import os
import sqlite3
from contextlib import contextmanager
from datetime import datetime, timezone
from pathlib import Path
from typing import Iterator


def _utc_now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def get_db_path() -> Path:
    """
    Returns the sqlite DB path.

    Defaults to: <repo>/app/data/language_app.db
    Override with: LANGUAGE_APP_DB_PATH
    """
    env = os.getenv("LANGUAGE_APP_DB_PATH")
    if env:
        return Path(env).expanduser().resolve()
    return (Path(__file__).resolve().parent.parent / "data" / "language_app.db").resolve()


def _ensure_parent_dir(path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)


@contextmanager
def db_conn() -> Iterator[sqlite3.Connection]:
    path = get_db_path()
    _ensure_parent_dir(path)
    conn = sqlite3.connect(str(path), check_same_thread=False)
    try:
        conn.row_factory = sqlite3.Row
        # Reasonable defaults for small local apps.
        conn.execute("PRAGMA journal_mode=WAL;")
        conn.execute("PRAGMA foreign_keys=ON;")
        yield conn
        conn.commit()
    finally:
        conn.close()


def init_db() -> None:
    with db_conn() as conn:
        conn.execute(
            "CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)"
        )
        current = conn.execute(
            "SELECT COALESCE(MAX(version), 0) AS v FROM schema_migrations"
        ).fetchone()["v"]

        migrations: list[tuple[int, str]] = [
            (1, _migration_001()),
            (2, _migration_002()),
        ]

        for version, sql in migrations:
            if version > int(current):
                conn.executescript(sql)
                conn.execute("INSERT INTO schema_migrations(version) VALUES (?)", (version,))


def _migration_001() -> str:
    # Keep it as executescript-friendly SQL.
    return """
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
"""


def _migration_002() -> str:
    """Add vocab_lookups table for tracking lookup history (struggle detection)."""
    return """
CREATE TABLE IF NOT EXISTS vocab_lookups (
  id TEXT PRIMARY KEY,
  vocab_item_id TEXT NOT NULL,
  looked_up_at TEXT NOT NULL,
  FOREIGN KEY(vocab_item_id) REFERENCES vocab_items(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_vocab_lookups_vocab_item_id ON vocab_lookups(vocab_item_id);
CREATE INDEX IF NOT EXISTS idx_vocab_lookups_looked_up_at ON vocab_lookups(looked_up_at);
"""
