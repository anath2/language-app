"""
CRUD operations for translation jobs.
"""

import json
import uuid
from datetime import datetime, timezone
from typing import Any

from app.persistence.db import db_conn
from app.persistence.models import (
    JobRecord,
    JobSegmentRecord,
    JobWithResults,
)


def _utc_now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def create_job(
    *,
    input_text: str,
    source_type: str = "text",
    job_type: str = "translation",
    metadata: dict[str, Any] | None = None,
) -> str:
    """Create a new pending job. Returns job_id."""
    job_id = uuid.uuid4().hex
    now = _utc_now_iso()
    metadata_json = json.dumps(metadata or {}, ensure_ascii=False)

    with db_conn() as conn:
        conn.execute(
            """
            INSERT INTO jobs (id, created_at, updated_at, status, job_type, source_type,
                              input_text, metadata_json)
            VALUES (?, ?, ?, 'pending', ?, ?, ?, ?)
            """,
            (job_id, now, now, job_type, source_type, input_text, metadata_json),
        )

    return job_id


def get_job(job_id: str) -> JobRecord | None:
    """Get job record by ID."""
    with db_conn() as conn:
        row = conn.execute("SELECT * FROM jobs WHERE id = ?", (job_id,)).fetchone()
        if row is None:
            return None
        return _row_to_job_record(row)


def _row_to_job_record(row: Any) -> JobRecord:
    """Convert a database row to JobRecord."""
    return JobRecord(
        id=row["id"],
        created_at=row["created_at"],
        updated_at=row["updated_at"],
        status=row["status"],
        job_type=row["job_type"],
        source_type=row["source_type"],
        input_text=row["input_text"],
        full_translation=row["full_translation"],
        error_message=row["error_message"],
        metadata=json.loads(row["metadata_json"] or "{}"),
        text_id=row["text_id"],
    )


