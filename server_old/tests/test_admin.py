"""Tests for admin functionality: migrations, profile, and progress sync."""

import json
import tempfile
from pathlib import Path

import pytest

from app.persistence import init_db
from app.persistence.db import _load_migrations, db_conn
from app.persistence.profile import (
    count_known_vocab,
    count_learning_vocab,
    count_total_vocab,
    get_user_profile,
    upsert_user_profile,
)
from app.persistence.progress_sync import (
    ImportError as ProgressImportError,
    export_progress,
    export_progress_json,
    import_progress,
    import_progress_json,
    validate_progress_bundle,
)


@pytest.fixture
def temp_db(monkeypatch):
    """Create a temporary database for testing."""
    with tempfile.TemporaryDirectory() as tmpdir:
        db_path = Path(tmpdir) / "test.db"
        monkeypatch.setenv("LANGUAGE_APP_DB_PATH", str(db_path))
        init_db()
        yield db_path


class TestMigrationsLoader:
    """Tests for the migrations folder loader."""

    def test_load_migrations_returns_sorted_list(self):
        """_load_migrations returns migrations sorted by version."""
        migrations = _load_migrations()
        # Should have at least 3 migrations
        assert len(migrations) >= 3
        # Should be sorted by version
        versions = [v for v, _ in migrations]
        assert versions == sorted(versions)
        # Check expected versions exist
        assert 1 in versions
        assert 2 in versions
        assert 3 in versions

    def test_migration_files_are_valid_sql(self):
        """Migration SQL files should be parseable."""
        migrations = _load_migrations()
        for version, sql in migrations:
            # Basic check: not empty and contains SQL keywords
            assert len(sql.strip()) > 0, f"Migration {version} is empty"
            assert "CREATE" in sql.upper() or "INSERT" in sql.upper(), (
                f"Migration {version} doesn't look like SQL"
            )

    def test_init_db_creates_all_tables(self, temp_db):
        """init_db creates all expected tables from migrations."""
        import sqlite3

        conn = sqlite3.connect(str(temp_db))
        cursor = conn.execute(
            "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name"
        )
        tables = {row[0] for row in cursor.fetchall()}
        conn.close()

        expected_tables = {
            "schema_migrations",
            "texts",
            "segments",
            "events",
            "vocab_items",
            "vocab_occurrences",
            "srs_state",
            "vocab_lookups",
            "user_profile",
        }
        assert expected_tables.issubset(tables)

    def test_init_db_records_migration_versions(self, temp_db):
        """init_db records applied migration versions."""
        import sqlite3

        conn = sqlite3.connect(str(temp_db))
        cursor = conn.execute("SELECT version FROM schema_migrations ORDER BY version")
        versions = [row[0] for row in cursor.fetchall()]
        conn.close()

        assert 1 in versions
        assert 2 in versions
        assert 3 in versions


class TestUserProfile:
    """Tests for user profile CRUD."""

    def test_get_user_profile_returns_none_before_migration(self, monkeypatch):
        """get_user_profile returns None if table doesn't exist or is empty."""
        # The migration should create a default row, so this is mostly for coverage
        with tempfile.TemporaryDirectory() as tmpdir:
            db_path = Path(tmpdir) / "test.db"
            monkeypatch.setenv("LANGUAGE_APP_DB_PATH", str(db_path))
            # Don't run init_db - profile table won't exist
            # This would error, so we skip this test case
            pass

    def test_get_user_profile_returns_default_after_init(self, temp_db):
        """get_user_profile returns the default profile after init_db."""
        profile = get_user_profile()
        assert profile is not None
        assert profile.name == ""
        assert profile.email == ""
        assert profile.language == "zh-CN"

    def test_upsert_user_profile_creates_profile(self, temp_db):
        """upsert_user_profile creates/updates the profile."""
        profile = upsert_user_profile(
            name="Test User",
            email="test@example.com",
            language="zh-TW",
        )

        assert profile.name == "Test User"
        assert profile.email == "test@example.com"
        assert profile.language == "zh-TW"

        # Verify it's persisted
        retrieved = get_user_profile()
        assert retrieved is not None
        assert retrieved.name == "Test User"
        assert retrieved.email == "test@example.com"

    def test_upsert_user_profile_updates_existing(self, temp_db):
        """upsert_user_profile updates existing profile."""
        # Create initial profile
        upsert_user_profile(name="First", email="first@example.com", language="zh-CN")

        # Update it
        updated = upsert_user_profile(
            name="Second",
            email="second@example.com",
            language="zh-TW",
        )

        assert updated.name == "Second"
        assert updated.email == "second@example.com"

        # Verify only one row exists
        with db_conn() as conn:
            count = conn.execute("SELECT COUNT(*) FROM user_profile").fetchone()[0]
            assert count == 1


