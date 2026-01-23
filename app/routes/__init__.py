"""
FastAPI route handlers.

This package organizes API endpoints by domain:
- auth: Authentication and homepage routes
- translation: Translation and OCR routes
- api: REST API for persistence and SRS
"""

from app.routes.api import router as api_router
from app.routes.auth import router as auth_router
from app.routes.translation import router as translation_router

__all__ = [
    "auth_router",
    "translation_router",
    "api_router",
]
