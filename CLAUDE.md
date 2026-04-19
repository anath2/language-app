# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Language App — a Go REST API that segments Chinese text into words, provides pinyin transliteration, and English translations. Uses a direct OpenAI-compatible HTTP client for structured output. Includes OCR text extraction, SRS vocabulary review, and segment editing.

The Go backend in `server/` is the active implementation. The legacy Python backend in `server_old/` is reference only. The frontend (`web/`) lives in a separate worktree.

## Commands

```bash
# Run Go server (from server/ directory)
go run cmd/server/main.go

# Run database migrations manually
go run cmd/migrate/main.go

# Run Go tests
cd server && go test ./...

# Run a specific Go test package
cd server && go test ./internal/queue/ -v

# Integration/E2E tests (default, no upstream API calls)
cd server && go test ./tests/integration -v

# Upstream-gated integration tests (.env.test, only when explicitly requested)
cd server && go test ./tests/integration -v -args -upstream

# GEPA prompt optimization (segmentation quality)
cd scripts-py && uv run python gepa_segmentation.py --dataset data/jepa/datasets/paragraphs.csv

# Lint OpenAPI spec
npx @redocly/cli lint server/docs/openapi.yaml

# Legacy Python server (reference only)
cd server_old && uv run uvicorn app.server:app --reload
cd server_old && uv run pytest
cd server_old && uv run ruff check .
```

## Architecture

### Go Backend (`server/`)

**Entry point**: `cmd/server/main.go` — loads config from env, starts HTTP server on `:8080` (override with `APP_ADDR` or `PORT`).

**Additional CLI tools** (`cmd/`):
- `migrate/` — Standalone migration runner (migrations also auto-run on server startup).

GEPA prompt optimization for segmentation is run via `scripts-py/gepa_segmentation.py`; it writes `data/jepa/compiled_instruction.txt`, which the translation `Provider` loads at startup.

**Package structure** (`internal/`):
- `config/` — Environment variable loading with legacy key fallbacks (`OPENAI_*` preferred, `OPENROUTER_*` supported). Validates config at startup.
- `http/` — Chi router setup, middleware, route registration, and handlers. `server.go` wires all dependencies via `handlers.ConfigureDependencies()`.
- `http/handlers/` — Request handlers organized by domain. `deps.go` defines three store interfaces (`translationStore`, `srsStore`, `profileStore`) plus the queue manager and two intelligence providers (`translationProvider`, `chatProvider`) as package-level vars. `chat.go` handles chat message creation/listing with SSE streaming; `health.go` serves the health check endpoint.
- `http/routes/` — Route group registration: `auth.go`, `translation.go`, `vocab.go`, `review.go`, `admin.go`, `ocr.go`, `health.go`. Chat endpoints are registered via the translation route group.
- `http/middleware/` — Auth (session cookie-based) and timeout middleware. Timeout is skipped for SSE streaming endpoints.
- `intelligence/` — Defines `TranslationProvider` and `ChatProvider` interfaces plus shared request types (`ChatWithTranslationRequest`, `ChatSegmentContext`). No implementation lives here.
  - `intelligence/translation/` — `Provider` implements `TranslationProvider` via direct HTTP to an OpenAI-compatible endpoint with `response_format: json_schema`. Also contains `parse.go` (fail-fast JSON unmarshal), `guards.go` (CJK detection/segment skip), `cedict.go` (CC-CEDICT dictionary). Loads `data/jepa/compiled_instruction.txt` from the Python GEPA script at startup when present.
  - `intelligence/chat/` — `Provider` implements `ChatProvider` with real OpenAI SSE streaming: POSTs to `/chat/completions` with `stream: true`, reads response line-by-line with `bufio.Scanner`, calls `onChunk` per token.