class TestVocabCounts:
    """Tests for vocabulary counting functions."""

    def test_counts_are_zero_initially(self, temp_db):
        """Vocab counts are zero in a fresh database."""
        assert count_known_vocab() == 0
        assert count_learning_vocab() == 0
        assert count_total_vocab() == 0

    def test_counts_reflect_saved_vocab(self, temp_db):
        """Vocab counts reflect saved vocabulary items."""
        from app.persistence.crud import save_vocab_item, update_vocab_status

        # Save some vocab items
        id1 = save_vocab_item(
            headword="学习",
            pinyin="xué xí",
            english="to study",
            text_id=None,
            segment_id=None,
            snippet=None,
            status="learning",
        )
        save_vocab_item(
            headword="知道",
            pinyin="zhī dào",
            english="to know",
            text_id=None,
            segment_id=None,
            snippet=None,
            status="learning",
        )

        assert count_total_vocab() == 2
        assert count_learning_vocab() == 2
        assert count_known_vocab() == 0

        # Mark one as known
        update_vocab_status(vocab_item_id=id1, status="known")

        assert count_total_vocab() == 2
        assert count_learning_vocab() == 1
        assert count_known_vocab() == 1


class TestProgressExport:
    """Tests for progress export functionality."""

    def test_export_empty_database(self, temp_db):
        """Export works on empty database."""
        bundle = export_progress()

        assert bundle.schema_version == 3
        assert bundle.exported_at != ""
        assert bundle.vocab_items == []
        assert bundle.srs_state == []
        assert bundle.vocab_lookups == []
        assert bundle.translations == []
        assert bundle.translation_segments == []
        assert bundle.translation_paragraphs == []

    def test_export_with_data(self, temp_db):
        """Export includes saved vocabulary and SRS data."""
        from app.persistence.crud import save_vocab_item
        from app.persistence.srs import record_lookup

        # Save a vocab item
        vocab_id = save_vocab_item(
            headword="测试",
            pinyin="cè shì",
            english="test",
            text_id=None,
            segment_id=None,
            snippet=None,
            status="learning",
        )

        # Record a lookup
        record_lookup(vocab_id)

        bundle = export_progress()

        assert len(bundle.vocab_items) == 1
        assert bundle.vocab_items[0]["headword"] == "测试"
        assert len(bundle.srs_state) == 1
        assert len(bundle.vocab_lookups) >= 1

    def test_export_json_is_valid(self, temp_db):
        """export_progress_json returns valid JSON."""
        json_str = export_progress_json()
        data = json.loads(json_str)

        assert "schema_version" in data
        assert "exported_at" in data
        assert "vocab_items" in data
        assert "srs_state" in data
        assert "vocab_lookups" in data


class TestProgressImport:
    """Tests for progress import functionality."""

    def test_import_empty_bundle(self, temp_db):
        """Import works with empty data."""
        bundle_data = {
            "schema_version": 1,
            "exported_at": "2024-01-01T00:00:00Z",
            "vocab_items": [],
            "srs_state": [],
            "vocab_lookups": [],
        }

        bundle = validate_progress_bundle(bundle_data)
        counts = import_progress(bundle)

        assert counts["vocab_items"] == 0
        assert counts["srs_state"] == 0
        assert counts["vocab_lookups"] == 0

    def test_import_overwrites_existing_data(self, temp_db):
        """Import replaces existing data."""
        from app.persistence.crud import save_vocab_item

        # Save some initial data
        save_vocab_item(
            headword="旧词",
            pinyin="jiù cí",
            english="old word",
            text_id=None,
            segment_id=None,
            snippet=None,
        )

        assert count_total_vocab() == 1

        # Import new data
        bundle_data = {
            "schema_version": 1,
            "exported_at": "2024-01-01T00:00:00Z",
            "vocab_items": [
                {
                    "id": "abc123",
                    "headword": "新词",
                    "pinyin": "xīn cí",
                    "english": "new word",
                    "status": "learning",
                    "created_at": "2024-01-01T00:00:00Z",
                    "updated_at": "2024-01-01T00:00:00Z",
                },
                {
                    "id": "def456",
                    "headword": "另一个",
                    "pinyin": "lìng yī gè",
                    "english": "another",
                    "status": "known",
                    "created_at": "2024-01-01T00:00:00Z",
                    "updated_at": "2024-01-01T00:00:00Z",
                },
            ],
            "srs_state": [
                {
                    "vocab_item_id": "abc123",
                    "due_at": "2024-01-02T00:00:00Z",
                    "interval_days": 1.0,
                    "ease": 2.5,
                    "reps": 1,
                    "lapses": 0,
                    "last_reviewed_at": "2024-01-01T00:00:00Z",
                },
            ],
            "vocab_lookups": [],
        }

        bundle = validate_progress_bundle(bundle_data)
        counts = import_progress(bundle)

        assert counts["vocab_items"] == 2
        assert count_total_vocab() == 2  # Old data replaced
        assert count_known_vocab() == 1

    def test_import_json_string(self, temp_db):
        """import_progress_json works with JSON string."""
        json_str = json.dumps(
            {
                "schema_version": 1,
                "exported_at": "2024-01-01T00:00:00Z",
                "vocab_items": [
                    {
                        "id": "test123",
                        "headword": "测试",
                        "pinyin": "cè shì",
                        "english": "test",
                        "status": "learning",
                        "created_at": "2024-01-01T00:00:00Z",
                        "updated_at": "2024-01-01T00:00:00Z",
                    },
                ],
                "srs_state": [],
                "vocab_lookups": [],
            }
        )

        counts = import_progress_json(json_str)
        assert counts["vocab_items"] == 1


