"""Authentication module for single-user session-based auth."""

import os
import secrets
from datetime import datetime, timezone

from itsdangerous import URLSafeTimedSerializer, BadSignature, SignatureExpired
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import RedirectResponse, Response


def get_secret_key() -> str:
    """Get the secret key for signing sessions."""
    key = os.getenv("APP_SECRET_KEY")
    if not key:
        raise ValueError("APP_SECRET_KEY environment variable is required")
    return key


def get_password() -> str:
    """Get the configured password."""
    password = os.getenv("APP_PASSWORD")
    if not password:
        raise ValueError("APP_PASSWORD environment variable is required")
    return password


def get_session_max_age() -> int:
    """Get session max age in seconds."""
    hours = int(os.getenv("SESSION_MAX_AGE_HOURS", "168"))  # Default 7 days
    return hours * 3600


def is_secure_cookies() -> bool:
    """Check if secure cookies should be used."""
    return os.getenv("SECURE_COOKIES", "true").lower() == "true"


class SessionManager:
    """Manages session creation and verification using signed cookies."""

    COOKIE_NAME = "session"

    def __init__(self):
        self.serializer = URLSafeTimedSerializer(get_secret_key())
        self.max_age = get_session_max_age()

    def create_session(self) -> str:
        """Create a signed session token."""
        data = {
            "authenticated": True,
            "created_at": datetime.now(timezone.utc).isoformat(),
        }
        return self.serializer.dumps(data)

    def verify_session(self, token: str) -> bool:
        """Verify a session token is valid and not expired."""
        try:
            self.serializer.loads(token, max_age=self.max_age)
            return True
        except (BadSignature, SignatureExpired):
            return False

    def set_session_cookie(self, response: Response) -> None:
        """Set the session cookie on a response."""
        token = self.create_session()
        response.set_cookie(
            key=self.COOKIE_NAME,
            value=token,
            max_age=self.max_age,
            httponly=True,
            secure=is_secure_cookies(),
            samesite="lax",
        )

    def clear_session_cookie(self, response: Response) -> None:
        """Clear the session cookie."""
        response.delete_cookie(key=self.COOKIE_NAME)


def verify_password(input_password: str) -> bool:
    """Verify password using constant-time comparison."""
    expected = get_password()
    return secrets.compare_digest(input_password.encode(), expected.encode())


# Lazy-initialized session manager
_session_manager: SessionManager | None = None


def get_session_manager() -> SessionManager:
    """Get or create the session manager singleton."""
    global _session_manager
    if _session_manager is None:
        _session_manager = SessionManager()
    return _session_manager


class AuthMiddleware(BaseHTTPMiddleware):
    """Middleware to protect routes with session authentication."""

    EXCLUDED_PATHS = {"/login", "/health"}
    EXCLUDED_PREFIXES = ("/css/",)

    async def dispatch(self, request: Request, call_next):
        path = request.url.path

        # Check if path is excluded
        if path in self.EXCLUDED_PATHS:
            return await call_next(request)

        for prefix in self.EXCLUDED_PREFIXES:
            if path.startswith(prefix):
                return await call_next(request)

        # Check session cookie
        session_token = request.cookies.get(SessionManager.COOKIE_NAME)
        if session_token and get_session_manager().verify_session(session_token):
            return await call_next(request)

        # Not authenticated - handle based on request type
        is_htmx = request.headers.get("HX-Request") == "true"
        accepts_html = "text/html" in request.headers.get("Accept", "")

        if is_htmx:
            # HTMX request - return redirect header
            response = Response(status_code=401)
            response.headers["HX-Redirect"] = "/login"
            return response
        elif accepts_html:
            # Regular browser request - redirect to login
            return RedirectResponse(url="/login", status_code=303)
        else:
            # API request - return 401 JSON
            return Response(
                content='{"detail": "Not authenticated"}',
                status_code=401,
                media_type="application/json",
            )
