"""
User profile CRUD operations.
"""

from datetime import datetime, timezone

from app.persistence.db import db_conn
from app.persistence.models import UserProfile


def _utc_now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def get_user_profile() -> UserProfile | None:
    """Get the user profile (single row)."""
    with db_conn() as conn:
        row = conn.execute("SELECT * FROM user_profile WHERE id = 1").fetchone()
        if row is None:
            return None
        return UserProfile(
            name=row["name"],
            email=row["email"],
            language=row["language"],
            created_at=row["created_at"],
            updated_at=row["updated_at"],
        )


def upsert_user_profile(*, name: str, email: str, language: str) -> UserProfile:
    """
    Create or update the user profile.
    Returns the updated profile.
    """
    now = _utc_now_iso()
    with db_conn() as conn:
        # Try to update first
        result = conn.execute(
            """
            UPDATE user_profile
            SET name = ?, email = ?, language = ?, updated_at = ?
            WHERE id = 1
            """,
            (name, email, language, now),
        )
        if result.rowcount == 0:
            # Insert if no row exists
            conn.execute(
                """
                INSERT INTO user_profile (id, name, email, language, created_at, updated_at)
                VALUES (1, ?, ?, ?, ?, ?)
                """,
                (name, email, language, now, now),
            )
    return UserProfile(
        name=name,
        email=email,
        language=language,
        created_at=now,
        updated_at=now,
    )


def count_known_vocab() -> int:
    """Count vocabulary items with status='known'."""
    with db_conn() as conn:
        row = conn.execute(
            "SELECT COUNT(*) as count FROM vocab_items WHERE status = 'known'"
        ).fetchone()
        return row["count"] if row else 0


def count_learning_vocab() -> int:
    """Count vocabulary items with status='learning'."""
    with db_conn() as conn:
        row = conn.execute(
            "SELECT COUNT(*) as count FROM vocab_items WHERE status = 'learning'"
        ).fetchone()
        return row["count"] if row else 0


def count_total_vocab() -> int:
    """Count all vocabulary items."""
    with db_conn() as conn:
        row = conn.execute("SELECT COUNT(*) as count FROM vocab_items").fetchone()
        return row["count"] if row else 0
