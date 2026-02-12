# AGENTS.md

## Project Overview
Language App is a Go REST API that segments Chinese text into words, provides pinyin transliteration, and English translations. It uses DSPy with OpenAI-compatible upstream APIs and supports SRS vocabulary workflows.

## Commands
```bash
# Run Go server
cd server
go run cmd/server/main.go

# Run all Go tests (includes default integration tests)
cd server
go test ./...

# Run integration tests package only
cd server
go test ./tests/integration -v

# Run upstream-gated integration tests (.env.test required)
cd server
go test ./tests/integration -v -args -upstream
```

## Architecture

### Backend
- DSPy pipeline components: `Segmenter`, `Translator`, `OCRExtractor`, and `Pipeline` (sync and async execution).
- API pattern: JSON endpoints for API consumers and HTML fragment endpoints for HTMX.
- Streaming: `/translate-stream` for SSE progress updates.
- Pipeline uses lazy initialization with a lock via `get_pipeline()` for thread safety.
- Persistence uses SQLite in `app/persistence/` with migrations; DB defaults to `app/data/language_app.db` and can be overridden by `LANGUAGE_APP_DB_PATH`.
- SRS system (`app/persistence/srs.py`) implements SM-2 with status transitions and opacity values.
- CC-CEDICT dictionary (`app/cedict.py`) loads at startup to support translation context.

### Frontend
- Frontend workspace lives in `web/` with Svelte + Vite tooling.
- Server-rendered templates and legacy static assets live in `server/app/templates` and `server/app/static`.
- CSS is split across `variables.css`, `base.css`, and `segments.css` under `web/public/css`.

## Environment Variables
Required in `.env`:
- `OPENROUTER_API_KEY`
- `OPENROUTER_MODEL`
- `APP_PASSWORD`
- `APP_SECRET_KEY`

Optional:
- `SESSION_MAX_AGE_HOURS` (defaults to 168)
- `SECURE_COOKIES` (set `false` for local HTTP development)

## Testing Pattern
Default integration tests avoid upstream API calls and run with local temp DBs. Upstream-dependent integration tests are opt-in with `-upstream` and load `.env.test`.

When touching code under `server/internal/intelligence/`, run the upstream integration tests in `server/tests/integration/upstream_llm_test.go`:

```bash
cd server
go test ./tests/integration -v -run '^TestUpstream' -args -upstream
```

## Key Conventions
- Segment editing currently uses stub API calls for split/join; replace with real backend endpoints when implemented.
- Translation results are stored in a `translationResults` array and synced between `app.js` and `segment-editor.js` via getter/setter.
- SRS opacity: `1.0` for new/struggling words and `0` for known words.
- Pastel colors cycle across 8 options by segment index.
