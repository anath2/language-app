# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Language App - A FastAPI web application that segments Chinese text into words, provides pinyin transliteration, and English translations. Uses DSPy with OpenRouter for LLM-powered processing and supports OCR extraction from images.

## Commands

```bash
# Run development server
cd server
uv run uvicorn app.server:app --reload

# Run frontend dev server
cd web
npm install
npm run dev

# Run all tests
cd server
uv run pytest

# Run specific test file
cd server
uv run pytest tests/test_pipeline.py -v

# Type checking (backend)
cd server
uv run pyright

# Type checking (frontend)
cd web
npm run typecheck

# Frontend production build
cd web
npm run build

# Linting
cd server
uv run ruff check .

# Format code
cd server
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
server/
├── app/
│   ├── static/        # Legacy HTMX frontend (still served)
│   └── templates/
web/                   # Svelte 5 SPA (primary frontend)
├── public/css/        # Global CSS (variables, base, segments)
├── src/
│   ├── App.svelte     # Root orchestrator — translations, vocab, review, layout
│   ├── main.ts        # Entry point (mount API)
│   ├── app.css        # Minimal reset
│   ├── lib/
│   │   ├── api.ts     # Typed fetch wrappers (getJson, postJson, deleteRequest)
│   │   ├── utils.ts   # Pure helpers (formatTimeAgo)
│   │   ├── types.ts   # Shared types (translation, review, vocab — no segment types)
│   │   └── router.svelte.ts  # pushState/popstate router (routes: /, /translations/:id)
│   ├── features/
│   │   └── segments/          # Segment feature module
│   │       ├── types.ts       # All segment-related types
│   │       ├── api.ts         # translateBatch API call
│   │       ├── utils.ts       # getPastelColor
│   │       ├── Segments.svelte        # Top-level orchestrator (streaming, edit toggle)
│   │       ├── SegmentDisplay.svelte  # Normal mode: segments + tooltips + vocab
│   │       ├── SegmentEditor.svelte   # Edit mode: split/join with pending tracking
│   │       └── TranslationTable.svelte # Collapsible details table
│   └── components/
│       ├── ReviewPanel.svelte        # SRS review slide-out panel (~110 lines)
│       ├── TranslateForm.svelte      # Text input + OCR upload form (~130 lines)
│       └── TranslationList.svelte     # Translation card list (~70 lines, stateless)
├── vite.config.js     # Dev proxy to FastAPI on :8000
├── tsconfig.json      # Strict TS, verbatimModuleSyntax
└── package.json       # Svelte 5.49.2, Vite 7.3.1, TS 5.6
```

**Svelte 5 Conventions**:

*Runes (reactivity)*:
- `$state()` for all mutable reactive state — typed inline (e.g. `let x = $state<Foo[]>([])`)
- `$derived()` / `$derived.by()` for computed values. Prefer `$derived` over `$effect` for state synchronization.
- `$effect()` only for true side effects (fetching data on mount, DOM interactions). Keep effects minimal.
- `$props()` for component props (replaces `export let`). Destructure with an interface type.
- `$bindable()` for two-way bindable props.

*Event handling*:
- Use `onclick`, `oninput` etc. (properties, not `on:click` directives) — Svelte 5 convention.
- Use callback props instead of `createEventDispatcher`. No event modifier support; wrap handlers manually (e.g. `preventDefault`).

*Snippets & children*:
- Use `{#snippet name(params)}` / `{@render name()}` instead of slots.
- Content inside component tags becomes the `children` snippet prop.

*Component patterns*:
- TypeScript in `<script lang="ts">` — no preprocessor needed for type-only features in Svelte 5.
- Interfaces for all data shapes defined at the top of the component script.
- `mount()` API for entry point (not `new App()`).

*State sharing (when components are split)*:
- For cross-component state, use `.svelte.ts` files with `$state` wrapped in object getters/setters or classes (raw exported `$state` vars lose reactivity across modules).
- For deeply nested component trees, use type-safe context (`setContext`/`getContext` with a `createContext` helper).

*Styling*:
- Global CSS in `web/public/css/` (variables, base, segments) — loaded via `index.html`.
- Component-scoped styles via `<style>` blocks where appropriate.

**Segment Editing UX**:
- Click segment → tooltip with Edit button
- Edit mode shows character boundaries with clickable split points
- Join indicators (⊕) appear between adjacent segments
- Direct click actions (no confirmation popovers)
- Undo button appears after split/join operations

## Development Servers

The frontend dev server (`web/`, Vite on port 5173) is always running. Do not attempt to start it. The backend server (`server/`, FastAPI on port 8000) is also assumed running. Vite proxies API requests to the backend automatically.

After making frontend changes, verify with `cd web && npm run typecheck` — do not restart the dev server.

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

- API layer in `web/src/lib/api.ts` — generic typed fetch wrappers; all endpoints proxied through Vite dev server
- SSE streaming for real-time translation progress (in `features/segments/Segments.svelte`)
- Minimal pushState router in `lib/router.svelte.ts` — routes: `/` (home), `/translations/:id` (detail view). Browser back/forward supported.
- No external state management library — all state is local `$state` runes in App.svelte and Segments.svelte
- Segment editing (split/join) uses `POST /api/segments/translate-batch` to re-translate modified segments
- SRS opacity: 1.0 = new/struggling word (full color), 0 = known word (no highlight)
- Pastel colors cycle through 8 options based on segment index
