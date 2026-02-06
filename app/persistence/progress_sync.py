"""
Progress sync: export and import learning progress as JSON.

Tables included in the progress bundle:
- vocab_items
- srs_state
- vocab_lookups
- jobs (schema v2+)
- job_segments (schema v2+)
- job_paragraphs (schema v2+)

The import uses overwrite semantics: all existing data in these tables
is replaced with the uploaded data in a single transaction.
"""

import json
from dataclasses import asdict
from datetime import datetime, timezone
from typing import Any

from app.persistence.db import db_conn
from app.persistence.models import ProgressBundle

# Current schema version for progress exports
# v1: vocab_items, srs_state, vocab_lookups
# v2: added jobs, job_segments, job_paragraphs
PROGRESS_SCHEMA_VERSION = 2


def export_progress() -> ProgressBundle:
    """
    Export learning progress as a ProgressBundle.

    Includes all vocab_items, srs_state, vocab_lookups, and job tables.
    """
    with db_conn() as conn:
        # Export vocab_items
        vocab_items = []
        rows = conn.execute(
            """
            SELECT id, headword, pinyin, english, status, created_at, updated_at
            FROM vocab_items
            ORDER BY created_at
            """
        ).fetchall()
        for row in rows:
            vocab_items.append(
                {
                    "id": row["id"],
                    "headword": row["headword"],
                    "pinyin": row["pinyin"],
                    "english": row["english"],
                    "status": row["status"],
                    "created_at": row["created_at"],
                    "updated_at": row["updated_at"],
                }
            )

        # Export srs_state
        srs_state = []
        rows = conn.execute(
            """
            SELECT vocab_item_id, due_at, interval_days, ease, reps, lapses, last_reviewed_at
            FROM srs_state
            """
        ).fetchall()
        for row in rows:
            srs_state.append(
                {
                    "vocab_item_id": row["vocab_item_id"],
                    "due_at": row["due_at"],
                    "interval_days": row["interval_days"],
                    "ease": row["ease"],
                    "reps": row["reps"],
                    "lapses": row["lapses"],
                    "last_reviewed_at": row["last_reviewed_at"],
                }
            )

        # Export vocab_lookups
        vocab_lookups = []
        rows = conn.execute(
            """
            SELECT id, vocab_item_id, looked_up_at
            FROM vocab_lookups
            ORDER BY looked_up_at
            """
        ).fetchall()
        for row in rows:
            vocab_lookups.append(
                {
                    "id": row["id"],
                    "vocab_item_id": row["vocab_item_id"],
                    "looked_up_at": row["looked_up_at"],
                }
            )

        # Export jobs
        jobs = []
        rows = conn.execute(
            """
            SELECT id, created_at, updated_at, status, job_type, source_type,
                   input_text, full_translation, error_message, metadata_json, text_id
            FROM jobs
            ORDER BY created_at
            """
        ).fetchall()
        for row in rows:
            jobs.append(
                {
                    "id": row["id"],
                    "created_at": row["created_at"],
                    "updated_at": row["updated_at"],
                    "status": row["status"],
                    "job_type": row["job_type"],
                    "source_type": row["source_type"],
                    "input_text": row["input_text"],
                    "full_translation": row["full_translation"],
                    "error_message": row["error_message"],
                    "metadata_json": row["metadata_json"],
                    "text_id": row["text_id"],
                }
            )

        # Export job_segments
        job_segments = []
        rows = conn.execute(
            """
            SELECT id, job_id, paragraph_idx, seg_idx, segment_text, pinyin, english, created_at
            FROM job_segments
            ORDER BY job_id, paragraph_idx, seg_idx
            """
        ).fetchall()
        for row in rows:
            job_segments.append(
                {
                    "id": row["id"],
                    "job_id": row["job_id"],
                    "paragraph_idx": row["paragraph_idx"],
                    "seg_idx": row["seg_idx"],
                    "segment_text": row["segment_text"],
                    "pinyin": row["pinyin"],
                    "english": row["english"],
                    "created_at": row["created_at"],
                }
            )

        # Export job_paragraphs
        job_paragraphs = []
        rows = conn.execute(
            """
            SELECT id, job_id, paragraph_idx, indent, separator
            FROM job_paragraphs
            ORDER BY job_id, paragraph_idx
            """
        ).fetchall()
        for row in rows:
            job_paragraphs.append(
                {
                    "id": row["id"],
                    "job_id": row["job_id"],
                    "paragraph_idx": row["paragraph_idx"],
                    "indent": row["indent"],
                    "separator": row["separator"],
                }
            )

    return ProgressBundle(
        schema_version=PROGRESS_SCHEMA_VERSION,
        exported_at=datetime.now(timezone.utc).isoformat(),
        vocab_items=vocab_items,
        srs_state=srs_state,
        vocab_lookups=vocab_lookups,
        jobs=jobs,
        job_segments=job_segments,
        job_paragraphs=job_paragraphs,
    )


