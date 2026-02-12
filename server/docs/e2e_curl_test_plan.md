# E2E Curl Test Plan (Server Root)

This plan assumes the Go server home path is:

- `/Users/ajitnath/Personal/language-app-server/server`

All commands below run from that directory.

By default, the E2E script uses `.env.test` for runtime config.

## 1) Environment and paths

- Ensure `.env.test` exists in `server/.env.test`.
- Use a disposable DB for the run:
  - `LANGUAGE_APP_DB_PATH=data/e2e_test.db`
- Use a fixed test port:
  - `PORT=18080`
- Artifacts directory:
  - `tmp/e2e/`

## 2) Start server in background

```bash
mkdir -p tmp/e2e data
PORT=18080 LANGUAGE_APP_DB_PATH=data/e2e_test.db go run ./cmd/server > tmp/e2e/server.log 2>&1 &
echo $! > tmp/e2e/server.pid
```

Wait for health:

```bash
for i in $(seq 1 30); do
  code=$(curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:18080/health || true)
  [ "$code" = "200" ] && break
  sleep 1
done
```

## Quick Run (recommended)

```bash
./scripts/e2e_curl.sh
```

Use a different env file when needed:

```bash
ENV_FILE=.env ./scripts/e2e_curl.sh
```

## 3) Unauthenticated auth contract

- JSON request:
  - `GET /api/texts/1` with `Accept: application/json` -> `401` and `{"detail":"Not authenticated"}`
- HTMX request:
  - `GET /api/texts/1` with `HX-Request: true` -> `401` and header `HX-Redirect: /login`
- HTML request:
  - `GET /api/texts/1` with `Accept: text/html` -> `303` and `Location: /login`

## 4) Login and cookie session

```bash
curl -si -c tmp/e2e/cookies.txt -X POST \
  -d "password=${APP_PASSWORD}" \
  http://127.0.0.1:18080/login
```

Expected:
- `303 See Other`
- `Set-Cookie: session=...`

## 5) Translation CRUD + SSE

1. Create:
   - `POST /api/translations` with `{"input_text":"你好世界","source_type":"text"}`
2. Extract `translation_id` from response.
3. Verify:
   - `GET /api/translations/{id}` -> `200`
   - `GET /api/translations/{id}/status` -> `200`
4. Stream:
   - `GET /api/translations/{id}/stream`
   - Verify SSE data includes event payload types:
     - `start`
     - at least one `progress`
     - terminal `complete` or `error`
5. Delete:
   - `DELETE /api/translations/{id}` -> `200`
   - `GET /api/translations/{id}` -> `404`

## 6) Core API checks

- `POST /api/texts` then `GET /api/texts/{id}`
- `POST /api/vocab/save`
- `POST /api/vocab/lookup`
- `GET /api/vocab/srs-info`
- `GET /api/review/queue`
- `GET /api/review/count`
- `POST /api/segments/translate-batch`

## 7) Admin + OCR checks

- `GET /admin/api/profile`
- `POST /admin/api/profile`
- `GET /admin/progress/export`
- `POST /admin/progress/import`
- `POST /extract-text` with multipart dummy file

## 8) Migration command checks

Run migrate twice (idempotency) against same DB:

```bash
PORT=18080 LANGUAGE_APP_DB_PATH=data/e2e_test.db go run ./cmd/migrate
PORT=18080 LANGUAGE_APP_DB_PATH=data/e2e_test.db go run ./cmd/migrate
```

Then restart server and recheck:
- `/health` -> `200`
- login + one protected endpoint -> `200`

## 9) Teardown

```bash
kill "$(cat tmp/e2e/server.pid)" 2>/dev/null || true
```

Artifacts:
- `tmp/e2e/server.log`
- request/response captures under `tmp/e2e/`
- `tmp/e2e/run_info.txt` (resolved env and runtime settings)
- `tmp/e2e/translation_summary.txt` (key translation endpoint outputs)
- `tmp/e2e/failure_context.log` (on assertion failures)

## Troubleshooting

- If translation output looks stubbed, inspect:
  - `tmp/e2e/translate_batch.body`
  - `tmp/e2e/translation_stream.txt`
  - `tmp/e2e/translation_summary.txt`
- If a check fails, review:
  - `tmp/e2e/failure_context.log`
  - `tmp/e2e/server.log`
- To test with alternate model/base URL quickly, set env vars inline:

```bash
OPENAI_MODEL="your/model" OPENAI_BASE_URL="https://openrouter.ai/api/v1" ./scripts/e2e_curl.sh
```
