"""
CRUD operations for translations.
"""

import json
import uuid
from datetime import datetime, timezone
from typing import Any

from app.persistence.db import db_conn
from app.persistence.models import (
    TranslationRecord,
    TranslationSegmentRecord,
    TranslationWithResults,
)


def _utc_now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def create_translation(
    *,
    input_text: str,
    source_type: str = "text",
    translation_type: str = "translation",
    metadata: dict[str, Any] | None = None,
) -> str:
    """Create a new pending translation. Returns translation_id."""
    translation_id = uuid.uuid4().hex
    now = _utc_now_iso()
    metadata_json = json.dumps(metadata or {}, ensure_ascii=False)

    with db_conn() as conn:
        conn.execute(
            """
            INSERT INTO translations (id, created_at, updated_at, status, translation_type, source_type,
                              input_text, metadata_json)
            VALUES (?, ?, ?, 'pending', ?, ?, ?, ?)
            """,
            (translation_id, now, now, translation_type, source_type, input_text, metadata_json),
        )

    return translation_id


def get_translation(translation_id: str) -> TranslationRecord | None:
    """Get translation record by ID."""
    with db_conn() as conn:
        row = conn.execute("SELECT * FROM translations WHERE id = ?", (translation_id,)).fetchone()
        if row is None:
            return None
        return _row_to_translation_record(row)


def _row_to_translation_record(row: Any) -> TranslationRecord:
    """Convert a database row to TranslationRecord."""
    return TranslationRecord(
        id=row["id"],
        created_at=row["created_at"],
        updated_at=row["updated_at"],
        status=row["status"],
        translation_type=row["translation_type"],
        source_type=row["source_type"],
        input_text=row["input_text"],
        full_translation=row["full_translation"],
        error_message=row["error_message"],
        metadata=json.loads(row["metadata_json"] or "{}"),
        text_id=row["text_id"],
    )


def get_translation_with_results(translation_id: str) -> TranslationWithResults | None:
    """Get translation with all segment results, structured by paragraph."""
    translation = get_translation(translation_id)
    if translation is None:
        return None

    with db_conn() as conn:
        # Get paragraphs
        para_rows = conn.execute(
            """
            SELECT * FROM translation_paragraphs
            WHERE translation_id = ?
            ORDER BY paragraph_idx
            """,
            (translation_id,),
        ).fetchall()

        # Get segments
        seg_rows = conn.execute(
            """
            SELECT * FROM translation_segments
            WHERE translation_id = ?
            ORDER BY paragraph_idx, seg_idx
            """,
            (translation_id,),
        ).fetchall()

    # Build paragraph structure with segments
    paragraphs: list[dict[str, Any]] = []
    para_map: dict[int, dict[str, Any]] = {}

    for row in para_rows:
        para = {
            "paragraph_idx": row["paragraph_idx"],
            "indent": row["indent"],
            "separator": row["separator"],
            "translations": [],
        }
        para_map[row["paragraph_idx"]] = para
        paragraphs.append(para)

    for row in seg_rows:
        para_idx = row["paragraph_idx"]
        if para_idx in para_map:
            para_map[para_idx]["translations"].append(
                {
                    "segment": row["segment_text"],
                    "pinyin": row["pinyin"],
                    "english": row["english"],
                }
            )

    return TranslationWithResults(translation=translation, paragraphs=paragraphs)


def update_translation_status(
    translation_id: str,
    status: str,
    error_message: str | None = None,
) -> None:
    """Update translation status."""
    if status not in {"pending", "processing", "completed", "failed"}:
        raise ValueError(f"Invalid translation status: {status}")

    now = _utc_now_iso()
    with db_conn() as conn:
        conn.execute(
            """
            UPDATE translations
            SET status = ?, error_message = ?, updated_at = ?
            WHERE id = ?
            """,
            (status, error_message, now, translation_id),
        )


def complete_translation(
    translation_id: str, full_translation: str, text_id: str | None = None
) -> None:
    """Mark translation as completed with full translation."""
    now = _utc_now_iso()
    with db_conn() as conn:
        conn.execute(
            """
            UPDATE translations
            SET status = 'completed', full_translation = ?, text_id = ?, updated_at = ?
            WHERE id = ?
            """,
            (full_translation, text_id, now, translation_id),
        )


def fail_translation(translation_id: str, error_message: str) -> None:
    """Mark translation as failed with error message."""
    update_translation_status(translation_id, "failed", error_message)