def export_progress_json() -> str:
    """Export progress as a JSON string."""
    bundle = export_progress()
    return json.dumps(asdict(bundle), ensure_ascii=False, indent=2)


class ImportError(Exception):
    """Raised when progress import fails validation."""

    pass


def validate_progress_bundle(data: dict[str, Any]) -> ProgressBundle:
    """
    Validate and parse a progress bundle from JSON data.

    Raises ImportError if validation fails.
    Backward compatible: v1 bundles without job tables are accepted.
    """
    # Check schema version
    schema_version = data.get("schema_version")
    if schema_version is None:
        raise ImportError("Missing 'schema_version' field")
    if not isinstance(schema_version, int):
        raise ImportError("'schema_version' must be an integer")
    if schema_version > PROGRESS_SCHEMA_VERSION:
        raise ImportError(
            f"Unsupported schema version {schema_version}. "
            f"Maximum supported: {PROGRESS_SCHEMA_VERSION}"
        )

    # Check required tables (v1)
    vocab_items = data.get("vocab_items")
    if vocab_items is None:
        raise ImportError("Missing 'vocab_items' field")
    if not isinstance(vocab_items, list):
        raise ImportError("'vocab_items' must be a list")

    srs_state = data.get("srs_state")
    if srs_state is None:
        raise ImportError("Missing 'srs_state' field")
    if not isinstance(srs_state, list):
        raise ImportError("'srs_state' must be a list")

    vocab_lookups = data.get("vocab_lookups")
    if vocab_lookups is None:
        raise ImportError("Missing 'vocab_lookups' field")
    if not isinstance(vocab_lookups, list):
        raise ImportError("'vocab_lookups' must be a list")

    # Validate vocab_items rows
    required_vocab_fields = {
        "id",
        "headword",
        "pinyin",
        "english",
        "status",
        "created_at",
        "updated_at",
    }
    for i, item in enumerate(vocab_items):
        if not isinstance(item, dict):
            raise ImportError(f"vocab_items[{i}] must be an object")
        missing = required_vocab_fields - set(item.keys())
        if missing:
            raise ImportError(f"vocab_items[{i}] missing fields: {missing}")

    # Validate srs_state rows
    required_srs_fields = {
        "vocab_item_id",
        "due_at",
        "interval_days",
        "ease",
        "reps",
        "lapses",
        "last_reviewed_at",
    }
    for i, item in enumerate(srs_state):
        if not isinstance(item, dict):
            raise ImportError(f"srs_state[{i}] must be an object")
        missing = required_srs_fields - set(item.keys())
        if missing:
            raise ImportError(f"srs_state[{i}] missing fields: {missing}")

    # Validate vocab_lookups rows
    required_lookup_fields = {"id", "vocab_item_id", "looked_up_at"}
    for i, item in enumerate(vocab_lookups):
        if not isinstance(item, dict):
            raise ImportError(f"vocab_lookups[{i}] must be an object")
        missing = required_lookup_fields - set(item.keys())
        if missing:
            raise ImportError(f"vocab_lookups[{i}] missing fields: {missing}")

    # Optional job tables (v2+)
    jobs = data.get("jobs")
    job_segments = data.get("job_segments")
    job_paragraphs = data.get("job_paragraphs")

    # Validate jobs if present
    if jobs is not None:
        if not isinstance(jobs, list):
            raise ImportError("'jobs' must be a list")
        required_job_fields = {
            "id",
            "created_at",
            "updated_at",
            "status",
            "job_type",
            "source_type",
            "input_text",
            "metadata_json",
        }
        for i, item in enumerate(jobs):
            if not isinstance(item, dict):
                raise ImportError(f"jobs[{i}] must be an object")
            missing = required_job_fields - set(item.keys())
            if missing:
                raise ImportError(f"jobs[{i}] missing fields: {missing}")

    # Validate job_segments if present
    if job_segments is not None:
        if not isinstance(job_segments, list):
            raise ImportError("'job_segments' must be a list")
        required_seg_fields = {
            "id",
            "job_id",
            "paragraph_idx",
            "seg_idx",
            "segment_text",
            "pinyin",
            "english",
            "created_at",
        }
        for i, item in enumerate(job_segments):
            if not isinstance(item, dict):
                raise ImportError(f"job_segments[{i}] must be an object")
            missing = required_seg_fields - set(item.keys())
            if missing:
                raise ImportError(f"job_segments[{i}] missing fields: {missing}")

    # Validate job_paragraphs if present
    if job_paragraphs is not None:
        if not isinstance(job_paragraphs, list):
            raise ImportError("'job_paragraphs' must be a list")
        required_para_fields = {"id", "job_id", "paragraph_idx", "indent", "separator"}
        for i, item in enumerate(job_paragraphs):
            if not isinstance(item, dict):
                raise ImportError(f"job_paragraphs[{i}] must be an object")
            missing = required_para_fields - set(item.keys())
            if missing:
                raise ImportError(f"job_paragraphs[{i}] missing fields: {missing}")

    return ProgressBundle(
        schema_version=schema_version,
        exported_at=data.get("exported_at", ""),
        vocab_items=vocab_items,
        srs_state=srs_state,
        vocab_lookups=vocab_lookups,
        jobs=jobs,
        job_segments=job_segments,
        job_paragraphs=job_paragraphs,
    )


