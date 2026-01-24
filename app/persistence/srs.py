"""
Spaced Repetition System (SRS) algorithm implementation.

This module handles:
- SM-2 algorithm for active review
- Passive lookup tracking with struggle detection
- Opacity calculation for UI rendering
"""

import sqlite3
import uuid
from datetime import datetime, timedelta, timezone

from app.persistence.db import db_conn
from app.persistence.models import ReviewCard, SRSState, VocabSRSInfo

# =============================================================================
# SRS Constants
# =============================================================================

DECAY_DAYS = 30.0  # Days until opacity reaches 0
STRUGGLE_THRESHOLD = 3  # Lookups in 7 days to be "struggling"
STRUGGLE_WINDOW_DAYS = 7
STRUGGLE_OPACITY_BOOST = 0.3  # Minimum opacity for struggling words
MIN_EASE = 1.3
DEFAULT_EASE = 2.5
GRADUATING_INTERVAL = 1.0  # Days


def _utc_now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def initialize_srs_state(vocab_item_id: str) -> None:
    """
    Create or reset SRS state for a vocab item.
    Sets last_reviewed_at to NOW so the word starts with full opacity.
    """
    now = _utc_now_iso()
    with db_conn() as conn:
        conn.execute(
            """
            INSERT INTO srs_state (vocab_item_id, due_at, interval_days, ease, reps, lapses, last_reviewed_at)
            VALUES (?, ?, 0, ?, 0, 0, ?)
            ON CONFLICT(vocab_item_id) DO UPDATE SET
                last_reviewed_at = excluded.last_reviewed_at
            """,
            (vocab_item_id, now, DEFAULT_EASE, now),
        )


def _get_srs_state(conn: sqlite3.Connection, vocab_item_id: str) -> SRSState | None:
    """Get SRS state for a vocab item (internal helper)."""
    row = conn.execute(
        "SELECT * FROM srs_state WHERE vocab_item_id = ?", (vocab_item_id,)
    ).fetchone()
    if row is None:
        return None
    return SRSState(
        vocab_item_id=row["vocab_item_id"],
        due_at=row["due_at"],
        interval_days=row["interval_days"],
        ease=row["ease"],
        reps=row["reps"],
        lapses=row["lapses"],
        last_reviewed_at=row["last_reviewed_at"],
    )


def _count_recent_lookups(conn: sqlite3.Connection, vocab_item_id: str) -> int:
    """Count lookups in the last 7 days for struggle detection."""
    cutoff = (datetime.now(timezone.utc) - timedelta(days=STRUGGLE_WINDOW_DAYS)).isoformat()
    row = conn.execute(
        """
        SELECT COUNT(*) as cnt FROM vocab_lookups
        WHERE vocab_item_id = ? AND looked_up_at >= ?
        """,
        (vocab_item_id, cutoff),
    ).fetchone()
    return row["cnt"] if row else 0


def is_struggling(vocab_item_id: str) -> bool:
    """Check if a vocab item is being looked up frequently (struggling)."""
    with db_conn() as conn:
        count = _count_recent_lookups(conn, vocab_item_id)
        return count >= STRUGGLE_THRESHOLD


def compute_opacity(last_looked_up_at: str | None, is_struggling: bool) -> float:
    """
    Compute opacity based on recency of last lookup.

    - Just looked up: 1.0 (full opacity)
    - 30+ days ago: 0.0 (transparent)
    - Struggling words have minimum opacity of STRUGGLE_OPACITY_BOOST
    """
    if last_looked_up_at is None:
        # Never looked up - treat as very old (transparent)
        base_opacity = 0.0
    else:
        try:
            last_dt = datetime.fromisoformat(last_looked_up_at)
            now = datetime.now(timezone.utc)
            days_since = (now - last_dt).total_seconds() / 86400
            base_opacity = max(0.0, 1.0 - (days_since / DECAY_DAYS))
        except (ValueError, TypeError):
            base_opacity = 1.0  # Default to visible on parse error

    if is_struggling:
        return max(base_opacity, STRUGGLE_OPACITY_BOOST)
    return base_opacity


