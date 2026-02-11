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
    return (
        Path(__file__).resolve().parent.parent / "data" / "language_app.db"
    ).resolve()


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


def _get_migrations_dir() -> Path:
    """Returns the path to the migrations folder."""
    return Path(__file__).resolve().parent / "migrations"


def _load_migrations() -> list[tuple[int, str]]:
    """
    Load all .sql migration files from the migrations folder.
    Returns a sorted list of (version, sql_content) tuples.

    Files must be named: NNN_description.sql (e.g., 001_init.sql)
    """
    migrations_dir = _get_migrations_dir()
    if not migrations_dir.exists():
        return []

    migrations: list[tuple[int, str]] = []
    for sql_file in sorted(migrations_dir.glob("*.sql")):
        # Extract version number from filename (e.g., "001_init.sql" -> 1)
        filename = sql_file.name
        version_str = filename.split("_")[0]
        try:
            version = int(version_str)
        except ValueError:
            continue  # Skip files that don't start with a number

        sql_content = sql_file.read_text(encoding="utf-8")
        migrations.append((version, sql_content))

    return sorted(migrations, key=lambda x: x[0])


def init_db() -> None:
    """
    Initialize the database by running any pending migrations.
    Migrations are loaded from app/persistence/migrations/*.sql files.
    """
    with db_conn() as conn:
        conn.execute(
            "CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)"
        )
        current = conn.execute(
            "SELECT COALESCE(MAX(version), 0) AS v FROM schema_migrations"
        ).fetchone()["v"]

        migrations = _load_migrations()

        for version, sql in migrations:
            if version > int(current):
                conn.executescript(sql)
                conn.execute(
                    "INSERT INTO schema_migrations(version) VALUES (?)", (version,)
                )
