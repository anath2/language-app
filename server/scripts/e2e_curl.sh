#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

ENV_FILE="${ENV_FILE:-.env.test}"
if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ENV_FILE"
  set +a
else
  echo "FAILED [env_file]: env file not found: $ENV_FILE"
  exit 1
fi

PORT_VALUE="${PORT:-$((39000 + RANDOM % 1000))}"
BASE_URL="${BASE_URL:-http://127.0.0.1:${PORT_VALUE}}"
DB_PATH="${E2E_DB_PATH:-data/e2e_test.db}"
MIGRATIONS_DIR="${E2E_MIGRATIONS_DIR:-migrations}"
ARTIFACT_DIR="${ARTIFACT_DIR:-tmp/e2e}"
COOKIE_JAR="$ARTIFACT_DIR/cookies.txt"
SERVER_LOG="$ARTIFACT_DIR/server.log"
SERVER_PID_FILE="$ARTIFACT_DIR/server.pid"
SERVER_BIN="$ARTIFACT_DIR/server_bin"
RUN_INFO="$ARTIFACT_DIR/run_info.txt"
SUMMARY_FILE="$ARTIFACT_DIR/translation_summary.txt"
CURRENT_HDR=""
CURRENT_BODY=""

mkdir -p "$ARTIFACT_DIR" "$(dirname "$DB_PATH")"
rm -f "$COOKIE_JAR" "$SERVER_LOG" "$SERVER_PID_FILE" "$DB_PATH" "$SERVER_BIN" "$RUN_INFO" "$SUMMARY_FILE"

cleanup() {
  if [[ -f "$SERVER_PID_FILE" ]]; then
    pid="$(cat "$SERVER_PID_FILE" || true)"
    if [[ -n "${pid:-}" ]] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  fi
}
trap cleanup EXIT

status_from_headers() {
  awk 'NR==1 {print $2}' "$1"
}

assert_status() {
  expected="$1"
  actual="$2"
  label="$3"
  if [[ "$expected" != "$actual" ]]; then
    echo "FAILED [$label]: expected status $expected, got $actual"
    dump_failure_context "$label"
    exit 1
  fi
}

assert_contains() {
  needle="$1"
  file="$2"
  label="$3"
  if ! rg -q "$needle" "$file"; then
    echo "FAILED [$label]: expected to find '$needle' in $file"
    dump_failure_context "$label"
    exit 1
  fi
}

extract_json_value() {
  key="$1"
  file="$2"
  sed -nE "s/.*\"${key}\"[[:space:]]*:[[:space:]]*\"([^\"]+)\".*/\1/p" "$file" | head -n1
}

dump_failure_context() {
  label="$1"
  {
    echo "---- FAILURE CONTEXT [$label] ----"
    echo "env_file=$ENV_FILE"
    echo "base_url=$BASE_URL"
    echo "db_path=$DB_PATH"
    echo "migrations_dir=$MIGRATIONS_DIR"
    if [[ -n "$CURRENT_HDR" && -f "$CURRENT_HDR" ]]; then
      echo "---- response headers ----"
      cat "$CURRENT_HDR"
    fi
    if [[ -n "$CURRENT_BODY" && -f "$CURRENT_BODY" ]]; then
      echo "---- response body ----"
      sed -n '1,120p' "$CURRENT_BODY"
    fi
    if [[ -f "$SERVER_LOG" ]]; then
      echo "---- server log tail ----"
      tail -n 120 "$SERVER_LOG"
    fi
    echo "---- END FAILURE CONTEXT ----"
  } | tee "$ARTIFACT_DIR/failure_context.log"
}

echo "env_file=$ENV_FILE" > "$RUN_INFO"
echo "base_url=$BASE_URL" >> "$RUN_INFO"
echo "port=$PORT_VALUE" >> "$RUN_INFO"
echo "db_path=$DB_PATH" >> "$RUN_INFO"
echo "migrations_dir=$MIGRATIONS_DIR" >> "$RUN_INFO"
echo "artifact_dir=$ARTIFACT_DIR" >> "$RUN_INFO"

echo "==> Running migrate command twice (idempotency)"
PORT="$PORT_VALUE" LANGUAGE_APP_DB_PATH="$DB_PATH" LANGUAGE_APP_MIGRATIONS_DIR="$MIGRATIONS_DIR" \
  go run ./cmd/migrate > "$ARTIFACT_DIR/migrate_1.log" 2>&1
