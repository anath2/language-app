"""Pytest configuration for tests."""

import os

# Set required environment variables before importing app modules
os.environ.setdefault("OPENROUTER_API_KEY", "test-key-not-used")
os.environ.setdefault("OPENROUTER_MODEL", "test-model")

# Auth environment variables for testing
os.environ.setdefault("APP_PASSWORD", "test-password")
os.environ.setdefault("APP_SECRET_KEY", "test-secret-key-not-for-production")
os.environ.setdefault("SECURE_COOKIES", "false")
