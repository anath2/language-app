# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Language App — a Go REST API that segments Chinese text into words, provides pinyin transliteration, and English translations. Uses dspy-go with OpenAI-compatible endpoints for LLM-powered processing. Includes OCR text extraction, SRS vocabulary review, and segment editing.

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
cd server && go run cmd/gepa-segmentation/main.go --dataset data/jepa/sentences_20.csv

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
- `gepa-segmentation/` — GEPA prompt optimization for segmentation quality. Writes compiled instruction to `data/jepa/compiled_instruction.txt`, which `DSPyProvider` loads at runtime (falls back to a default instruction).

**Package structure** (`internal/`):
- `config/` — Environment variable loading with legacy key fallbacks (`OPENAI_*` preferred, `OPENROUTER_*` supported). Validates config at startup.
- `http/` — Chi router setup, middleware, route registration, and handlers. `server.go` wires all dependencies via `handlers.ConfigureDependencies()`.
- `http/handlers/` — Request handlers organized by domain. `deps.go` defines four store interfaces (`translationStore`, `textEventStore`, `srsStore`, `profileStore`) plus the queue manager and intelligence provider as package-level vars.
- `http/routes/` — Route group registration: `auth.go`, `translation.go`, `api.go`, `admin.go`.
- `http/middleware/` — Auth (session cookie-based) and timeout middleware. Timeout is skipped for SSE streaming endpoints.
- `intelligence/` — LLM integration via dspy-go `modules.Predict` with structured output. `Provider` interface (`Segment`, `TranslateSegments`). `DSPyProvider` implementation loads CC-CEDICT dictionary for preferred pinyin and compiled GEPA instruction for segmentation. `guards.go` has CJK detection and segment skip logic. Response parsing handles multiple LLM output formats (JSON arrays, objects with "segments" key, markdown-fenced blocks, newline-separated, freeform text).
- `queue/` — In-memory job manager with lease-based processing (30s lease). Tracks running jobs with mutex. Resumes restartable jobs on startup. Segments input by sentence boundaries, processes one-by-one.
- `translation/` — SQLite persistence layer split into four store files: `store_translation.go` (CRUD, progress), `store_vocab_srs.go` (SM-2 SRS scheduling, review queue, export/import), `store_text_events.go` (text records, events), `store_profile.go` (user profile). Common types in `store.go`.
- `migrations/` — Goose migration runner. SQL files in `server/migrations/` (6 migrations, latest `00006_go_compat.sql`).

**Key patterns**:
- Dependency injection via `handlers.ConfigureDependencies(translationStore, textEventStore, srsStore, profileStore, manager, provider)` — package-level vars, not a DI container.
- `intelligence.Provider` interface allows swapping LLM backends for testing.
- Translation jobs flow: `POST /api/translations` → `store.Create()` → `manager.StartProcessing()` → background goroutine segments + translates one-by-one → progress saved to DB → SSE stream reads from DB.
- Pure REST API — JSON-only auth (`POST /api/auth/login` with `{"password":"..."}`) returns `{"ok":true}` + Set-Cookie. All admin routes under `/api/admin/*`. OCR at `/api/extract-text`.
- OpenAPI 3.2.0 spec at `server/docs/openapi.yaml`.

**Scripts** (`scripts/segmentation/`):
- `gepa_harness.go` — Full GEPA optimization pipeline used by `cmd/gepa-segmentation/`. Supports multi-seed optimization campaigns. Outputs artifacts to `data/jepa/`.

## Environment Variables

Requires `.env` file in `server/` (or repo root):
- `OPENAI_API_KEY` (or legacy `OPENROUTER_API_KEY`) — Required for LLM
- `OPENAI_MODEL` (or legacy `OPENROUTER_MODEL`) — Model identifier
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

Key unit test files: `dspy_provider_endpoint_test.go` (URL normalization), `dspy_provider_parse_test.go` (response parsing), `guards_cedict_test.go` (segment filtering + dictionary), `queue/manager_test.go` (job lifecycle).

## Key Conventions

- **Always run `cd server && gofmt -w .` after finishing a piece of work** (before committing)
- CC-CEDICT pinyin is preferred over LLM-generated pinyin when available
- SRS opacity: 1.0 = new/struggling word (full highlight), 0 = known word (no highlight)
- Segment editing (split/join) re-translates via `POST /api/segments/translate-batch`
- SSE streaming delivers segment-by-segment translation progress at `/api/translations/{id}/stream`
