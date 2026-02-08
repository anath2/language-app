"""Tests for the persistence layer (Milestone 0)."""

import tempfile
from pathlib import Path

import pytest

from app.persistence import (
    compute_opacity,
    create_event,
    create_text,
    get_db_path,
    get_due_count,
    get_review_queue,
    get_text,
    get_vocab_srs_info,
    init_db,
    initialize_srs_state,
    is_struggling,
    record_lookup,
    record_review_grade,
    save_vocab_item,
    update_vocab_status,
)
from app.persistence.srs import STRUGGLE_OPACITY_BOOST


@pytest.fixture
def temp_db(monkeypatch):
    """Create a temporary database for testing."""
    with tempfile.TemporaryDirectory() as tmpdir:
        db_path = Path(tmpdir) / "test.db"
        monkeypatch.setenv("LANGUAGE_APP_DB_PATH", str(db_path))
        init_db()
        yield db_path


class TestGetDbPath:
    def test_default_path(self, monkeypatch):
        """Uses default path when env var not set."""
        monkeypatch.delenv("LANGUAGE_APP_DB_PATH", raising=False)
        path = get_db_path()
        assert path.name == "language_app.db"
        assert "data" in str(path)

    def test_custom_path_from_env(self, monkeypatch):
        """Uses path from environment variable."""
        monkeypatch.setenv("LANGUAGE_APP_DB_PATH", "/tmp/custom.db")
        path = get_db_path()
        # On macOS, /tmp resolves to /private/tmp
        assert path.name == "custom.db"
        assert "tmp" in str(path)


class TestInitDb:
    def test_creates_tables(self, temp_db):
        """init_db creates all required tables."""
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
        }
        assert expected_tables.issubset(tables)

    def test_migration_is_idempotent(self, temp_db):
        """Running init_db multiple times is safe."""
        # Already initialized in fixture, run again
        init_db()
        init_db()
        # Should not raise


class TestTextCrud:
    def test_create_text_returns_record(self, temp_db):
        """create_text returns a TextRecord with generated id."""
        record = create_text(
            raw_text="你好世界",
            source_type="text",
            metadata={"key": "value"},
        )
        assert record.id is not None
        assert len(record.id) == 32  # UUID hex
        assert record.raw_text == "你好世界"
        assert record.source_type == "text"
        assert record.metadata == {"key": "value"}
        assert record.created_at is not None

    def test_create_text_normalizes_whitespace(self, temp_db):
        """create_text strips leading/trailing whitespace."""
        record = create_text(
            raw_text="  你好  ",
            source_type="text",
            metadata=None,
        )
        assert record.normalized_text == "你好"

    def test_get_text_returns_record(self, temp_db):
        """get_text retrieves a previously created text."""
        created = create_text(
            raw_text="测试文本",
            source_type="ocr",
            metadata={"source": "image.png"},
        )
        retrieved = get_text(created.id)

        assert retrieved is not None
        assert retrieved.id == created.id
        assert retrieved.raw_text == "测试文本"
        assert retrieved.source_type == "ocr"
        assert retrieved.metadata == {"source": "image.png"}

    def test_get_text_returns_none_for_missing(self, temp_db):
        """get_text returns None for non-existent id."""
        result = get_text("nonexistent_id")
        assert result is None


class TestEventCrud:
    def test_create_event_returns_id(self, temp_db):
        """create_event returns the event id."""
        event_id = create_event(
            event_type="tap",
            text_id=None,
            segment_id=None,
            payload={"headword": "你好"},
        )
        assert event_id is not None
        assert len(event_id) == 32

    def test_create_event_with_text_reference(self, temp_db):
        """create_event can reference a text."""
        text = create_text(raw_text="测试", source_type="text", metadata=None)
        event_id = create_event(
            event_type="view",
            text_id=text.id,
            segment_id=None,
            payload={},
        )
        assert event_id is not None