PORT="$PORT_VALUE" LANGUAGE_APP_DB_PATH="$DB_PATH" LANGUAGE_APP_MIGRATIONS_DIR="$MIGRATIONS_DIR" \
  go run ./cmd/migrate > "$ARTIFACT_DIR/migrate_2.log" 2>&1

echo "==> Starting server in background"
go build -o "$SERVER_BIN" ./cmd/server
PORT="$PORT_VALUE" LANGUAGE_APP_DB_PATH="$DB_PATH" LANGUAGE_APP_MIGRATIONS_DIR="$MIGRATIONS_DIR" \
  "$SERVER_BIN" > "$SERVER_LOG" 2>&1 &
echo $! > "$SERVER_PID_FILE"

echo "==> Waiting for health endpoint"
for _ in $(seq 1 30); do
  if ! kill -0 "$(cat "$SERVER_PID_FILE")" 2>/dev/null; then
    echo "FAILED [server_start]: background server process exited"
    cat "$SERVER_LOG"
    exit 1
  fi
  code="$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health" || true)"
  if [[ "$code" == "200" ]]; then
    break
  fi
  sleep 1
done
assert_status 200 "$code" "health"

echo "==> Unauthenticated auth behavior checks"
CURRENT_HDR="$ARTIFACT_DIR/unauth_json.hdr"
CURRENT_BODY="$ARTIFACT_DIR/unauth_json.body"
curl -sS -D "$ARTIFACT_DIR/unauth_json.hdr" -o "$ARTIFACT_DIR/unauth_json.body" \
  -H "Accept: application/json" "$BASE_URL/api/texts/1"
assert_status 401 "$(status_from_headers "$ARTIFACT_DIR/unauth_json.hdr")" "unauth_json_status"
assert_contains "\"detail\":\"Not authenticated\"" "$ARTIFACT_DIR/unauth_json.body" "unauth_json_body"

CURRENT_HDR="$ARTIFACT_DIR/unauth_htmx.hdr"
CURRENT_BODY="$ARTIFACT_DIR/unauth_htmx.body"
curl -sS -D "$ARTIFACT_DIR/unauth_htmx.hdr" -o "$ARTIFACT_DIR/unauth_htmx.body" \
  -H "HX-Request: true" "$BASE_URL/api/texts/1"
assert_status 401 "$(status_from_headers "$ARTIFACT_DIR/unauth_htmx.hdr")" "unauth_htmx_status"
assert_contains "Hx-Redirect: /login" "$ARTIFACT_DIR/unauth_htmx.hdr" "unauth_htmx_header"

CURRENT_HDR="$ARTIFACT_DIR/unauth_html.hdr"
CURRENT_BODY="$ARTIFACT_DIR/unauth_html.body"
curl -sS -D "$ARTIFACT_DIR/unauth_html.hdr" -o "$ARTIFACT_DIR/unauth_html.body" \
  -H "Accept: text/html" "$BASE_URL/api/texts/1"
assert_status 303 "$(status_from_headers "$ARTIFACT_DIR/unauth_html.hdr")" "unauth_html_status"
assert_contains "Location: /login" "$ARTIFACT_DIR/unauth_html.hdr" "unauth_html_header"

echo "==> Login and session"
CURRENT_HDR="$ARTIFACT_DIR/login.hdr"
CURRENT_BODY="$ARTIFACT_DIR/login.body"
curl -sS -D "$ARTIFACT_DIR/login.hdr" -o "$ARTIFACT_DIR/login.body" -c "$COOKIE_JAR" \
  -X POST -d "password=${APP_PASSWORD}" "$BASE_URL/login"
assert_status 303 "$(status_from_headers "$ARTIFACT_DIR/login.hdr")" "login_status"

CURRENT_HDR="$ARTIFACT_DIR/translations_page.hdr"
CURRENT_BODY="$ARTIFACT_DIR/translations_page.body"
curl -sS -D "$ARTIFACT_DIR/translations_page.hdr" -o "$ARTIFACT_DIR/translations_page.body" \
  -b "$COOKIE_JAR" "$BASE_URL/translations"
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/translations_page.hdr")" "translations_page_status"

