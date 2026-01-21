# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Language App - A FastAPI web application that segments Chinese text into words, provides pinyin transliteration, and English translations. Uses DSPy with OpenRouter for LLM-powered processing and supports OCR extraction from images.

## Commands

```bash
# Run development server
uv run uvicorn app.server:app --reload

# Run all tests
uv run pytest

# Run specific test file
uv run pytest tests/test_pipeline.py -v

# Type checking
uv run pyright

# Linting
uv run ruff check .

# Format code
uv run ruff format .
```

## Architecture

**DSPy Pipeline Pattern**: The core processing uses DSPy's declarative approach with typed Signatures:
- `Segmenter` - Segments Chinese text into word list
- `Translator` - Batch translates segments to (pinyin, english) tuples
- `OCRExtractor` - Extracts Chinese text from images
- `Pipeline` - Orchestrates segmenter → translator flow with both sync (`forward`) and async (`aforward`) methods

**API Structure**: Dual endpoint pattern - JSON endpoints (`/translate-text`, `/translate-image`) for API consumers and HTML fragment endpoints (`/translate-html`, `/translate-image-html`) for HTMX frontend.

**Thread Safety**: Pipeline uses lazy initialization with a lock (`get_pipeline()`) to ensure safe concurrent access.

**Persistence Layer** (`app/persistence.py`): SQLite-based local storage with migration system. Tables: `texts`, `segments`, `events`, `vocab_items`, `vocab_occurrences`, `srs_state`. Uses context manager `db_conn()` for connections. DB location defaults to `app/data/language_app.db`, override with `LANGUAGE_APP_DB_PATH` env var.

## Environment Variables

Requires `.env` file with:
- `OPENROUTER_API_KEY` - Required for LLM processing
- `OPENROUTER_MODEL` - Model identifier for OpenRouter
- `APP_PASSWORD` - Required for authentication
- `APP_SECRET_KEY` - Required for signing session cookies
- `SESSION_MAX_AGE_HOURS` - Optional, defaults to 168 (7 days)
- `SECURE_COOKIES` - Optional, set to `false` for local HTTP development (defaults to `true`)

## Testing Pattern

Tests mock DSPy's `ChainOfThought` and `Predict` at the module level to avoid API calls while testing orchestration logic. Environment variables are set before importing `app.server`.