class TestVocabCrud:
    def test_save_vocab_item_creates_new(self, temp_db):
        """save_vocab_item creates a new vocab item."""
        vocab_id = save_vocab_item(
            headword="学习",
            pinyin="xué xí",
            english="to study",
            text_id=None,
            segment_id=None,
            snippet="我喜欢学习中文",
        )
        assert vocab_id is not None
        assert len(vocab_id) == 32

    def test_save_vocab_item_upserts_on_duplicate(self, temp_db):
        """save_vocab_item returns existing id for duplicate headword/pinyin/english."""
        vocab_id_1 = save_vocab_item(
            headword="学习",
            pinyin="xué xí",
            english="to study",
            text_id=None,
            segment_id=None,
            snippet="snippet 1",
        )
        vocab_id_2 = save_vocab_item(
            headword="学习",
            pinyin="xué xí",
            english="to study",
            text_id=None,
            segment_id=None,
            snippet="snippet 2",
        )
        # Same vocab item
        assert vocab_id_1 == vocab_id_2

    def test_save_vocab_item_different_senses_are_distinct(self, temp_db):
        """Different pinyin/english for same headword creates new items."""
        vocab_id_1 = save_vocab_item(
            headword="行",
            pinyin="xíng",
            english="to walk",
            text_id=None,
            segment_id=None,
            snippet=None,
        )
        vocab_id_2 = save_vocab_item(
            headword="行",
            pinyin="háng",
            english="row, line",
            text_id=None,
            segment_id=None,
            snippet=None,
        )
        assert vocab_id_1 != vocab_id_2

    def test_update_vocab_status(self, temp_db):
        """update_vocab_status changes the status."""
        vocab_id = save_vocab_item(
            headword="测试",
            pinyin="cè shì",
            english="test",
            text_id=None,
            segment_id=None,
            snippet=None,
        )
        # Should not raise
        update_vocab_status(vocab_item_id=vocab_id, status="learning")
        update_vocab_status(vocab_item_id=vocab_id, status="known")

    def test_update_vocab_status_rejects_invalid(self, temp_db):
        """update_vocab_status raises for invalid status."""
        vocab_id = save_vocab_item(
            headword="错误",
            pinyin="cuò wù",
            english="error",
            text_id=None,
            segment_id=None,
            snippet=None,
        )
        with pytest.raises(ValueError, match="Invalid status"):
            update_vocab_status(vocab_item_id=vocab_id, status="invalid_status")


class TestSRSFunctions:
    """Tests for SRS (Spaced Repetition System) functions."""

    def test_save_vocab_item_auto_initializes_srs(self, temp_db):
        """save_vocab_item automatically creates SRS state."""
        import sqlite3

        vocab_id = save_vocab_item(
            headword="自动",
            pinyin="zì dòng",
            english="automatic",
            text_id=None,
            segment_id=None,
            snippet=None,
        )

        conn = sqlite3.connect(str(temp_db))
        conn.row_factory = sqlite3.Row
        row = conn.execute(
            "SELECT * FROM srs_state WHERE vocab_item_id = ?", (vocab_id,)
        ).fetchone()
        conn.close()

        assert row is not None
        assert row["vocab_item_id"] == vocab_id
        assert row["last_reviewed_at"] is not None

    def test_initialize_srs_state_creates_record(self, temp_db):
        """initialize_srs_state creates a new SRS record."""
        vocab_id = save_vocab_item(
            headword="初始化",
            pinyin="chū shǐ huà",
            english="initialize",
            text_id=None,
            segment_id=None,
            snippet=None,
        )
        # SRS state already created by save_vocab_item, but initialize should be idempotent
        initialize_srs_state(vocab_id)
        # Should not raise

    def test_compute_opacity_just_looked_up(self, temp_db):
        """compute_opacity returns 1.0 for just looked up words."""
        from datetime import datetime, timezone

        now = datetime.now(timezone.utc).isoformat()
        opacity = compute_opacity(now, is_struggling=False)
        assert opacity == pytest.approx(1.0, abs=0.01)

    def test_compute_opacity_decays_over_time(self, temp_db):
        """compute_opacity decreases as time passes."""
        from datetime import datetime, timedelta, timezone

        # 15 days ago should be ~50% opacity
        past = (datetime.now(timezone.utc) - timedelta(days=15)).isoformat()
        opacity = compute_opacity(past, is_struggling=False)
        assert 0.4 < opacity < 0.6

        # 30+ days ago should be ~0
        old = (datetime.now(timezone.utc) - timedelta(days=35)).isoformat()
        opacity_old = compute_opacity(old, is_struggling=False)
        assert opacity_old == 0.0

    def test_compute_opacity_struggling_has_minimum(self, temp_db):
        """compute_opacity has minimum for struggling words."""
        from datetime import datetime, timedelta, timezone

        # Even 30+ days ago, struggling words have minimum opacity
        old = (datetime.now(timezone.utc) - timedelta(days=35)).isoformat()
        opacity = compute_opacity(old, is_struggling=True)
        assert opacity >= STRUGGLE_OPACITY_BOOST

    def test_record_lookup_updates_timestamp(self, temp_db):
        """record_lookup updates last_reviewed_at."""
        vocab_id = save_vocab_item(
            headword="查询",
            pinyin="chá xún",
            english="lookup",
            text_id=None,
            segment_id=None,
            snippet="测试查询",
        )

        result = record_lookup(vocab_id)
        assert result is not None
        assert result.vocab_item_id == vocab_id
        assert result.headword == "查询"
        assert result.opacity == pytest.approx(1.0, abs=0.01)

    def test_record_lookup_returns_none_for_missing(self, temp_db):
        """record_lookup returns None for non-existent vocab item."""
        result = record_lookup("nonexistent_id")
        assert result is None

    def test_is_struggling_false_initially(self, temp_db):
        """is_struggling returns False for new vocab items."""
        vocab_id = save_vocab_item(
            headword="新词",
            pinyin="xīn cí",
            english="new word",
            text_id=None,
            segment_id=None,
            snippet=None,
        )
        assert is_struggling(vocab_id) is False

    def test_is_struggling_true_after_many_lookups(self, temp_db):
        """is_struggling returns True after 3+ lookups in 7 days."""
        vocab_id = save_vocab_item(
            headword="困难",
            pinyin="kùn nán",
            english="difficult",
            text_id=None,
            segment_id=None,
            snippet=None,
        )

        # Look up 3 times
        record_lookup(vocab_id)
        record_lookup(vocab_id)
        record_lookup(vocab_id)

        assert is_struggling(vocab_id) is True

    def test_get_vocab_srs_info_returns_saved_words(self, temp_db):
        """get_vocab_srs_info returns info for saved headwords."""
        save_vocab_item(
            headword="信息",
            pinyin="xìn xī",
            english="information",
            text_id=None,
            segment_id=None,
            snippet=None,
        )

        results = get_vocab_srs_info(["信息", "不存在"])
        assert len(results) == 1
        assert results[0].headword == "信息"
        assert results[0].opacity == pytest.approx(1.0, abs=0.01)

    def test_get_vocab_srs_info_empty_list(self, temp_db):
        """get_vocab_srs_info returns empty for empty input."""
        results = get_vocab_srs_info([])
        assert results == []


