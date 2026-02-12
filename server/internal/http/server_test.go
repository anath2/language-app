package http_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anath2/language-app/internal/config"
	httprouter "github.com/anath2/language-app/internal/http"
)

func newTestConfig(t *testing.T) config.Config {
	t.Helper()

	tmp := t.TempDir()
	serverRoot := detectServerRoot(t)

	return config.Config{
		Addr:                 ":0",
		AppPassword:          "test-password",
		AppSecretKey:         "test-secret",
		SessionMaxAgeSeconds: 3600,
		SecureCookies:        false,
		MigrationsDir:        filepath.Join(serverRoot, "migrations"),
		TranslationDBPath:    filepath.Join(tmp, "translations.db"),
		OpenAIAPIKey:         "test-openrouter-key",
		OpenAIModel:          "openai/gpt-4o-mini",
		OpenAIBaseURL:        "http://127.0.0.1:9/v1",
	}
}

func detectServerRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	current := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	t.Fatalf("unable to detect server root from %s", wd)
	return ""
}

func loginAndGetSessionCookie(t *testing.T, router http.Handler, password string) string {
	t.Helper()

	payload, _ := json.Marshal(map[string]string{"password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected login to return 200, got %d", res.Code)
	}

	setCookies := res.Result().Cookies()
	for _, c := range setCookies {
		if c.Name == "session" {
			return c.String()
		}
	}
	t.Fatal("expected session cookie in login response")
	return ""
}

func TestRouteContractWithAuthenticatedSession(t *testing.T) {
	cfg := newTestConfig(t)
	router := httprouter.NewRouter(cfg)
	sessionCookie := loginAndGetSessionCookie(t, router, cfg.AppPassword)

	tests := []struct {
		name   string
		method string
		path   string
		status int
	}{
		{name: "health", method: http.MethodGet, path: "/health", status: http.StatusOK},
		{name: "logout", method: http.MethodPost, path: "/api/auth/logout", status: http.StatusOK},
		{name: "create translation", method: http.MethodPost, path: "/api/translations", status: http.StatusBadRequest},
		{name: "list translations", method: http.MethodGet, path: "/api/translations", status: http.StatusOK},
		{name: "get translation", method: http.MethodGet, path: "/api/translations/123", status: http.StatusNotFound},
		{name: "translation status", method: http.MethodGet, path: "/api/translations/123/status", status: http.StatusNotFound},
		{name: "delete translation", method: http.MethodDelete, path: "/api/translations/123", status: http.StatusNotFound},
		{name: "create text", method: http.MethodPost, path: "/api/texts", status: http.StatusBadRequest},
		{name: "get text", method: http.MethodGet, path: "/api/texts/123", status: http.StatusNotFound},
		{name: "create event", method: http.MethodPost, path: "/api/events", status: http.StatusBadRequest},
		{name: "save vocab", method: http.MethodPost, path: "/api/vocab/save", status: http.StatusBadRequest},
		{name: "update vocab status", method: http.MethodPost, path: "/api/vocab/status", status: http.StatusBadRequest},
		{name: "lookup vocab", method: http.MethodPost, path: "/api/vocab/lookup", status: http.StatusBadRequest},
		{name: "vocab srs info", method: http.MethodGet, path: "/api/vocab/srs-info", status: http.StatusOK},
		{name: "review queue", method: http.MethodGet, path: "/api/review/queue", status: http.StatusOK},
		{name: "review answer", method: http.MethodPost, path: "/api/review/answer", status: http.StatusBadRequest},
		{name: "review count", method: http.MethodGet, path: "/api/review/count", status: http.StatusOK},
		{name: "translate batch", method: http.MethodPost, path: "/api/segments/translate-batch", status: http.StatusBadRequest},
		{name: "export progress", method: http.MethodGet, path: "/api/admin/progress/export", status: http.StatusOK},
		{name: "import progress", method: http.MethodPost, path: "/api/admin/progress/import", status: http.StatusBadRequest},
		{name: "get profile", method: http.MethodGet, path: "/api/admin/profile", status: http.StatusOK},
		{name: "update profile", method: http.MethodPost, path: "/api/admin/profile", status: http.StatusBadRequest},
		{name: "extract text no file", method: http.MethodPost, path: "/api/extract-text", status: http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Cookie", sessionCookie)
			res := httptest.NewRecorder()

			router.ServeHTTP(res, req)

			if res.Code != tc.status {
				t.Fatalf("expected status %d, got %d", tc.status, res.Code)
			}
		})
	}

	sseReq := httptest.NewRequest(http.MethodGet, "/api/translations/123/stream", nil)
	sseReq.Header.Set("Cookie", sessionCookie)
	sseRes := httptest.NewRecorder()
	router.ServeHTTP(sseRes, sseReq)

	if sseRes.Code != http.StatusOK {
		t.Fatalf("expected SSE status 200, got %d", sseRes.Code)
	}
	if got := sseRes.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("expected SSE content type, got %q", got)
	}
}