def record_lookup(vocab_item_id: str) -> VocabSRSInfo | None:
    """
    Record a passive lookup event for a vocab item.
    Updates last_reviewed_at and adds to lookup history.
    Returns the updated SRS info with opacity.
    """
    now = _utc_now_iso()
    lookup_id = uuid.uuid4().hex

    with db_conn() as conn:
        # Get vocab item info
        vocab_row = conn.execute(
            "SELECT id, headword, pinyin, english, status FROM vocab_items WHERE id = ?",
            (vocab_item_id,),
        ).fetchone()
        if vocab_row is None:
            return None

        # Ensure SRS state exists
        srs_state = _get_srs_state(conn, vocab_item_id)
        if srs_state is None:
            conn.execute(
                """
                INSERT INTO srs_state (vocab_item_id, due_at, interval_days, ease, reps, lapses, last_reviewed_at)
                VALUES (?, ?, 0, ?, 0, 0, ?)
                """,
                (vocab_item_id, now, DEFAULT_EASE, now),
            )
        else:
            # Update last_reviewed_at
            conn.execute(
                "UPDATE srs_state SET last_reviewed_at = ? WHERE vocab_item_id = ?",
                (now, vocab_item_id),
            )

        # Record the lookup
        conn.execute(
            "INSERT INTO vocab_lookups (id, vocab_item_id, looked_up_at) VALUES (?, ?, ?)",
            (lookup_id, vocab_item_id, now),
        )

        # Calculate struggling status
        struggling = _count_recent_lookups(conn, vocab_item_id) >= STRUGGLE_THRESHOLD
        opacity = compute_opacity(now, struggling)

        return VocabSRSInfo(
            vocab_item_id=vocab_item_id,
            headword=vocab_row["headword"],
            pinyin=vocab_row["pinyin"],
            english=vocab_row["english"],
            opacity=opacity,
            is_struggling=struggling,
            status=vocab_row["status"],
        )


def get_vocab_srs_info(headwords: list[str]) -> list[VocabSRSInfo]:
    """
    Get SRS info for a list of headwords (for opacity rendering).
    Returns info for words that have been saved (exist in vocab_items).
    """
    # Import here to avoid circular dependency
    from app.utils import should_skip_segment

    # Filter out punctuation-only headwords
    headwords = [h for h in headwords if not should_skip_segment(h)]
    if not headwords:
        return []

    placeholders = ",".join("?" * len(headwords))
    results = []

    with db_conn() as conn:
        rows = conn.execute(
            f"""
            SELECT vi.id, vi.headword, vi.pinyin, vi.english, vi.status, ss.last_reviewed_at
            FROM vocab_items vi
            LEFT JOIN srs_state ss ON vi.id = ss.vocab_item_id
            WHERE vi.headword IN ({placeholders})
            """,
            headwords,
        ).fetchall()

        for row in rows:
            vocab_item_id = row["id"]
            last_reviewed_at = row["last_reviewed_at"]
            status = row["status"]
            struggling = _count_recent_lookups(conn, vocab_item_id) >= STRUGGLE_THRESHOLD
            opacity = compute_opacity(last_reviewed_at, struggling)

            results.append(
                VocabSRSInfo(
                    vocab_item_id=vocab_item_id,
                    headword=row["headword"],
                    pinyin=row["pinyin"],
                    english=row["english"],
                    opacity=opacity,
                    is_struggling=struggling,
                    status=status,
                )
            )

    return results