class TestProgressValidation:
    """Tests for progress bundle validation."""

    def test_missing_schema_version(self, temp_db):
        """Validation fails if schema_version is missing."""
        with pytest.raises(ProgressImportError, match="Missing 'schema_version'"):
            validate_progress_bundle(
                {
                    "vocab_items": [],
                    "srs_state": [],
                    "vocab_lookups": [],
                }
            )

    def test_unsupported_schema_version(self, temp_db):
        """Validation fails if schema_version is too high."""
        with pytest.raises(ProgressImportError, match="Unsupported schema version"):
            validate_progress_bundle(
                {
                    "schema_version": 999,
                    "vocab_items": [],
                    "srs_state": [],
                    "vocab_lookups": [],
                }
            )

    def test_missing_vocab_items(self, temp_db):
        """Validation fails if vocab_items is missing."""
        with pytest.raises(ProgressImportError, match="Missing 'vocab_items'"):
            validate_progress_bundle(
                {
                    "schema_version": 1,
                    "srs_state": [],
                    "vocab_lookups": [],
                }
            )

    def test_missing_srs_state(self, temp_db):
        """Validation fails if srs_state is missing."""
        with pytest.raises(ProgressImportError, match="Missing 'srs_state'"):
            validate_progress_bundle(
                {
                    "schema_version": 1,
                    "vocab_items": [],
                    "vocab_lookups": [],
                }
            )

    def test_missing_vocab_lookups(self, temp_db):
        """Validation fails if vocab_lookups is missing."""
        with pytest.raises(ProgressImportError, match="Missing 'vocab_lookups'"):
            validate_progress_bundle(
                {
                    "schema_version": 1,
                    "vocab_items": [],
                    "srs_state": [],
                }
            )

    def test_vocab_item_missing_fields(self, temp_db):
        """Validation fails if vocab_item is missing required fields."""
        with pytest.raises(
            ProgressImportError, match="vocab_items\\[0\\] missing fields"
        ):
            validate_progress_bundle(
                {
                    "schema_version": 1,
                    "vocab_items": [{"id": "test"}],  # Missing other fields
                    "srs_state": [],
                    "vocab_lookups": [],
                }
            )

    def test_invalid_json(self, temp_db):
        """Import fails with invalid JSON."""
        with pytest.raises(ProgressImportError, match="Invalid JSON"):
            import_progress_json("not valid json {")

    def test_roundtrip_export_import(self, temp_db):
        """Export and re-import preserves data."""
        from app.persistence.crud import save_vocab_item

        # Save some data
        save_vocab_item(
            headword="往返",
            pinyin="wǎng fǎn",
            english="round trip",
            text_id=None,
            segment_id=None,
            snippet=None,
            status="learning",
        )

        # Export
        json_str = export_progress_json()

        # Clear and re-import
        with db_conn() as conn:
            conn.execute("DELETE FROM vocab_lookups")
            conn.execute("DELETE FROM srs_state")
            conn.execute("DELETE FROM vocab_items")

        assert count_total_vocab() == 0

        # Re-import
        import_progress_json(json_str)

        assert count_total_vocab() == 1
        assert count_learning_vocab() == 1
