"""Tests for admin API endpoints (profile management)."""

import tempfile
from pathlib import Path

import pytest
from fastapi.testclient import TestClient

from app.persistence.profile import upsert_user_profile, get_user_profile
from app.server import app


@pytest.fixture
def client_unauthed():
    """Create unauthenticated test client."""
    with tempfile.TemporaryDirectory() as tmpdir:
        db_path = Path(tmpdir) / "test.db"
        try:
            # Set environment variables for the test
            import os
            os.environ["LANGUAGE_APP_DB_PATH"] = str(db_path)
            os.environ["OPENROUTER_API_KEY"] = "test-key"
            os.environ["OPENROUTER_MODEL"] = "test-model"
            os.environ["APP_PASSWORD"] = "test-password"
            os.environ["APP_SECRET_KEY"] = "test-secret-key"
            os.environ["SECURE_COOKIES"] = "false"

            from app.server import app

            with TestClient(app) as client:
                yield client
        finally:
            # Cleanup
            if "LANGUAGE_APP_DB_PATH" in os.environ:
                del os.environ["LANGUAGE_APP_DB_PATH"]


@pytest.fixture
def client(monkeypatch):
    """Create authenticated test client with temporary database."""
    with tempfile.TemporaryDirectory() as tmpdir:
        db_path = Path(tmpdir) / "test.db"
        monkeypatch.setenv("LANGUAGE_APP_DB_PATH", str(db_path))
        monkeypatch.setenv("OPENROUTER_API_KEY", "test-key")
        monkeypatch.setenv("OPENROUTER_MODEL", "test-model")
        monkeypatch.setenv("APP_PASSWORD", "test-password")
        monkeypatch.setenv("APP_SECRET_KEY", "test-secret-key")
        monkeypatch.setenv("SECURE_COOKIES", "false")

        from app.server import app

        with TestClient(app) as client:
            # Login to get session cookie (TestClient auto-follows redirect)
            client.post("/login", data={"password": "test-password"})
            yield client


class TestAdminProfileAPI:
    """Test GET /admin/api/profile and POST /admin/api/profile"""

    def test_get_profile_default_profile(self, client):
        """GET /admin/api/profile - returns default profile when no profile has been set."""
        res = client.get("/admin/api/profile")
        assert res.status_code == 200
        data = res.json()
        assert data["profile"]["name"] == ""
        assert data["profile"]["email"] == ""
        assert data["profile"]["language"] == "zh-CN"
        assert data["vocabStats"] == {"known": 0, "learning": 0, "total": 0}

    def test_get_profile_with_profile(self, client):
        """GET /admin/api/profile - profile exists."""
        # Create profile first via upsert
        profile = upsert_user_profile(
            name="Test User",
            email="test@example.com",
            language="Mandarin"
        )

        res = client.get("/admin/api/profile")
        assert res.status_code == 200
        data = res.json()
        assert data["profile"]["name"] == "Test User"
        assert data["profile"]["email"] == "test@example.com"
        assert data["profile"]["language"] == "Mandarin"
        assert data["vocabStats"] == {"known": 0, "learning": 0, "total": 0}

    def test_get_profile_requires_auth(self, client_unauthed):
        """GET /admin/api/profile - must be authenticated."""
        res = client_unauthed.get("/api/admin/api/profile")
        assert res.status_code == 401

    def test_update_profile_create_new(self, client):
        """POST /admin/api/profile - creates new profile."""
        data = {"name": "New User", "email": "new@example.com", "language": "Mandarin"}

        res = client.post("/admin/api/profile", data=data)
        assert res.status_code == 200
        result = res.json()
        assert result["profile"]["name"] == "New User"
        assert result["profile"]["email"] == "new@example.com"
        assert result["profile"]["language"] == "Mandarin"

        # Verify via db
        profile = get_user_profile()
        assert profile is not None
        assert profile.name == "New User"
        assert profile.email == "new@example.com"
        assert profile.language == "Mandarin"

    def test_update_profile_update_existing(self, client):
        """POST /admin/api/profile - updates existing profile."""
        # Create initial profile
        upsert_user_profile(
            name="Initial Name",
            email="initial@example.com",
            language="Cantonese"
        )

        # Update it
        data = {"name": "Updated Name", "email": "updated@example.com", "language": "Mandarin"}
        res = client.post("/admin/api/profile", data=data)
        assert res.status_code == 200
        result = res.json()
        assert result["profile"]["name"] == "Updated Name"
        assert result["profile"]["email"] == "updated@example.com"
        assert result["profile"]["language"] == "Mandarin"

        # Verify via db
        profile = get_user_profile()
        assert profile.name == "Updated Name"
        assert profile.email == "updated@example.com"
        assert profile.language == "Mandarin"

    def test_update_profile_missing_fields(self, client):
        """POST /admin/api/profile - missing required fields."""
        # Missing email
        data = {"name": "User", "language": "Mandarin"}
        res = client.post("/admin/api/profile", data=data)
        assert res.status_code == 422

        # Missing name
        data = {"email": "user@example.com", "language": "Mandarin"}
        res = client.post("/admin/api/profile", data=data)
        assert res.status_code == 422

        # Missing language
        data = {"name": "User", "email": "user@example.com"}
        res = client.post("/admin/api/profile", data=data)
        assert res.status_code == 422

    def test_update_profile_requires_auth(self, client_unauthed):
        """POST /api/admin/api/profile - must be authenticated."""
        data = {"name": "User", "email": "user@example.com", "language": "Mandarin"}
        res = client_unauthed.post("/api/admin/api/profile", data=data)
        assert res.status_code == 401

    def test_vocab_stats_with_data(self, client):
        """GET /api/admin/api/profile - should count saved vocab."""
        # Create vocab items with different statuses directly
        from app.persistence.db import db_conn
        from datetime import datetime, timezone

        now = datetime.now(timezone.utc).isoformat()
        with db_conn() as conn:
            conn.executemany(
                "INSERT INTO vocab_items (id, headword, pinyin, english, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
                [
                    (1, "你好", "nǐ hǎo", "hello", "known", now, now),
                    (2, "学习", "xué xí", "study", "learning", now, now),
                    (3, "再见", "zài jiàn", "goodbye", "unknown", now, now),
                    (4, "朋友", "péng you", "friend", "known", now, now),
                ],
            )

        res = client.get("/admin/api/profile")
        assert res.status_code == 200
        data = res.json()
        assert data["vocabStats"] == {"known": 2, "learning": 1, "total": 4}