- `queue/` — In-memory job manager with lease-based processing (30s lease). Tracks running jobs with mutex. Resumes restartable jobs on startup. Segments input by sentence boundaries, processes one-by-one.
- `translation/` — SQLite persistence layer. `store.go` has common types; store files: `store_translation.go` (CRUD, progress), `store_vocab_srs.go` (SM-2 SRS scheduling, review queue, import/export, last-seen vocab context), `store_profile.go` (user profile), `store_jobs.go` (job queue). `db.go` initializes the DB connection; `scan_helpers.go` has shared row-scanning utilities.
- `migrations/` — Goose migration runner. SQL files in `server/migrations/` (15 migrations, latest `00015_drop_texts_and_rewire_translation_fks.sql`).

**Key patterns**:
- Dependency injection via `handlers.ConfigureDependencies(translationStore, srsStore, profileStore, manager, translationProvider, chatProvider)` — package-level vars, not a DI container.
- `intelligence.TranslationProvider` and `intelligence.ChatProvider` interfaces allow swapping LLM backends for testing.
- Translation jobs flow: `POST /api/translations` → `store.Create()` → `manager.StartProcessing()` → background goroutine segments + translates one-by-one → progress saved to DB → SSE stream reads from DB.
- Vocab/SRS flow: `POST /api/vocab/save` upserts `vocab_items` and tracks denormalized context (`last_seen_translation_id`, `last_seen_snippet`, `last_seen_at`, `seen_count`) used by review queues.
- Pure REST API — JSON-only auth (`POST /api/auth/login` with `{"password":"..."}`) returns `{"ok":true}` + Set-Cookie. All admin routes under `/api/admin/*`. OCR at `/api/extract-text`.
- OpenAPI 3.2.0 spec at `server/docs/openapi.yaml`.

**Python scripts** (`scripts-py/` at repo root):
- `gepa_segmentation.py` — GEPA optimization using Python dspy. Outputs `compiled_instruction.txt` to `data/jepa/` and run artifacts to `data/jepa/runs/`. Run with `cd scripts-py && uv run python gepa_segmentation.py`.

## Environment Variables

Requires `.env` file in `server/` (or repo root):
- `OPENAI_API_KEY` (or legacy `OPENROUTER_API_KEY`) — Required for LLM
- `OPENAI_TRANSLATION_MODEL` (or legacy `OPENROUTER_TRANSLATION_MODEL`) — Model for segmentation/translation
- `OPENAI_CHAT_MODEL` (or legacy `OPENROUTER_CHAT_MODEL`) — Model for chat responses (raw SSE streaming)
- `OPENAI_BASE_URL` (or legacy `OPENROUTER_BASE_URL`) — Must end with `/v1`. Defaults to `https://openrouter.ai/api/v1`
- `APP_PASSWORD` — Required for authentication
- `APP_SECRET_KEY` — Required for signing session cookies
- `SESSION_MAX_AGE_HOURS` — Optional, defaults to 168 (7 days)
- `SECURE_COOKIES` — Optional, set to `false` for local HTTP development (defaults to `true`)
- `LANGUAGE_APP_DB_PATH` — Optional, defaults to `server/data/language_app.db`
- `CEDICT_PATH` — Optional, defaults to `server/data/cedict_ts.u8`
- `OPENAI_DEBUG_LOG` — Optional, set `true` to log upstream LLM requests

## Testing Patterns

**Go**: Standard `testing` package. `tests/integration/` contains JSON REST integration coverage (mock provider, temp DB); upstream LLM tests are opt-in via `-args -upstream` (requires `.env.test`).

When touching code under `server/internal/intelligence/`, run the upstream integration tests:

```bash
cd server && go test ./tests/integration -v -run '^TestUpstream' -args -upstream
```

Key unit test files: `intelligence/translation/cedict_test.go` (segment filtering + dictionary + pinyin), `intelligence/translation/parse_test.go` (response parsing), `queue/manager_test.go` (job lifecycle).

## Key Conventions

- **Always run `cd server && gofmt -w .` after finishing a piece of work** (before committing)
- CC-CEDICT pinyin is preferred over LLM-generated pinyin when available
- SRS opacity: 1.0 = new/struggling word (full highlight), 0 = known word (no highlight)
- Segment editing (split/join) re-translates via `POST /api/translations/sentence-segments/translate`
- SSE streaming delivers segment-by-segment translation progress at `/api/translations/{id}/stream`