echo "==> Translation CRUD + SSE"
CURRENT_HDR="$ARTIFACT_DIR/create_translation.hdr"
CURRENT_BODY="$ARTIFACT_DIR/create_translation.body"
curl -sS -D "$ARTIFACT_DIR/create_translation.hdr" -o "$ARTIFACT_DIR/create_translation.body" \
  -b "$COOKIE_JAR" -H "Content-Type: application/json" \
  -X POST "$BASE_URL/api/translations" \
  -d '{"input_text":"人工智能改变世界","source_type":"text"}'
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/create_translation.hdr")" "create_translation_status"
translation_id="$(extract_json_value "translation_id" "$ARTIFACT_DIR/create_translation.body")"
if [[ -z "$translation_id" ]]; then
  echo "FAILED [create_translation_id]: missing translation_id"
  dump_failure_context "create_translation_id"
  exit 1
fi

CURRENT_HDR="$ARTIFACT_DIR/get_translation.hdr"
CURRENT_BODY="$ARTIFACT_DIR/get_translation.body"
curl -sS -D "$ARTIFACT_DIR/get_translation.hdr" -o "$ARTIFACT_DIR/get_translation.body" \
  -b "$COOKIE_JAR" "$BASE_URL/api/translations/$translation_id"
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/get_translation.hdr")" "get_translation_status"

CURRENT_HDR="$ARTIFACT_DIR/get_translation_status.hdr"
CURRENT_BODY="$ARTIFACT_DIR/get_translation_status.body"
curl -sS -D "$ARTIFACT_DIR/get_translation_status.hdr" -o "$ARTIFACT_DIR/get_translation_status.body" \
  -b "$COOKIE_JAR" "$BASE_URL/api/translations/$translation_id/status"
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/get_translation_status.hdr")" "get_translation_status_status"

CURRENT_HDR="$ARTIFACT_DIR/translation_stream.hdr"
CURRENT_BODY="$ARTIFACT_DIR/translation_stream.txt"
curl -sS -D "$ARTIFACT_DIR/translation_stream.hdr" -o "$ARTIFACT_DIR/translation_stream.txt" \
  -b "$COOKIE_JAR" "$BASE_URL/api/translations/$translation_id/stream"
assert_contains "\"type\":\"start\"" "$ARTIFACT_DIR/translation_stream.txt" "sse_start"
assert_contains "\"type\":\"progress\"" "$ARTIFACT_DIR/translation_stream.txt" "sse_progress"
if ! rg -q "\"type\":\"complete\"|\"type\":\"error\"" "$ARTIFACT_DIR/translation_stream.txt"; then
  echo "FAILED [sse_terminal]: expected complete or error event"
  dump_failure_context "sse_terminal"
  exit 1
fi

CURRENT_HDR="$ARTIFACT_DIR/delete_translation.hdr"
CURRENT_BODY="$ARTIFACT_DIR/delete_translation.body"
curl -sS -D "$ARTIFACT_DIR/delete_translation.hdr" -o "$ARTIFACT_DIR/delete_translation.body" \
  -b "$COOKIE_JAR" -X DELETE "$BASE_URL/api/translations/$translation_id"
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/delete_translation.hdr")" "delete_translation_status"

CURRENT_HDR="$ARTIFACT_DIR/get_deleted_translation.hdr"
CURRENT_BODY="$ARTIFACT_DIR/get_deleted_translation.body"
curl -sS -D "$ARTIFACT_DIR/get_deleted_translation.hdr" -o "$ARTIFACT_DIR/get_deleted_translation.body" \
  -b "$COOKIE_JAR" "$BASE_URL/api/translations/$translation_id"
assert_status 404 "$(status_from_headers "$ARTIFACT_DIR/get_deleted_translation.hdr")" "get_deleted_translation_status"

echo "==> Core API checks"
CURRENT_HDR="$ARTIFACT_DIR/create_text.hdr"
CURRENT_BODY="$ARTIFACT_DIR/create_text.body"
curl -sS -D "$ARTIFACT_DIR/create_text.hdr" -o "$ARTIFACT_DIR/create_text.body" \
  -b "$COOKIE_JAR" -H "Content-Type: application/json" \
  -X POST "$BASE_URL/api/texts" \
  -d '{"raw_text":"人工智能改变世界","source_type":"text","metadata":{"source":"e2e"}}'
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/create_text.hdr")" "create_text_status"
text_id="$(extract_json_value "id" "$ARTIFACT_DIR/create_text.body")"
if [[ -z "$text_id" ]]; then
  echo "FAILED [create_text_id]: missing text id"
  dump_failure_context "create_text_id"
  exit 1
fi

