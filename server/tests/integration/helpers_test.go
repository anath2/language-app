package integration_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/anath2/language-app/internal/config"
	httprouter "github.com/anath2/language-app/internal/http"
)

func newLocalConfig(t *testing.T) config.Config {
	t.Helper()

	serverRoot := detectServerRoot(t)
	return config.Config{
		Addr:                   ":0",
		AppPassword:            "test-password",
		AppSecretKey:           "test-secret",
		SessionMaxAgeSeconds:   3600,
		SecureCookies:          false,
		MigrationsDir:          filepath.Join(serverRoot, "migrations"),
		TranslationDBPath:      filepath.Join(t.TempDir(), "translations.db"),
		CedictPath:             filepath.Join(serverRoot, "data", "cedict_ts.u8"),
		OpenAIAPIKey:           "test-openai-key",
		OpenAITranslationModel: "openai/gpt-4o-mini",
		OpenAIChatModel:        "openai/gpt-4o-mini",
		OpenAIBaseURL:          "http://127.0.0.1:9/v1",
	}
}

func newUpstreamConfig(t *testing.T) config.Config {
	t.Helper()

	serverRoot := detectServerRoot(t)
	requireEnv(t, "APP_PASSWORD")
	requireEnv(t, "APP_SECRET_KEY")
	requireEnv(t, "OPENAI_API_KEY")
	requireEnv(t, "OPENAI_TRANSLATION_MODEL")
	requireEnv(t, "OPENAI_CHAT_MODEL")
	requireEnv(t, "OPENAI_BASE_URL")

	secureCookies := true
	if raw := strings.TrimSpace(os.Getenv("SECURE_COOKIES")); raw != "" {
		secureCookies = strings.EqualFold(raw, "true")
	}

	sessionHours := 168
	if raw := strings.TrimSpace(os.Getenv("SESSION_MAX_AGE_HOURS")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("invalid SESSION_MAX_AGE_HOURS in env: %v", err)
		}
		sessionHours = parsed
	}

	cedictPath := strings.TrimSpace(os.Getenv("CEDICT_PATH"))
	if cedictPath == "" {
		cedictPath = filepath.Join(serverRoot, "data", "cedict_ts.u8")
	} else if !filepath.IsAbs(cedictPath) {
		cedictPath = filepath.Join(serverRoot, cedictPath)
	}

	return config.Config{
		Addr:                   ":0",
		AppPassword:            os.Getenv("APP_PASSWORD"),
		AppSecretKey:           os.Getenv("APP_SECRET_KEY"),
		SessionMaxAgeSeconds:   sessionHours * 3600,
		SecureCookies:          secureCookies,
		MigrationsDir:          filepath.Join(serverRoot, "migrations"),
		TranslationDBPath:      filepath.Join(t.TempDir(), "translations.db"),
		CedictPath:             cedictPath,
		OpenAIAPIKey:           os.Getenv("OPENAI_API_KEY"),
		OpenAITranslationModel: os.Getenv("OPENAI_TRANSLATION_MODEL"),
		OpenAIChatModel:        os.Getenv("OPENAI_CHAT_MODEL"),
		OpenAIBaseURL:          os.Getenv("OPENAI_BASE_URL"),
		OpenAIDebugLog:         strings.EqualFold(strings.TrimSpace(os.Getenv("OPENAI_DEBUG_LOG")), "true"),
	}
}

func detectServerRoot(t *testing.T) string {
	t.Helper()
	root, err := detectServerRootFromWD()
	if err != nil {
		t.Fatalf("detect server root: %v", err)
	}
	return root
}

func detectServerRootFromWD() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	current := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", os.ErrNotExist
}

func requireEnv(t *testing.T, key string) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		t.Fatalf("required env var %s is missing", key)
	}
	return value
}

func loginSessionCookie(t *testing.T, router http.Handler, password string) string {
	t.Helper()

	res := doJSONRequest(t, router, http.MethodPost, "/api/auth/login", map[string]string{
		"password": password,
	}, "")
	if res.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d", res.Code)
	}

	for _, cookie := range res.Result().Cookies() {
		if cookie.Name == "session" {
			return cookie.String()
		}
	}

	t.Fatal("expected session cookie in login response")
	return ""
}

func doJSONRequest(t *testing.T, router http.Handler, method, path string, payload any, cookie string) *httptest.ResponseRecorder {
	t.Helper()

	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(encoded)
	}

	req := httptest.NewRequest(method, path, body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func decodeBodyJSON(t *testing.T, res *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		t.Fatalf("decode response JSON: %v", err)
	}
}

func doRawRequest(router http.Handler, req *http.Request) *httptest.ResponseRecorder {
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func extractSSEDataLines(body string) []string {
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "data: ")))
		}
	}
	return out
}

func newRouterWithConfig(cfg config.Config) http.Handler {
	return httprouter.NewRouter(cfg)
}