def save_translation_paragraph(
    translation_id: str,
    paragraph_idx: int,
    indent: str,
    separator: str,
) -> str:
    """Save paragraph metadata for a translation. Returns paragraph record ID."""
    para_id = uuid.uuid4().hex
    with db_conn() as conn:
        conn.execute(
            """
            INSERT INTO translation_paragraphs (id, translation_id, paragraph_idx, indent, separator)
            VALUES (?, ?, ?, ?, ?)
            """,
            (para_id, translation_id, paragraph_idx, indent, separator),
        )
    return para_id


def save_translation_segment(
    translation_id: str,
    paragraph_idx: int,
    seg_idx: int,
    segment_text: str,
    pinyin: str,
    english: str,
) -> str:
    """Save a single segment translation result. Returns segment record ID."""
    seg_id = uuid.uuid4().hex
    now = _utc_now_iso()
    with db_conn() as conn:
        conn.execute(
            """
            INSERT INTO translation_segments (id, translation_id, paragraph_idx, seg_idx,
                                      segment_text, pinyin, english, created_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                seg_id,
                translation_id,
                paragraph_idx,
                seg_idx,
                segment_text,
                pinyin,
                english,
                now,
            ),
        )
    return seg_id


def update_translation_segments(
    translation_id: str,
    paragraph_idx: int,
    segments: list[dict[str, str]],
) -> None:
    """
    Replace all segments for a paragraph with new segments.
    Used after split/join operations.

    segments: list of {segment_text, pinyin, english}
    """
    now = _utc_now_iso()
    with db_conn() as conn:
        # Delete existing segments for this paragraph
        conn.execute(
            "DELETE FROM translation_segments WHERE translation_id = ? AND paragraph_idx = ?",
            (translation_id, paragraph_idx),
        )
        # Insert new segments
        for idx, seg in enumerate(segments):
            seg_id = uuid.uuid4().hex
            conn.execute(
                """
                INSERT INTO translation_segments (id, translation_id, paragraph_idx, seg_idx,
                                          segment_text, pinyin, english, created_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    seg_id,
                    translation_id,
                    paragraph_idx,
                    idx,
                    seg["segment_text"],
                    seg["pinyin"],
                    seg["english"],
                    now,
                ),
            )


def list_translations(
    *,
    limit: int = 20,
    offset: int = 0,
    status: str | None = None,
) -> tuple[list[TranslationRecord], int]:
    """
    List translations, most recent first.
    Returns (translations, total_count).
    """
    with db_conn() as conn:
        # Build query with optional status filter
        where_clause = "WHERE status = ?" if status else ""
        params: tuple[Any, ...] = (status,) if status else ()

        # Get total count
        count_query = f"SELECT COUNT(*) as cnt FROM translations {where_clause}"
        total = conn.execute(count_query, params).fetchone()["cnt"]

        # Get paginated results
        query = f"""
            SELECT * FROM translations
            {where_clause}
            ORDER BY created_at DESC
            LIMIT ? OFFSET ?
        """
        if status:
            rows = conn.execute(query, (status, limit, offset)).fetchall()
        else:
            rows = conn.execute(query, (limit, offset)).fetchall()

    translations = [_row_to_translation_record(row) for row in rows]
    return translations, total


def get_translation_segment_count(translation_id: str) -> tuple[int, int]:
    """
    Get segment counts for a translation.
    Returns (completed_segments, total_segments).
    """
    with db_conn() as conn:
        row = conn.execute(
            """
            SELECT
                COUNT(*) as total,
                SUM(CASE WHEN pinyin != '' OR english != '' THEN 1 ELSE 0 END) as completed
            FROM translation_segments
            WHERE translation_id = ?
            """,
            (translation_id,),
        ).fetchone()

    return row["completed"] or 0, row["total"] or 0


def delete_translation(translation_id: str) -> bool:
    """Delete a translation and its results. Returns True if translation existed."""
    with db_conn() as conn:
        # CASCADE will handle translation_segments and translation_paragraphs
        cursor = conn.execute("DELETE FROM translations WHERE id = ?", (translation_id,))
        return cursor.rowcount > 0


def get_translation_segments(translation_id: str) -> list[TranslationSegmentRecord]:
    """Get all segments for a translation, ordered by paragraph and segment index."""
    with db_conn() as conn:
        rows = conn.execute(
            """
            SELECT * FROM translation_segments
            WHERE translation_id = ?
            ORDER BY paragraph_idx, seg_idx
            """,
            (translation_id,),
        ).fetchall()

    return [
        TranslationSegmentRecord(
            id=row["id"],
            translation_id=row["translation_id"],
            paragraph_idx=row["paragraph_idx"],
            seg_idx=row["seg_idx"],
            segment_text=row["segment_text"],
            pinyin=row["pinyin"],
            english=row["english"],
            created_at=row["created_at"],
        )
        for row in rows
    ]
