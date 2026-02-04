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

### Backend

**DSPy Pipeline Pattern**: The core processing uses DSPy's declarative approach with typed Signatures:
- `Segmenter` - Segments Chinese text into word list
- `Translator` - Batch translates segments to (pinyin, english) tuples
- `OCRExtractor` - Extracts Chinese text from images
- `Pipeline` - Orchestrates segmenter → translator flow with both sync (`forward`) and async (`aforward`) methods

**API Structure**: Dual endpoint pattern - JSON endpoints (`/translate-text`, `/translate-image`) for API consumers and HTML fragment endpoints (`/translate-html`, `/translate-image-html`) for HTMX frontend. Streaming endpoint `/translate-stream` provides SSE for real-time translation progress.

**Thread Safety**: Pipeline uses lazy initialization with a lock (`get_pipeline()`) to ensure safe concurrent access.

**Persistence Layer** (`app/persistence/`): SQLite-based local storage with migration system. Tables: `texts`, `segments`, `events`, `vocab_items`, `vocab_occurrences`, `vocab_lookups`, `srs_state`, `user_profile`. Uses context manager `db_conn()` for connections. DB location defaults to `app/data/language_app.db`, override with `LANGUAGE_APP_DB_PATH` env var.

**SRS System** (`app/persistence/srs.py`): SM-2 spaced repetition algorithm with passive lookup tracking. Tracks vocabulary through status transitions (unknown → learning → known) and calculates opacity for UI visualization based on review recency.

**Dictionary Integration** (`app/cedict.py`): CC-CEDICT dictionary loaded at startup. Definitions are passed to the LLM for context-aware translation selection.

### Frontend

**File Structure**:
```
app/
├── static/
│   ├── css/
│   │   ├── variables.css    # CSS custom properties (colors, fonts, sizes)
│   │   ├── base.css         # Body, buttons, forms, inputs, spinners
│   │   └── segments.css     # Segments, editing UI, review panel, tooltips
│   └── js/
│       ├── segment-editor.js # Split/join segment functionality (IIFE)
│       └── app.js            # Core app logic, translation, SRS
└── templates/
    └── index.html            # HTML structure only (~150 lines)
```

**JavaScript Modules**:

`segment-editor.js` - IIFE exposing `SegmentEditor` object:
- `init(deps)` - Initialize with dependencies (getPastelColor, addSegmentInteraction, etc.)
- `enterEditMode(segment)` - Enter segment editing mode (shows split points, join indicators)
- `exitEditMode()` - Exit editing mode
- Handles split/join operations with undo support

`app.js` - Main application logic:
- Translation streaming via SSE (`/translate-stream`)
- SRS tracking (savedVocabMap, opacity updates)
- Review panel (flashcard system)
- Segment interactions (tooltips, click-to-pin)
- Exposes `window.App` for inline onclick handlers

**CSS Organization**:
- `variables.css` - All CSS custom properties (--primary, --pastel-*, --text-*, etc.)
- `base.css` - Base typography, buttons (.btn-primary, .btn-secondary), form inputs, spinners
- `segments.css` - Segment colors, states (.segment-pending, .saved, .editing), review panel

**Segment Editing UX**:
- Click segment → tooltip with Edit button
- Edit mode shows character boundaries with clickable split points
- Join indicators (⊕) appear between adjacent segments
- Direct click actions (no confirmation popovers)
- Undo button appears after split/join operations

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

## Key Conventions

- Segment editing uses stub API calls (`stubSplitSegment`, `stubJoinSegments`) - replace with real backend endpoints when implemented
- Translation results stored in `translationResults` array, synced between app.js and segment-editor.js via getter/setter
- SRS opacity: 1.0 = new/struggling word (full color), 0 = known word (no highlight)
- Pastel colors cycle through 8 options based on segment index
