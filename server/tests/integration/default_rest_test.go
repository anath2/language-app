package integration_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/anath2/language-app/internal/migrations"
)

func TestAuthAndSessionFlow(t *testing.T) {
	cfg := newLocalConfig(t)
	router := newRouterWithConfig(cfg)

	req, err := http.NewRequest(http.MethodGet, "/api/texts/1", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	rec := doRawRequest(router, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated status 401, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"detail":"Not authenticated"}` {
		t.Fatalf("unexpected unauthenticated body: %q", rec.Body.String())
	}

	badLogin := doJSONRequest(t, router, http.MethodPost, "/api/auth/login", map[string]string{
		"password": "wrong-password",
	}, "")
	if badLogin.Code != http.StatusUnauthorized {
		t.Fatalf("expected bad login 401, got %d", badLogin.Code)
	}

	sessionCookie := loginSessionCookie(t, router, cfg.AppPassword)
	logout := doJSONRequest(t, router, http.MethodPost, "/api/auth/logout", nil, sessionCookie)
	if logout.Code != http.StatusOK {
		t.Fatalf("expected logout 200, got %d", logout.Code)
	}
}

func TestCoreJSONPersistenceFlow(t *testing.T) {
	cfg := newLocalConfig(t)
	router := newRouterWithConfig(cfg)
	sessionCookie := loginSessionCookie(t, router, cfg.AppPassword)

	createText := doJSONRequest(t, router, http.MethodPost, "/api/texts", map[string]any{
		"raw_text":    "人工智能改变世界",
		"source_type": "text",
		"metadata":    map[string]any{"source": "integration"},
	}, sessionCookie)
	if createText.Code != http.StatusOK {
		t.Fatalf("expected create text 200, got %d", createText.Code)
	}
	var textResp struct {
		ID string `json:"id"`
	}
	decodeBodyJSON(t, createText, &textResp)
	if textResp.ID == "" {
		t.Fatal("expected text id in create response")
	}

	getText := doJSONRequest(t, router, http.MethodGet, "/api/texts/"+textResp.ID, nil, sessionCookie)
	if getText.Code != http.StatusOK {
		t.Fatalf("expected get text 200, got %d", getText.Code)
	}

	saveVocab := doJSONRequest(t, router, http.MethodPost, "/api/vocab/save", map[string]any{
		"headword": "你好",
		"pinyin":   "ni hao",
		"english":  "hello",
		"text_id":  textResp.ID,
		"status":   "learning",
	}, sessionCookie)
	if saveVocab.Code != http.StatusOK {
		t.Fatalf("expected save vocab 200, got %d", saveVocab.Code)
	}
	var vocabResp struct {
		VocabItemID string `json:"vocab_item_id"`
	}
	decodeBodyJSON(t, saveVocab, &vocabResp)
	if vocabResp.VocabItemID == "" {
		t.Fatal("expected vocab_item_id in save vocab response")
	}

	lookup := doJSONRequest(t, router, http.MethodPost, "/api/vocab/lookup", map[string]any{
		"vocab_item_id": vocabResp.VocabItemID,
	}, sessionCookie)
	if lookup.Code != http.StatusOK {
		t.Fatalf("expected vocab lookup 200, got %d", lookup.Code)
	}

	srsInfo := doJSONRequest(t, router, http.MethodGet, "/api/vocab/srs-info?headwords=%E4%BD%A0%E5%A5%BD", nil, sessionCookie)
	if srsInfo.Code != http.StatusOK {
		t.Fatalf("expected vocab srs-info 200, got %d", srsInfo.Code)
	}

	reviewQueue := doJSONRequest(t, router, http.MethodGet, "/api/review/words/queue", nil, sessionCookie)
	if reviewQueue.Code != http.StatusOK {
		t.Fatalf("expected review queue 200, got %d", reviewQueue.Code)
	}

	reviewCount := doJSONRequest(t, router, http.MethodGet, "/api/review/words/count", nil, sessionCookie)
	if reviewCount.Code != http.StatusOK {
		t.Fatalf("expected review count 200, got %d", reviewCount.Code)
	}
}

func TestAdminJSONEndpoints(t *testing.T) {
	cfg := newLocalConfig(t)
	router := newRouterWithConfig(cfg)
	sessionCookie := loginSessionCookie(t, router, cfg.AppPassword)

	getProfile := doJSONRequest(t, router, http.MethodGet, "/api/admin/profile", nil, sessionCookie)
	if getProfile.Code != http.StatusOK {
		t.Fatalf("expected get profile 200, got %d", getProfile.Code)
	}

	updateProfile := doJSONRequest(t, router, http.MethodPost, "/api/admin/profile", map[string]string{
		"name":     "Integration User",
		"email":    "integration@example.com",
		"language": "zh-CN",
	}, sessionCookie)
	if updateProfile.Code != http.StatusOK {
		t.Fatalf("expected update profile 200, got %d", updateProfile.Code)
	}

	export := doJSONRequest(t, router, http.MethodGet, "/api/admin/progress/export", nil, sessionCookie)
	if export.Code != http.StatusOK {
		t.Fatalf("expected export progress 200, got %d", export.Code)
	}
}

func TestMigrationIdempotencyAndRestartSmoke(t *testing.T) {
	cfg := newLocalConfig(t)

	if err := migrations.RunUp(cfg.TranslationDBPath, cfg.MigrationsDir); err != nil {
		t.Fatalf("first migration run failed: %v", err)
	}
	if err := migrations.RunUp(cfg.TranslationDBPath, cfg.MigrationsDir); err != nil {
		t.Fatalf("second migration run failed: %v", err)
	}

	router := newRouterWithConfig(cfg)
	loginSessionCookie(t, router, cfg.AppPassword)

	health := doJSONRequest(t, router, http.MethodGet, "/health", nil, "")
	if health.Code != http.StatusOK {
		t.Fatalf("expected health 200 on first boot, got %d", health.Code)
	}

	// Simulate restart by building a fresh router against the same DB file.
	restartedRouter := newRouterWithConfig(cfg)
	loginSessionCookie(t, restartedRouter, cfg.AppPassword)

	restartHealth := doJSONRequest(t, restartedRouter, http.MethodGet, "/health", nil, "")
	if restartHealth.Code != http.StatusOK {
		t.Fatalf("expected health 200 after restart, got %d", restartHealth.Code)
	}
}
