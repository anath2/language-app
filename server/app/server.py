"""
Language App - FastAPI Application Entry Point

This module provides the FastAPI application instance with:
- Lifespan management (startup/shutdown)
- Authentication middleware
- Static files and template configuration
- Route registration
"""

import os
from contextlib import asynccontextmanager
from pathlib import Path

from fastapi import FastAPI
from fastapi.staticfiles import StaticFiles

from app.auth import AuthMiddleware
from app.persistence import init_db
from app.routes import (
    admin_router,
    api_router,
    auth_router,
    jobs_router,
    translation_router,
)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Lifespan context manager for startup/shutdown events."""
    # Validate required auth environment variables
    if not os.getenv("APP_PASSWORD"):
        raise ValueError("APP_PASSWORD environment variable is required")
    if not os.getenv("APP_SECRET_KEY"):
        raise ValueError("APP_SECRET_KEY environment variable is required")

    # Startup: Initialize database
    init_db()
    yield
    # Shutdown: Nothing needed currently


# Initialize FastAPI app
app = FastAPI(title="Language App", version="1.0.0", lifespan=lifespan)

# Add authentication middleware
app.add_middleware(AuthMiddleware)

# Configure static files
BASE_DIR = Path(__file__).resolve().parent
ROOT_DIR = BASE_DIR.parent.parent
WEB_PUBLIC_DIR = ROOT_DIR / "web" / "public"
app.mount("/css", StaticFiles(directory=WEB_PUBLIC_DIR / "css"), name="css")
app.mount("/static", StaticFiles(directory=BASE_DIR / "static"), name="static")

# Register routers
app.include_router(auth_router)
app.include_router(translation_router)
app.include_router(api_router)
app.include_router(admin_router)
app.include_router(jobs_router)


@app.get("/health")
async def health_check():
    """Health check endpoint for monitoring"""
    return {"status": "ok"}