func TestTranslationCRUDFlow(t *testing.T) {
	cfg := newTestConfig(t)
	router := httprouter.NewRouter(cfg)
	sessionCookie := loginAndGetSessionCookie(t, router, cfg.AppPassword)

	reqBody := map[string]string{
		"input_text":  "你好世界",
		"source_type": "text",
	}
	payload, _ := json.Marshal(reqBody)

	createReq := httptest.NewRequest(http.MethodPost, "/api/translations", bytes.NewReader(payload))
	createReq.Header.Set("Cookie", sessionCookie)
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	router.ServeHTTP(createRes, createReq)

	if createRes.Code != http.StatusOK {
		t.Fatalf("expected create status 200, got %d", createRes.Code)
	}

	var created struct {
		TranslationID string `json:"translation_id"`
		Status        string `json:"status"`
	}
	if err := json.NewDecoder(createRes.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.TranslationID == "" {
		t.Fatal("expected translation_id in create response")
	}
	if created.Status != "pending" {
		t.Fatalf("expected pending status, got %q", created.Status)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/translations?limit=20&offset=0", nil)
	listReq.Header.Set("Cookie", sessionCookie)
	listRes := httptest.NewRecorder()
	router.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d", listRes.Code)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/translations/"+created.TranslationID, nil)
	detailReq.Header.Set("Cookie", sessionCookie)
	detailRes := httptest.NewRecorder()
	router.ServeHTTP(detailRes, detailReq)
	if detailRes.Code != http.StatusOK {
		t.Fatalf("expected detail status 200, got %d", detailRes.Code)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/translations/"+created.TranslationID+"/status", nil)
	statusReq.Header.Set("Cookie", sessionCookie)
	statusRes := httptest.NewRecorder()
	router.ServeHTTP(statusRes, statusReq)
	if statusRes.Code != http.StatusOK {
		t.Fatalf("expected status endpoint 200, got %d", statusRes.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/translations/"+created.TranslationID, nil)
	deleteReq.Header.Set("Cookie", sessionCookie)
	deleteRes := httptest.NewRecorder()
	router.ServeHTTP(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusOK {
		t.Fatalf("expected delete status 200, got %d", deleteRes.Code)
	}

	notFoundReq := httptest.NewRequest(http.MethodGet, "/api/translations/"+created.TranslationID, nil)
	notFoundReq.Header.Set("Cookie", sessionCookie)
	notFoundRes := httptest.NewRecorder()
	router.ServeHTTP(notFoundRes, notFoundReq)
	if notFoundRes.Code != http.StatusNotFound {
		t.Fatalf("expected detail after delete 404, got %d", notFoundRes.Code)
	}
}

func TestTranslationSSEFlow(t *testing.T) {
	cfg := newTestConfig(t)
	router := httprouter.NewRouter(cfg)
	sessionCookie := loginAndGetSessionCookie(t, router, cfg.AppPassword)

	reqBody := map[string]string{
		"input_text":  "你好世界",
		"source_type": "text",
	}
	payload, _ := json.Marshal(reqBody)

	createReq := httptest.NewRequest(http.MethodPost, "/api/translations", bytes.NewReader(payload))
	createReq.Header.Set("Cookie", sessionCookie)
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	router.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusOK {
		t.Fatalf("expected create status 200, got %d", createRes.Code)
	}

	var created struct {
		TranslationID string `json:"translation_id"`
	}
	if err := json.NewDecoder(createRes.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.TranslationID == "" {
		t.Fatal("expected translation_id")
	}

	// Give the worker a brief chance to make progress; stream must still replay full lifecycle.
	time.Sleep(30 * time.Millisecond)

	streamReq := httptest.NewRequest(http.MethodGet, "/api/translations/"+created.TranslationID+"/stream", nil)
	streamReq.Header.Set("Cookie", sessionCookie)
	streamRes := httptest.NewRecorder()
	router.ServeHTTP(streamRes, streamReq)

	if streamRes.Code != http.StatusOK {
		t.Fatalf("expected stream status 200, got %d", streamRes.Code)
	}
	if got := streamRes.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("expected event-stream content type, got %q", got)
	}

	dataLines := extractSSEDataLines(streamRes.Body.String())
	if len(dataLines) < 1 {
		t.Fatalf("expected at least one SSE event, got %d", len(dataLines))
	}

	var first map[string]any
	if err := json.Unmarshal([]byte(dataLines[0]), &first); err != nil {
		t.Fatalf("invalid first SSE json: %v", err)
	}
	if first["type"] != "start" && first["type"] != "error" {
		t.Fatalf("expected first SSE event to be start or error, got %v", first["type"])
	}
	if first["type"] == "start" && first["translation_id"] != created.TranslationID {
		t.Fatalf("expected matching translation_id, got %v", first["translation_id"])
	}

	var last map[string]any
	if err := json.Unmarshal([]byte(dataLines[len(dataLines)-1]), &last); err != nil {
		t.Fatalf("invalid last SSE json: %v", err)
	}
	if last["type"] != "complete" && last["type"] != "error" {
		t.Fatalf("expected last SSE event to be complete or error, got %v", last["type"])
	}
}

func TestCoreAPIPersistenceFlow(t *testing.T) {
	cfg := newTestConfig(t)
	router := httprouter.NewRouter(cfg)
	sessionCookie := loginAndGetSessionCookie(t, router, cfg.AppPassword)

	textPayload, _ := json.Marshal(map[string]any{
		"raw_text":    "你好世界",
		"source_type": "text",
		"metadata":    map[string]any{"source": "test"},
	})
	createTextReq := httptest.NewRequest(http.MethodPost, "/api/texts", bytes.NewReader(textPayload))
	createTextReq.Header.Set("Cookie", sessionCookie)
	createTextReq.Header.Set("Content-Type", "application/json")
	createTextRes := httptest.NewRecorder()
	router.ServeHTTP(createTextRes, createTextReq)
	if createTextRes.Code != http.StatusOK {
		t.Fatalf("expected create text status 200, got %d", createTextRes.Code)
	}
	var createTextOut struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(createTextRes.Body).Decode(&createTextOut); err != nil || createTextOut.ID == "" {
		t.Fatalf("expected text id, err=%v", err)
	}

	getTextReq := httptest.NewRequest(http.MethodGet, "/api/texts/"+createTextOut.ID, nil)
	getTextReq.Header.Set("Cookie", sessionCookie)
	getTextRes := httptest.NewRecorder()
	router.ServeHTTP(getTextRes, getTextReq)
	if getTextRes.Code != http.StatusOK {
		t.Fatalf("expected get text status 200, got %d", getTextRes.Code)
	}

	saveVocabPayload, _ := json.Marshal(map[string]any{
		"headword": "你好",
		"pinyin":   "ni hao",
		"english":  "hello",
		"text_id":  createTextOut.ID,
		"status":   "learning",
	})
	saveVocabReq := httptest.NewRequest(http.MethodPost, "/api/vocab/save", bytes.NewReader(saveVocabPayload))
	saveVocabReq.Header.Set("Cookie", sessionCookie)
	saveVocabReq.Header.Set("Content-Type", "application/json")
	saveVocabRes := httptest.NewRecorder()
	router.ServeHTTP(saveVocabRes, saveVocabReq)
	if saveVocabRes.Code != http.StatusOK {
		t.Fatalf("expected save vocab status 200, got %d", saveVocabRes.Code)
	}
	var saveVocabOut struct {
		VocabItemID string `json:"vocab_item_id"`
	}
	if err := json.NewDecoder(saveVocabRes.Body).Decode(&saveVocabOut); err != nil || saveVocabOut.VocabItemID == "" {
		t.Fatalf("expected vocab_item_id, err=%v", err)
	}

	lookupPayload, _ := json.Marshal(map[string]any{"vocab_item_id": saveVocabOut.VocabItemID})
	lookupReq := httptest.NewRequest(http.MethodPost, "/api/vocab/lookup", bytes.NewReader(lookupPayload))
	lookupReq.Header.Set("Cookie", sessionCookie)
	lookupReq.Header.Set("Content-Type", "application/json")
	lookupRes := httptest.NewRecorder()
	router.ServeHTTP(lookupRes, lookupReq)
	if lookupRes.Code != http.StatusOK {
		t.Fatalf("expected lookup status 200, got %d", lookupRes.Code)
	}

	reviewQueueReq := httptest.NewRequest(http.MethodGet, "/api/review/queue", nil)
	reviewQueueReq.Header.Set("Cookie", sessionCookie)
	reviewQueueRes := httptest.NewRecorder()
	router.ServeHTTP(reviewQueueRes, reviewQueueReq)
	if reviewQueueRes.Code != http.StatusOK {
		t.Fatalf("expected review queue status 200, got %d", reviewQueueRes.Code)
	}

	translateBatchPayload, _ := json.Marshal(map[string]any{
		"segments": []string{"你", "好"},
	})
	translateBatchReq := httptest.NewRequest(http.MethodPost, "/api/segments/translate-batch", bytes.NewReader(translateBatchPayload))
	translateBatchReq.Header.Set("Cookie", sessionCookie)
	translateBatchReq.Header.Set("Content-Type", "application/json")
	translateBatchRes := httptest.NewRecorder()
	router.ServeHTTP(translateBatchRes, translateBatchReq)
	if translateBatchRes.Code != http.StatusOK {
		t.Fatalf("expected translate-batch status 200, got %d", translateBatchRes.Code)
	}
}

func TestAdminAndOCRContracts(t *testing.T) {
	cfg := newTestConfig(t)
	router := httprouter.NewRouter(cfg)
	sessionCookie := loginAndGetSessionCookie(t, router, cfg.AppPassword)

	getProfileReq := httptest.NewRequest(http.MethodGet, "/api/admin/profile", nil)
	getProfileReq.Header.Set("Cookie", sessionCookie)
	getProfileRes := httptest.NewRecorder()
	router.ServeHTTP(getProfileRes, getProfileReq)
	if getProfileRes.Code != http.StatusOK {
		t.Fatalf("expected profile status 200, got %d", getProfileRes.Code)
	}

	updateProfilePayload, _ := json.Marshal(map[string]string{
		"name":     "A",
		"email":    "a@example.com",
		"language": "zh-CN",
	})
	updateProfileReq := httptest.NewRequest(http.MethodPost, "/api/admin/profile", bytes.NewReader(updateProfilePayload))
	updateProfileReq.Header.Set("Cookie", sessionCookie)
	updateProfileReq.Header.Set("Content-Type", "application/json")
	updateProfileRes := httptest.NewRecorder()
	router.ServeHTTP(updateProfileRes, updateProfileReq)
	if updateProfileRes.Code != http.StatusOK {
		t.Fatalf("expected update profile status 200, got %d", updateProfileRes.Code)
	}

	exportReq := httptest.NewRequest(http.MethodGet, "/api/admin/progress/export", nil)
	exportReq.Header.Set("Cookie", sessionCookie)
	exportRes := httptest.NewRecorder()
	router.ServeHTTP(exportRes, exportReq)
	if exportRes.Code != http.StatusOK {
		t.Fatalf("expected export progress status 200, got %d", exportRes.Code)
	}

	// OCR contract: multipart image accepted and returns text payload.
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", "test.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte("fake-image-bytes"))
	_ = writer.Close()

	ocrReq := httptest.NewRequest(http.MethodPost, "/api/extract-text", &body)
	ocrReq.Header.Set("Cookie", sessionCookie)
	ocrReq.Header.Set("Content-Type", writer.FormDataContentType())
	ocrRes := httptest.NewRecorder()
	router.ServeHTTP(ocrRes, ocrReq)
	if ocrRes.Code != http.StatusOK {
		t.Fatalf("expected extract-text status 200, got %d", ocrRes.Code)
	}
}

func TestTranslationSSENotFound(t *testing.T) {
	cfg := newTestConfig(t)
	router := httprouter.NewRouter(cfg)
	sessionCookie := loginAndGetSessionCookie(t, router, cfg.AppPassword)

	streamReq := httptest.NewRequest(http.MethodGet, "/api/translations/not-found/stream", nil)
	streamReq.Header.Set("Cookie", sessionCookie)
	streamRes := httptest.NewRecorder()
	router.ServeHTTP(streamRes, streamReq)

	if streamRes.Code != http.StatusOK {
		t.Fatalf("expected stream status 200, got %d", streamRes.Code)
	}
	dataLines := extractSSEDataLines(streamRes.Body.String())
	if len(dataLines) == 0 {
		t.Fatal("expected SSE error event")
	}
	var event map[string]any
	if err := json.Unmarshal([]byte(dataLines[0]), &event); err != nil {
		t.Fatalf("invalid SSE json: %v", err)
	}
	if event["type"] != "error" {
		t.Fatalf("expected error event, got %v", event["type"])
	}
}

func TestAuthBehaviorParity(t *testing.T) {
	cfg := newTestConfig(t)
	router := httprouter.NewRouter(cfg)

	t.Run("api unauthenticated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/texts/1", nil)
		req.Header.Set("Accept", "application/json")
		res := httptest.NewRecorder()

		router.ServeHTTP(res, req)

		if res.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", res.Code)
		}
		body, _ := io.ReadAll(res.Result().Body)
		if strings.TrimSpace(string(body)) != `{"detail":"Not authenticated"}` {
			t.Fatalf("unexpected body: %q", string(body))
		}
	})
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