CURRENT_HDR="$ARTIFACT_DIR/get_text.hdr"
CURRENT_BODY="$ARTIFACT_DIR/get_text.body"
curl -sS -D "$ARTIFACT_DIR/get_text.hdr" -o "$ARTIFACT_DIR/get_text.body" \
  -b "$COOKIE_JAR" "$BASE_URL/api/texts/$text_id"
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/get_text.hdr")" "get_text_status"

CURRENT_HDR="$ARTIFACT_DIR/save_vocab.hdr"
CURRENT_BODY="$ARTIFACT_DIR/save_vocab.body"
curl -sS -D "$ARTIFACT_DIR/save_vocab.hdr" -o "$ARTIFACT_DIR/save_vocab.body" \
  -b "$COOKIE_JAR" -H "Content-Type: application/json" \
  -X POST "$BASE_URL/api/vocab/save" \
  -d "{\"headword\":\"你好\",\"pinyin\":\"ni hao\",\"english\":\"hello\",\"text_id\":\"$text_id\",\"status\":\"learning\"}"
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/save_vocab.hdr")" "save_vocab_status"
vocab_item_id="$(extract_json_value "vocab_item_id" "$ARTIFACT_DIR/save_vocab.body")"
if [[ -z "$vocab_item_id" ]]; then
  echo "FAILED [save_vocab_id]: missing vocab_item_id"
  dump_failure_context "save_vocab_id"
  exit 1
fi

CURRENT_HDR="$ARTIFACT_DIR/vocab_lookup.hdr"
CURRENT_BODY="$ARTIFACT_DIR/vocab_lookup.body"
curl -sS -D "$ARTIFACT_DIR/vocab_lookup.hdr" -o "$ARTIFACT_DIR/vocab_lookup.body" \
  -b "$COOKIE_JAR" -H "Content-Type: application/json" \
  -X POST "$BASE_URL/api/vocab/lookup" \
  -d "{\"vocab_item_id\":\"$vocab_item_id\"}"
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/vocab_lookup.hdr")" "vocab_lookup_status"

CURRENT_HDR="$ARTIFACT_DIR/vocab_srs_info.hdr"
CURRENT_BODY="$ARTIFACT_DIR/vocab_srs_info.body"
curl -sS -D "$ARTIFACT_DIR/vocab_srs_info.hdr" -o "$ARTIFACT_DIR/vocab_srs_info.body" \
  -b "$COOKIE_JAR" "$BASE_URL/api/vocab/srs-info?headwords=%E4%BD%A0%E5%A5%BD"
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/vocab_srs_info.hdr")" "vocab_srs_info_status"

CURRENT_HDR="$ARTIFACT_DIR/review_queue.hdr"
CURRENT_BODY="$ARTIFACT_DIR/review_queue.body"
curl -sS -D "$ARTIFACT_DIR/review_queue.hdr" -o "$ARTIFACT_DIR/review_queue.body" \
  -b "$COOKIE_JAR" "$BASE_URL/api/review/queue"
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/review_queue.hdr")" "review_queue_status"

CURRENT_HDR="$ARTIFACT_DIR/review_count.hdr"
CURRENT_BODY="$ARTIFACT_DIR/review_count.body"
curl -sS -D "$ARTIFACT_DIR/review_count.hdr" -o "$ARTIFACT_DIR/review_count.body" \
  -b "$COOKIE_JAR" "$BASE_URL/api/review/count"
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/review_count.hdr")" "review_count_status"

CURRENT_HDR="$ARTIFACT_DIR/translate_batch.hdr"
CURRENT_BODY="$ARTIFACT_DIR/translate_batch.body"
curl -sS -D "$ARTIFACT_DIR/translate_batch.hdr" -o "$ARTIFACT_DIR/translate_batch.body" \
  -b "$COOKIE_JAR" -H "Content-Type: application/json" \
  -X POST "$BASE_URL/api/segments/translate-batch" \
  -d '{"segments":["人工智能","改变","世界"],"context":"人工智能改变世界"}'
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/translate_batch.hdr")" "translate_batch_status"
assert_contains "\"translations\"" "$ARTIFACT_DIR/translate_batch.body" "translate_batch_shape"

echo "==> Admin + OCR checks"
CURRENT_HDR="$ARTIFACT_DIR/profile_get.hdr"
CURRENT_BODY="$ARTIFACT_DIR/profile_get.body"
curl -sS -D "$ARTIFACT_DIR/profile_get.hdr" -o "$ARTIFACT_DIR/profile_get.body" \
  -b "$COOKIE_JAR" "$BASE_URL/admin/api/profile"
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/profile_get.hdr")" "profile_get_status"