def get_job_with_results(job_id: str) -> JobWithResults | None:
    """Get job with all segment results, structured by paragraph."""
    job = get_job(job_id)
    if job is None:
        return None

    with db_conn() as conn:
        # Get paragraphs
        para_rows = conn.execute(
            """
            SELECT * FROM job_paragraphs
            WHERE job_id = ?
            ORDER BY paragraph_idx
            """,
            (job_id,),
        ).fetchall()

        # Get segments
        seg_rows = conn.execute(
            """
            SELECT * FROM job_segments
            WHERE job_id = ?
            ORDER BY paragraph_idx, seg_idx
            """,
            (job_id,),
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

    return JobWithResults(job=job, paragraphs=paragraphs)


def update_job_status(
    job_id: str,
    status: str,
    error_message: str | None = None,
) -> None:
    """Update job status."""
    if status not in {"pending", "processing", "completed", "failed"}:
        raise ValueError(f"Invalid job status: {status}")

    now = _utc_now_iso()
    with db_conn() as conn:
        conn.execute(
            """
            UPDATE jobs
            SET status = ?, error_message = ?, updated_at = ?
            WHERE id = ?
            """,
            (status, error_message, now, job_id),
        )


def complete_job(
    job_id: str, full_translation: str, text_id: str | None = None
) -> None:
    """Mark job as completed with full translation."""
    now = _utc_now_iso()
    with db_conn() as conn:
        conn.execute(
            """
            UPDATE jobs
            SET status = 'completed', full_translation = ?, text_id = ?, updated_at = ?
            WHERE id = ?
            """,
            (full_translation, text_id, now, job_id),
        )


def fail_job(job_id: str, error_message: str) -> None:
    """Mark job as failed with error message."""
    update_job_status(job_id, "failed", error_message)


def save_job_paragraph(
    job_id: str,
    paragraph_idx: int,
    indent: str,
    separator: str,
) -> str:
    """Save paragraph metadata for a job. Returns paragraph record ID."""
    para_id = uuid.uuid4().hex
    with db_conn() as conn:
        conn.execute(
            """
            INSERT INTO job_paragraphs (id, job_id, paragraph_idx, indent, separator)
            VALUES (?, ?, ?, ?, ?)
            """,
            (para_id, job_id, paragraph_idx, indent, separator),
        )
    return para_id


def save_job_segment(
    job_id: str,
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
            INSERT INTO job_segments (id, job_id, paragraph_idx, seg_idx,
                                      segment_text, pinyin, english, created_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                seg_id,
                job_id,
                paragraph_idx,
                seg_idx,
                segment_text,
                pinyin,
                english,
                now,
            ),
        )
    return seg_id


def update_job_segments(
    job_id: str,
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
            "DELETE FROM job_segments WHERE job_id = ? AND paragraph_idx = ?",
            (job_id, paragraph_idx),
        )
        # Insert new segments
        for idx, seg in enumerate(segments):
            seg_id = uuid.uuid4().hex
            conn.execute(
                """
                INSERT INTO job_segments (id, job_id, paragraph_idx, seg_idx,
                                          segment_text, pinyin, english, created_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    seg_id,
                    job_id,
                    paragraph_idx,
                    idx,
                    seg["segment_text"],
                    seg["pinyin"],
                    seg["english"],
                    now,
                ),
            )


def list_jobs(
    *,
    limit: int = 20,
    offset: int = 0,
    status: str | None = None,
) -> tuple[list[JobRecord], int]:
    """
    List jobs, most recent first.
    Returns (jobs, total_count).
    """
    with db_conn() as conn:
        # Build query with optional status filter
        where_clause = "WHERE status = ?" if status else ""
        params: tuple[Any, ...] = (status,) if status else ()

        # Get total count
        count_query = f"SELECT COUNT(*) as cnt FROM jobs {where_clause}"
        total = conn.execute(count_query, params).fetchone()["cnt"]

        # Get paginated results
        query = f"""
            SELECT * FROM jobs
            {where_clause}
            ORDER BY created_at DESC
            LIMIT ? OFFSET ?
        """
        if status:
            rows = conn.execute(query, (status, limit, offset)).fetchall()
        else:
            rows = conn.execute(query, (limit, offset)).fetchall()

    jobs = [_row_to_job_record(row) for row in rows]
    return jobs, total


def get_job_segment_count(job_id: str) -> tuple[int, int]:
    """
    Get segment counts for a job.
    Returns (completed_segments, total_segments).
    """
    with db_conn() as conn:
        row = conn.execute(
            """
            SELECT
                COUNT(*) as total,
                SUM(CASE WHEN pinyin != '' OR english != '' THEN 1 ELSE 0 END) as completed
            FROM job_segments
            WHERE job_id = ?
            """,
            (job_id,),
        ).fetchone()

    return row["completed"] or 0, row["total"] or 0


def delete_job(job_id: str) -> bool:
    """Delete a job and its results. Returns True if job existed."""
    with db_conn() as conn:
        # CASCADE will handle job_segments and job_paragraphs
        cursor = conn.execute("DELETE FROM jobs WHERE id = ?", (job_id,))
        return cursor.rowcount > 0


def get_job_segments(job_id: str) -> list[JobSegmentRecord]:
    """Get all segments for a job, ordered by paragraph and segment index."""
    with db_conn() as conn:
        rows = conn.execute(
            """
            SELECT * FROM job_segments
            WHERE job_id = ?
            ORDER BY paragraph_idx, seg_idx
            """,
            (job_id,),
        ).fetchall()

    return [
        JobSegmentRecord(
            id=row["id"],
            job_id=row["job_id"],
            paragraph_idx=row["paragraph_idx"],
            seg_idx=row["seg_idx"],
            segment_text=row["segment_text"],
            pinyin=row["pinyin"],
            english=row["english"],
            created_at=row["created_at"],
        )
        for row in rows
    ]
