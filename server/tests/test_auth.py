"""Tests for authentication functionality."""

import tempfile
from pathlib import Path

import pytest
from fastapi.testclient import TestClient


@pytest.fixture
def client(monkeypatch):
    """Create test client with temporary database."""
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
            yield client


class TestLoginPage:
    def test_login_page_accessible(self, client):
        """GET /login returns the login page."""
        response = client.get("/login", follow_redirects=False)
        assert response.status_code == 200
        assert "Password" in response.text

    def test_login_page_redirects_when_authenticated(self, client):
        """GET /login redirects to home when already authenticated."""
        # First login
        login_resp = client.post(
            "/login",
            data={"password": "test-password"},
            follow_redirects=False,
        )
        assert login_resp.status_code == 303

        # Get the session cookie
        session_cookie = login_resp.cookies.get("session")
        assert session_cookie is not None

        # Try to access login page with session
        response = client.get(
            "/login",
            cookies={"session": session_cookie},
            follow_redirects=False,
        )
        assert response.status_code == 303
        assert response.headers["location"] == "/"


class TestLoginSubmit:
    def test_login_success(self, client):
        """POST /login with correct password sets session cookie."""
        response = client.post(
            "/login",
            data={"password": "test-password"},
            follow_redirects=False,
        )
        assert response.status_code == 303
        assert response.headers["location"] == "/"
        assert "session" in response.cookies

    def test_login_wrong_password(self, client):
        """POST /login with wrong password returns error."""
        response = client.post(
            "/login",
            data={"password": "wrong-password"},
            follow_redirects=False,
        )
        assert response.status_code == 401
        assert "Invalid password" in response.text

    def test_login_empty_password(self, client):
        """POST /login with empty password returns error."""
        response = client.post(
            "/login",
            data={"password": ""},
            follow_redirects=False,
        )
        # FastAPI/Pydantic validates Form fields
        assert response.status_code in (401, 422)


class TestProtectedRoutes:
    def test_homepage_requires_auth(self, client):
        """GET / redirects to login when not authenticated."""
        response = client.get(
            "/",
            headers={"Accept": "text/html"},
            follow_redirects=False,
        )
        assert response.status_code == 303
        assert response.headers["location"] == "/login"

    def test_homepage_accessible_when_authenticated(self, client):
        """GET / works when authenticated."""
        # Login first
        login_resp = client.post(
            "/login",
            data={"password": "test-password"},
            follow_redirects=False,
        )
        session_cookie = login_resp.cookies.get("session")

        # Access homepage
        response = client.get(
            "/",
            cookies={"session": session_cookie},
            follow_redirects=False,
        )
        assert response.status_code == 200

    def test_api_returns_401_when_not_authenticated(self, client):
        """API endpoints return 401 JSON when not authenticated."""
        response = client.post(
            "/api/texts",
            json={"raw_text": "test", "source_type": "text"},
            headers={"Accept": "application/json"},
        )
        assert response.status_code == 401
        assert response.json() == {"detail": "Not authenticated"}

    def test_htmx_returns_redirect_header(self, client):
        """HTMX requests return HX-Redirect header when not authenticated."""
        response = client.post(
            "/translate-html",
            data={"text": "test"},
            headers={"HX-Request": "true"},
        )
        assert response.status_code == 401
        assert response.headers.get("HX-Redirect") == "/login"


class TestLogout:
    def test_logout_clears_session(self, client):
        """POST /logout clears session cookie."""
        # Login first
        login_resp = client.post(
            "/login",
            data={"password": "test-password"},
            follow_redirects=False,
        )
        session_cookie = login_resp.cookies.get("session")

        # Logout
        logout_resp = client.post(
            "/logout",
            cookies={"session": session_cookie},
            follow_redirects=False,
        )
        assert logout_resp.status_code == 303
        assert logout_resp.headers["location"] == "/login"

        # Verify session is cleared - cookie should be deleted
        # The response sets the cookie with empty value or max-age=0
        assert "session" in logout_resp.headers.get("set-cookie", "").lower()


class TestExcludedPaths:
    def test_health_endpoint_no_auth(self, client):
        """/health endpoint works without authentication."""
        response = client.get("/health")
        assert response.status_code == 200
        assert response.json() == {"status": "ok"}

    def test_static_files_no_auth(self, client):
        """/static/* paths don't require authentication."""
        # This will 404 if no static file exists, but shouldn't 401
        response = client.get("/static/nonexistent.css", follow_redirects=False)
        # Should be 404 not 303 redirect
        assert response.status_code == 404
