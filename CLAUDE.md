# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Language App - A web application that segments Chinese text into words, provides pinyin transliteration, and English translations. Uses dspy-go with OpenAI-compatible endpoints for LLM-powered processing. Includes OCR text extraction, SRS vocabulary review, and segment editing.

The project is migrating from Python (FastAPI) to Go. The Go backend in `server/` is the active implementation. The legacy Python backend lives in `server_old/` for reference.

This is a sparse worktree (`feature/go-migration` branch) containing only `server/` and `server_old/`. The frontend (`web/`) lives in a separate worktree.

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

# E2E curl tests (server must be running)
cd server && bash scripts/e2e_curl.sh

# Legacy Python server (reference only)
cd server_old && uv run uvicorn app.server:app --reload
cd server_old && uv run pytest
cd server_old && uv run ruff check .
cd server_old && uv run ruff format .
```

## Architecture

### Go Backend (`server/`)

**Entry point**: `cmd/server/main.go` — loads config from env, starts HTTP server.

**Package structure** (`internal/`):
- `config/` — Environment variable loading with legacy key fallbacks (`OPENAI_*` preferred, `OPENROUTER_*` supported). Validates config at startup.
- `http/` — Chi router setup, middleware (auth, timeout, CORS), route registration, and handlers. `server.go` is the initialization point that wires together all dependencies.
- `http/handlers/` — Request handlers organized by domain. `deps.go` holds the shared dependency configuration (`ConfigureDependencies`).
- `http/routes/` — Route group registration: `auth.go`, `translation.go`, `api.go`, `admin.go`.
- `http/middleware/` — Auth (session cookie-based) and timeout middleware. Timeout is skipped for SSE streaming endpoints.
- `intelligence/` — LLM integration via `dspy-go`. `Provider` interface with `DSPyProvider` implementation. CC-CEDICT dictionary for context-aware translation. `guards.go` has segment skip/punctuation logic.
- `queue/` — In-memory job manager for background translation processing. Tracks running jobs with mutex. Resumes restartable jobs on startup. Progress persisted to DB.
- `translation/` — SQLite persistence layer (`Store`). Translation CRUD, progress tracking, segment results, vocab, SRS state.
- `migrations/` — Goose migration runner. SQL files in `server/migrations/` (auto-run on server startup).

**Key patterns**:
- Dependency injection via `handlers.ConfigureDependencies(store, manager, provider)` — package-level vars, not a DI container.
- `intelligence.Provider` interface allows swapping LLM backends for testing.
- Translation jobs flow: `POST /api/translations` → `store.Create()` → `manager.StartProcessing()` → background goroutine segments + translates one-by-one → progress saved to DB → SSE stream reads from DB.
- Migrations run automatically in `NewRouter()` before serving requests.
- Go server default port is `:8080` (override with `APP_ADDR` or `PORT`).

### Route Contract

Full route inventory is frozen in `server/docs/route_contract.md`. Key patterns:
- Session cookie auth with three unauthenticated response modes (HTMX → 401 + HX-Redirect, HTML → 303 redirect, JSON → 401)
- SPA fallback: non-API paths serve `index.html`
- SSE streaming at `/api/translations/{id}/stream`

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

**Go**: Standard `testing` package. `dspy_provider_endpoint_test.go` tests the LLM integration (requires running API endpoint). `guards_cedict_test.go` tests segment filtering and dictionary logic. `config_test.go` and `server_test.go` test configuration and router setup. `queue/manager_test.go` tests the job manager.

**Python (legacy)**: Tests mock DSPy's `ChainOfThought` and `Predict` at the module level to avoid API calls. Environment variables are set before importing `app.server`.

## Key Conventions

- Environment variable keys prefer `OPENAI_*` prefix; `OPENROUTER_*` still supported as fallback
- CC-CEDICT pinyin is preferred over LLM-generated pinyin when available
- SRS opacity: 1.0 = new/struggling word (full highlight), 0 = known word (no highlight)
- Segment editing (split/join) re-translates via `POST /api/segments/translate-batch`
- SSE streaming delivers segment-by-segment translation progress
