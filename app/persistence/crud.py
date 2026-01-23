"""
CRUD operations for texts, events, and vocabulary items.
"""

import json
import uuid
from datetime import datetime, timezone
from typing import Any

from app.persistence.db import db_conn
from app.persistence.models import TextRecord
from app.persistence.srs import DEFAULT_EASE


def _utc_now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def create_text(*, raw_text: str, source_type: str, metadata: dict[str, Any] | None) -> TextRecord:
    text_id = uuid.uuid4().hex
    created_at = _utc_now_iso()
    normalized_text = raw_text.strip()
    metadata_json = json.dumps(metadata or {}, ensure_ascii=False)

    with db_conn() as conn:
        conn.execute(
            """
            INSERT INTO texts (id, created_at, source_type, raw_text, normalized_text, metadata_json)
            VALUES (?, ?, ?, ?, ?, ?)
            """,
            (text_id, created_at, source_type, raw_text, normalized_text, metadata_json),
        )

    return TextRecord(
        id=text_id,
        created_at=created_at,
        source_type=source_type,
        raw_text=raw_text,
        normalized_text=normalized_text,
        metadata=json.loads(metadata_json),
    )


def get_text(text_id: str) -> TextRecord | None:
    with db_conn() as conn:
        row = conn.execute("SELECT * FROM texts WHERE id = ?", (text_id,)).fetchone()
        if row is None:
            return None
        return TextRecord(
            id=row["id"],
            created_at=row["created_at"],
            source_type=row["source_type"],
            raw_text=row["raw_text"],
            normalized_text=row["normalized_text"],
            metadata=json.loads(row["metadata_json"] or "{}"),
        )


def create_event(
    *,
    event_type: str,
    text_id: str | None,
    segment_id: str | None,
    payload: dict[str, Any] | None,
) -> str:
    event_id = uuid.uuid4().hex
    ts = _utc_now_iso()
    payload_json = json.dumps(payload or {}, ensure_ascii=False)
    with db_conn() as conn:
        conn.execute(
            """
            INSERT INTO events (id, ts, text_id, segment_id, event_type, payload_json)
            VALUES (?, ?, ?, ?, ?, ?)
            """,
            (event_id, ts, text_id, segment_id, event_type, payload_json),
        )
    return event_id


def save_vocab_item(
    *,
    headword: str,
    pinyin: str,
    english: str,
    text_id: str | None,
    segment_id: str | None,
    snippet: str | None,
    status: str = "learning",
) -> str:
    """
    Upsert-like behavior based on (headword, pinyin, english).
    Returns vocab_item_id.
    """
    if status not in {"unknown", "learning", "known"}:
        raise ValueError("Invalid status")
    now = _utc_now_iso()
    snippet = snippet or ""

    with db_conn() as conn:
        # Insert or ignore; then select.
        vocab_item_id = uuid.uuid4().hex
        conn.execute(
            """
            INSERT OR IGNORE INTO vocab_items (id, headword, pinyin, english, status, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?)
            """,
            (vocab_item_id, headword, pinyin, english, status, now, now),
        )
        row = conn.execute(
            """
            SELECT id FROM vocab_items
            WHERE headword = ? AND pinyin = ? AND english = ?
            """,
            (headword, pinyin, english),
        ).fetchone()
        if row is None:
            # Extremely unlikely; fall back to the generated id.
            resolved_id = vocab_item_id
        else:
            resolved_id = row["id"]
            conn.execute(
                "UPDATE vocab_items SET updated_at = ? WHERE id = ?",
                (now, resolved_id),
            )

        occ_id = uuid.uuid4().hex
        conn.execute(
            """
            INSERT INTO vocab_occurrences (id, vocab_item_id, text_id, segment_id, snippet, created_at)
            VALUES (?, ?, ?, ?, ?, ?)
            """,
            (occ_id, resolved_id, text_id, segment_id, snippet, now),
        )

        # Auto-initialize SRS state for new vocab items
        conn.execute(
            """
            INSERT OR IGNORE INTO srs_state (vocab_item_id, due_at, interval_days, ease, reps, lapses, last_reviewed_at)
            VALUES (?, ?, 0, ?, 0, 0, ?)
            """,
            (resolved_id, now, DEFAULT_EASE, now),
        )

    return resolved_id


def update_vocab_status(*, vocab_item_id: str, status: str) -> None:
    if status not in {"unknown", "learning", "known"}:
        raise ValueError("Invalid status")
    now = _utc_now_iso()
    with db_conn() as conn:
        conn.execute(
            "UPDATE vocab_items SET status = ?, updated_at = ? WHERE id = ?",
            (status, now, vocab_item_id),
        )
