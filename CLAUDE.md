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
│   ├── App.svelte     # Root orchestrator (~500 lines — state, API, layout)
│   ├── main.ts        # Entry point (mount API)
│   ├── app.css        # Minimal reset
│   ├── lib/
│   │   ├── api.ts     # Typed fetch wrappers (getJson, postJson, deleteRequest)
│   │   ├── utils.ts   # Pure helpers (getPastelColor, formatTimeAgo)
│   │   └── types.ts   # Shared interfaces/types for all components
│   └── components/
│       ├── ReviewPanel.svelte        # SRS review slide-out panel (~110 lines)
│       ├── TranslateForm.svelte      # Text input + OCR upload form (~130 lines)
│       ├── JobQueue.svelte           # Job card list (~70 lines, stateless)
│       ├── SegmentDisplay.svelte     # Segments + tooltip + progress (~250 lines)
│       └── TranslationTable.svelte   # Collapsible details table (~40 lines)
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
- SSE streaming for real-time translation progress (implemented directly in App.svelte via `EventSource`)
- No routing library — SPA uses conditional rendering (`isExpandedView` boolean) for view switching
- No external state management library — all state is local `$state` runes in App.svelte
- Segment editing uses stub API calls (`stubSplitSegment`, `stubJoinSegments`) - replace with real backend endpoints when implemented
- SRS opacity: 1.0 = new/struggling word (full color), 0 = known word (no highlight)
- Pastel colors cycle through 8 options based on segment index
