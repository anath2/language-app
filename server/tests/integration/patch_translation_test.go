package integration_test

import (
	"net/http"
	"testing"
)

func TestPatchTranslationSource(t *testing.T) {
	cfg := newLocalConfig(t)
	router := newRouterWithConfig(cfg)
	sessionCookie := loginSessionCookie(t, router, cfg.AppPassword)

	// Create a translation with two sentences.
	createRes := doJSONRequest(t, router, http.MethodPost, "/api/translations", map[string]any{
		"input_text":  "今天天气很好。明天会下雨。",
		"source_type": "text",
	}, sessionCookie)
	if createRes.Code != http.StatusOK {
		t.Fatalf("expected create translation 200, got %d: %s", createRes.Code, createRes.Body.String())
	}
	var createResp struct {
		TranslationID string `json:"translation_id"`
	}
	decodeBodyJSON(t, createRes, &createResp)
	if createResp.TranslationID == "" {
		t.Fatal("expected translation_id in create response")
	}
	id := createResp.TranslationID

	// PATCH 404 for non-existent ID.
	notFound := doJSONRequest(t, router, http.MethodPatch, "/api/translations/nonexistent", map[string]any{
		"input_text": "你好。",
	}, sessionCookie)
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("expected PATCH non-existent 404, got %d", notFound.Code)
	}

	// PATCH with an appended sentence — all paragraphs are new (no hashes stored yet).
	patchRes := doJSONRequest(t, router, http.MethodPatch, "/api/translations/"+id, map[string]any{
		"input_text": "今天天气很好。明天会下雨。后天是晴天。",
	}, sessionCookie)
	if patchRes.Code != http.StatusOK {
		t.Fatalf("expected PATCH 200, got %d: %s", patchRes.Code, patchRes.Body.String())
	}
	var patchResp struct {
		Status           string `json:"status"`
		SentencesChanged int    `json:"sentences_changed"`
	}
	decodeBodyJSON(t, patchRes, &patchResp)
	if patchResp.SentencesChanged == 0 {
		t.Fatal("expected sentences_changed > 0 after appending a sentence")
	}
	if patchResp.Status != "pending" {
		t.Fatalf("expected status 'pending', got %q", patchResp.Status)
	}

	// PATCH with the exact same text — hashes are now stored, so 0 changes.
	patchSameRes := doJSONRequest(t, router, http.MethodPatch, "/api/translations/"+id, map[string]any{
		"input_text": "今天天气很好。明天会下雨。后天是晴天。",
	}, sessionCookie)
	if patchSameRes.Code != http.StatusOK {
		t.Fatalf("expected PATCH same text 200, got %d: %s", patchSameRes.Code, patchSameRes.Body.String())
	}
	var patchSameResp struct {
		Status           string `json:"status"`
		SentencesChanged int    `json:"sentences_changed"`
	}
	decodeBodyJSON(t, patchSameRes, &patchSameResp)
	if patchSameResp.SentencesChanged != 0 {
		t.Fatalf("expected sentences_changed == 0 for identical text, got %d", patchSameResp.SentencesChanged)
	}
	if patchSameResp.Status != "completed" {
		t.Fatalf("expected status 'completed' when no changes, got %q", patchSameResp.Status)
	}

	// PATCH with empty input_text — should 400.
	badReq := doJSONRequest(t, router, http.MethodPatch, "/api/translations/"+id, map[string]any{
		"input_text": "   ",
	}, sessionCookie)
	if badReq.Code != http.StatusBadRequest {
		t.Fatalf("expected PATCH empty input 400, got %d", badReq.Code)
	}
}