CURRENT_HDR="$ARTIFACT_DIR/profile_post.hdr"
CURRENT_BODY="$ARTIFACT_DIR/profile_post.body"
curl -sS -D "$ARTIFACT_DIR/profile_post.hdr" -o "$ARTIFACT_DIR/profile_post.body" \
  -b "$COOKIE_JAR" -H "Content-Type: application/json" \
  -X POST "$BASE_URL/admin/api/profile" \
  -d '{"name":"E2E User","email":"e2e@example.com","language":"zh-CN"}'
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/profile_post.hdr")" "profile_post_status"

CURRENT_HDR="$ARTIFACT_DIR/export_progress.hdr"
CURRENT_BODY="$ARTIFACT_DIR/export_progress.body"
curl -sS -D "$ARTIFACT_DIR/export_progress.hdr" -o "$ARTIFACT_DIR/export_progress.body" \
  -b "$COOKIE_JAR" "$BASE_URL/admin/progress/export"
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/export_progress.hdr")" "export_progress_status"

printf '{"vocab_items":[],"srs_state":[],"vocab_lookups":[]}' > "$ARTIFACT_DIR/import_payload.json"
CURRENT_HDR="$ARTIFACT_DIR/import_progress.hdr"
CURRENT_BODY="$ARTIFACT_DIR/import_progress.body"
curl -sS -D "$ARTIFACT_DIR/import_progress.hdr" -o "$ARTIFACT_DIR/import_progress.body" \
  -b "$COOKIE_JAR" \
  -X POST "$BASE_URL/admin/progress/import" \
  -F "file=@$ARTIFACT_DIR/import_payload.json;type=application/json"
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/import_progress.hdr")" "import_progress_status"

printf "fake-image-bytes" > "$ARTIFACT_DIR/dummy.png"
CURRENT_HDR="$ARTIFACT_DIR/extract_text.hdr"
CURRENT_BODY="$ARTIFACT_DIR/extract_text.body"
curl -sS -D "$ARTIFACT_DIR/extract_text.hdr" -o "$ARTIFACT_DIR/extract_text.body" \
  -b "$COOKIE_JAR" -X POST "$BASE_URL/extract-text" \
  -F "image=@$ARTIFACT_DIR/dummy.png;type=image/png"
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/extract_text.hdr")" "extract_text_status"

echo "==> Restart check after migration"
cleanup
PORT="$PORT_VALUE" LANGUAGE_APP_DB_PATH="$DB_PATH" LANGUAGE_APP_MIGRATIONS_DIR="$MIGRATIONS_DIR" \
  "$SERVER_BIN" > "$ARTIFACT_DIR/server_restart.log" 2>&1 &
echo $! > "$SERVER_PID_FILE"

for _ in $(seq 1 30); do
  if ! kill -0 "$(cat "$SERVER_PID_FILE")" 2>/dev/null; then
    echo "FAILED [server_restart]: background server process exited"
    cat "$ARTIFACT_DIR/server_restart.log"
    exit 1
  fi
  code="$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health" || true)"
  if [[ "$code" == "200" ]]; then
    break
  fi
  sleep 1
done
assert_status 200 "$code" "health_after_restart"

curl -sS -D "$ARTIFACT_DIR/login_restart.hdr" -o "$ARTIFACT_DIR/login_restart.body" -c "$COOKIE_JAR" \
  -X POST -d "password=${APP_PASSWORD}" "$BASE_URL/login"
assert_status 303 "$(status_from_headers "$ARTIFACT_DIR/login_restart.hdr")" "login_after_restart"

curl -sS -D "$ARTIFACT_DIR/translations_after_restart.hdr" -o "$ARTIFACT_DIR/translations_after_restart.body" \
  -b "$COOKIE_JAR" "$BASE_URL/translations"
assert_status 200 "$(status_from_headers "$ARTIFACT_DIR/translations_after_restart.hdr")" "translations_after_restart"

{
  echo "translation_id=$translation_id"
  echo "translation_status_json=$(tr '\n' ' ' < "$ARTIFACT_DIR/get_translation_status.body")"
  echo "translate_batch_json=$(tr '\n' ' ' < "$ARTIFACT_DIR/translate_batch.body")"
  echo "sse_preview=$(sed -n '1,10p' "$ARTIFACT_DIR/translation_stream.txt" | tr '\n' ' ')"
} > "$SUMMARY_FILE"

echo "E2E curl run completed successfully."
echo "Artifacts: $ARTIFACT_DIR"