class TestSRSReviewFunctions:
    """Tests for active review (SM-2) functions."""

    def test_record_review_grade_again_resets(self, temp_db):
        """Grade=0 (Again) resets interval and increments lapses."""
        vocab_id = save_vocab_item(
            headword="再来",
            pinyin="zài lái",
            english="again",
            text_id=None,
            segment_id=None,
            snippet=None,
        )
        update_vocab_status(vocab_item_id=vocab_id, status="learning")

        # First Good to establish some state
        state = record_review_grade(vocab_id, grade=2)
        assert state is not None
        assert state.reps == 1
        assert state.interval_days > 0

        # Now Again - should reset
        state = record_review_grade(vocab_id, grade=0)
        assert state is not None
        assert state.reps == 0
        assert state.lapses == 1
        assert state.interval_days == 0.0

    def test_record_review_grade_good_graduates(self, temp_db):
        """Grade=2 (Good) increases interval progressively."""
        vocab_id = save_vocab_item(
            headword="好",
            pinyin="hǎo",
            english="good",
            text_id=None,
            segment_id=None,
            snippet=None,
        )
        update_vocab_status(vocab_item_id=vocab_id, status="learning")

        # First Good: graduating interval (1 day)
        state = record_review_grade(vocab_id, grade=2)
        assert state is not None
        assert state.reps == 1
        assert state.interval_days == pytest.approx(1.0)

        # Second Good: 6 days
        state = record_review_grade(vocab_id, grade=2)
        assert state is not None
        assert state.reps == 2
        assert state.interval_days == pytest.approx(6.0)

        # Third Good: interval * ease
        state = record_review_grade(vocab_id, grade=2)
        assert state is not None
        assert state.reps == 3
        assert state.interval_days > 6.0

    def test_record_review_grade_invalid_raises(self, temp_db):
        """record_review_grade raises for invalid grade."""
        vocab_id = save_vocab_item(
            headword="无效",
            pinyin="wú xiào",
            english="invalid",
            text_id=None,
            segment_id=None,
            snippet=None,
        )
        with pytest.raises(ValueError, match="Grade must be 0, 1, or 2"):
            record_review_grade(vocab_id, grade=5)

    def test_get_review_queue_returns_due_items(self, temp_db):
        """get_review_queue returns items that are due."""
        vocab_id = save_vocab_item(
            headword="队列",
            pinyin="duì liè",
            english="queue",
            text_id=None,
            segment_id=None,
            snippet="复习队列",
        )
        update_vocab_status(vocab_item_id=vocab_id, status="learning")

        # Should be due immediately after saving
        cards = get_review_queue(limit=10)
        assert len(cards) >= 1

        card = next((c for c in cards if c.vocab_item_id == vocab_id), None)
        assert card is not None
        assert card.headword == "队列"
        assert "复习队列" in card.snippets

    def test_get_review_queue_excludes_non_learning(self, temp_db):
        """get_review_queue excludes items not in 'learning' status."""
        vocab_id = save_vocab_item(
            headword="排除",
            pinyin="pái chú",
            english="exclude",
            text_id=None,
            segment_id=None,
            snippet=None,
            status="unknown",  # Explicitly set to unknown to test exclusion
        )

        cards = get_review_queue(limit=10)
        card = next((c for c in cards if c.vocab_item_id == vocab_id), None)
        assert card is None

    def test_get_due_count(self, temp_db):
        """get_due_count returns count of due items."""
        initial_count = get_due_count()

        vocab_id = save_vocab_item(
            headword="计数",
            pinyin="jì shù",
            english="count",
            text_id=None,
            segment_id=None,
            snippet=None,
        )
        update_vocab_status(vocab_item_id=vocab_id, status="learning")

        new_count = get_due_count()
        assert new_count == initial_count + 1