def import_progress(bundle: ProgressBundle) -> dict[str, int]:
    """
    Import a progress bundle, replacing existing data.

    Uses a single transaction to ensure atomicity.
    Deletes existing rows first (in FK-safe order), then inserts new rows.

    Returns counts of imported rows per table.
    """
    with db_conn() as conn:
        # Delete existing data in FK-safe order
        # Job tables first (children before parents)
        conn.execute("DELETE FROM job_segments")
        conn.execute("DELETE FROM job_paragraphs")
        conn.execute("DELETE FROM jobs")
        # Then vocab tables
        conn.execute("DELETE FROM vocab_lookups")
        conn.execute("DELETE FROM srs_state")
        conn.execute("DELETE FROM vocab_items")

        # Insert vocab_items
        for item in bundle.vocab_items:
            conn.execute(
                """
                INSERT INTO vocab_items (id, headword, pinyin, english, status, created_at, updated_at)
                VALUES (?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    item["id"],
                    item["headword"],
                    item["pinyin"],
                    item["english"],
                    item["status"],
                    item["created_at"],
                    item["updated_at"],
                ),
            )

        # Insert srs_state
        for item in bundle.srs_state:
            conn.execute(
                """
                INSERT INTO srs_state (vocab_item_id, due_at, interval_days, ease, reps, lapses, last_reviewed_at)
                VALUES (?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    item["vocab_item_id"],
                    item["due_at"],
                    item["interval_days"],
                    item["ease"],
                    item["reps"],
                    item["lapses"],
                    item["last_reviewed_at"],
                ),
            )

        # Insert vocab_lookups
        for item in bundle.vocab_lookups:
            conn.execute(
                """
                INSERT INTO vocab_lookups (id, vocab_item_id, looked_up_at)
                VALUES (?, ?, ?)
                """,
                (
                    item["id"],
                    item["vocab_item_id"],
                    item["looked_up_at"],
                ),
            )

        # Insert jobs (if present)
        jobs_count = 0
        if bundle.jobs:
            for item in bundle.jobs:
                conn.execute(
                    """
                    INSERT INTO jobs (id, created_at, updated_at, status, job_type, source_type,
                                      input_text, full_translation, error_message, metadata_json, text_id)
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                    """,
                    (
                        item["id"],
                        item["created_at"],
                        item["updated_at"],
                        item["status"],
                        item["job_type"],
                        item["source_type"],
                        item["input_text"],
                        item.get("full_translation"),
                        item.get("error_message"),
                        item["metadata_json"],
                        item.get("text_id"),
                    ),
                )
            jobs_count = len(bundle.jobs)

        # Insert job_segments (if present)
        job_segments_count = 0
        if bundle.job_segments:
            for item in bundle.job_segments:
                conn.execute(
                    """
                    INSERT INTO job_segments (id, job_id, paragraph_idx, seg_idx,
                                              segment_text, pinyin, english, created_at)
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                    """,
                    (
                        item["id"],
                        item["job_id"],
                        item["paragraph_idx"],
                        item["seg_idx"],
                        item["segment_text"],
                        item["pinyin"],
                        item["english"],
                        item["created_at"],
                    ),
                )
            job_segments_count = len(bundle.job_segments)

        # Insert job_paragraphs (if present)
        job_paragraphs_count = 0
        if bundle.job_paragraphs:
            for item in bundle.job_paragraphs:
                conn.execute(
                    """
                    INSERT INTO job_paragraphs (id, job_id, paragraph_idx, indent, separator)
                    VALUES (?, ?, ?, ?, ?)
                    """,
                    (
                        item["id"],
                        item["job_id"],
                        item["paragraph_idx"],
                        item["indent"],
                        item["separator"],
                    ),
                )
            job_paragraphs_count = len(bundle.job_paragraphs)

        # Verify foreign key integrity
        fk_errors = conn.execute("PRAGMA foreign_key_check").fetchall()
        if fk_errors:
            raise ImportError(
                f"Foreign key violations detected: {len(fk_errors)} errors"
            )

    return {
        "vocab_items": len(bundle.vocab_items),
        "srs_state": len(bundle.srs_state),
        "vocab_lookups": len(bundle.vocab_lookups),
        "jobs": jobs_count,
        "job_segments": job_segments_count,
        "job_paragraphs": job_paragraphs_count,
    }


def import_progress_json(json_str: str) -> dict[str, int]:
    """
    Import progress from a JSON string.

    Validates and imports the data.
    Returns counts of imported rows per table.
    """
    try:
        data = json.loads(json_str)
    except json.JSONDecodeError as e:
        raise ImportError(f"Invalid JSON: {e}") from e

    bundle = validate_progress_bundle(data)
    return import_progress(bundle)