def record_review_grade(vocab_item_id: str, grade: int) -> SRSState | None:
    """
    Apply SM-2 algorithm for explicit grading in active review.

    Grade: 0=Again, 1=Hard, 2=Good

    Returns updated SRS state.
    """
    if grade not in (0, 1, 2):
        raise ValueError("Grade must be 0, 1, or 2")

    now = _utc_now_iso()

    with db_conn() as conn:
        state = _get_srs_state(conn, vocab_item_id)
        if state is None:
            # Initialize if missing
            conn.execute(
                """
                INSERT INTO srs_state (vocab_item_id, due_at, interval_days, ease, reps, lapses, last_reviewed_at)
                VALUES (?, ?, 0, ?, 0, 0, ?)
                """,
                (vocab_item_id, now, DEFAULT_EASE, now),
            )
            state = SRSState(
                vocab_item_id=vocab_item_id,
                due_at=now,
                interval_days=0,
                ease=DEFAULT_EASE,
                reps=0,
                lapses=0,
                last_reviewed_at=now,
            )

        # Apply SM-2 algorithm
        current_interval = state.interval_days
        ease = state.ease
        reps = state.reps
        lapses = state.lapses

        if grade == 0:  # Again
            new_interval = 0.0  # Due immediately (or very soon)
            new_ease = max(MIN_EASE, ease - 0.2)
            new_reps = 0
            new_lapses = lapses + 1
        elif grade == 1:  # Hard
            if reps == 0:
                new_interval = 0.5  # 12 hours
            else:
                new_interval = current_interval * 1.2
            new_ease = max(MIN_EASE, ease - 0.15)
            new_reps = reps + 1
            new_lapses = lapses
        else:  # Good (grade == 2)
            if reps == 0:
                new_interval = GRADUATING_INTERVAL
            elif reps == 1:
                new_interval = 6.0
            else:
                new_interval = current_interval * ease
            new_ease = ease  # No change for Good
            new_reps = reps + 1
            new_lapses = lapses

        # Calculate next due date
        due_dt = datetime.now(timezone.utc) + timedelta(days=new_interval)
        new_due_at = due_dt.isoformat()

        # Update database
        conn.execute(
            """
            UPDATE srs_state
            SET due_at = ?, interval_days = ?, ease = ?, reps = ?, lapses = ?, last_reviewed_at = ?
            WHERE vocab_item_id = ?
            """,
            (new_due_at, new_interval, new_ease, new_reps, new_lapses, now, vocab_item_id),
        )

        return SRSState(
            vocab_item_id=vocab_item_id,
            due_at=new_due_at,
            interval_days=new_interval,
            ease=new_ease,
            reps=new_reps,
            lapses=new_lapses,
            last_reviewed_at=now,
        )


def get_review_queue(limit: int = 10) -> list[ReviewCard]:
    """
    Get vocab items that are due for active review.
    Returns cards with headword, pinyin, english, and snippets.
    """
    now = _utc_now_iso()

    with db_conn() as conn:
        rows = conn.execute(
            """
            SELECT vi.id, vi.headword, vi.pinyin, vi.english
            FROM vocab_items vi
            JOIN srs_state ss ON vi.id = ss.vocab_item_id
            WHERE vi.status = 'learning'
              AND (ss.due_at IS NULL OR ss.due_at <= ?)
            ORDER BY ss.due_at ASC NULLS FIRST
            LIMIT ?
            """,
            (now, limit),
        ).fetchall()

        cards = []
        for row in rows:
            vocab_item_id = row["id"]

            # Get snippets from occurrences
            snippet_rows = conn.execute(
                """
                SELECT snippet FROM vocab_occurrences
                WHERE vocab_item_id = ? AND snippet != ''
                ORDER BY created_at DESC
                LIMIT 3
                """,
                (vocab_item_id,),
            ).fetchall()
            snippets = [s["snippet"] for s in snippet_rows]

            cards.append(
                ReviewCard(
                    vocab_item_id=vocab_item_id,
                    headword=row["headword"],
                    pinyin=row["pinyin"],
                    english=row["english"],
                    snippets=snippets,
                )
            )

        return cards


def get_due_count() -> int:
    """Get count of vocab items due for review."""
    now = _utc_now_iso()
    with db_conn() as conn:
        row = conn.execute(
            """
            SELECT COUNT(*) as cnt
            FROM vocab_items vi
            JOIN srs_state ss ON vi.id = ss.vocab_item_id
            WHERE vi.status = 'learning'
              AND (ss.due_at IS NULL OR ss.due_at <= ?)
            """,
            (now,),
        ).fetchone()
        return row["cnt"] if row else 0
