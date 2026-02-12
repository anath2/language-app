package integration_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestUpstreamTranslateBatch(t *testing.T) {
	requireUpstream(t)

	cfg := newUpstreamConfig(t)
	router := newRouterWithConfig(cfg)
	sessionCookie := loginSessionCookie(t, router, cfg.AppPassword)

	res := doJSONRequest(t, router, http.MethodPost, "/api/segments/translate-batch", map[string]any{
		"segments": []string{"人工智能", "改变", "世界"},
		"context":  "人工智能改变世界",
	}, sessionCookie)
	if res.Code != http.StatusOK {
		t.Fatalf("expected translate-batch 200, got %d: %s", res.Code, res.Body.String())
	}

	var out struct {
		Translations []struct {
			Segment string `json:"segment"`
			Pinyin  string `json:"pinyin"`
			English string `json:"english"`
		} `json:"translations"`
	}
	decodeBodyJSON(t, res, &out)
	if len(out.Translations) != 3 {
		t.Fatalf("expected 3 translations, got %d", len(out.Translations))
	}
	for _, tr := range out.Translations {
		if strings.TrimSpace(tr.English) == "" {
			t.Fatalf("expected non-empty english translation for segment %q", tr.Segment)
		}
		// Placeholder output indicates fallback path was used after upstream failure.
		if strings.HasPrefix(strings.TrimSpace(tr.English), "translation_of_") {
			t.Fatalf("upstream fallback detected for segment %q (english=%q)", tr.Segment, tr.English)
		}
	}
}

func TestUpstreamTranslationLifecycleSSE(t *testing.T) {
	requireUpstream(t)

	cfg := newUpstreamConfig(t)
	router := newRouterWithConfig(cfg)
	sessionCookie := loginSessionCookie(t, router, cfg.AppPassword)

	createRes := doJSONRequest(t, router, http.MethodPost, "/api/translations", map[string]any{
		"input_text":  "人工智能改变世界",
		"source_type": "text",
	}, sessionCookie)
	if createRes.Code != http.StatusOK {
		t.Fatalf("expected create translation 200, got %d: %s", createRes.Code, createRes.Body.String())
	}

	var created struct {
		TranslationID string `json:"translation_id"`
	}
	decodeBodyJSON(t, createRes, &created)
	if created.TranslationID == "" {
		t.Fatal("expected translation_id in create response")
	}

	// Give the background worker time to make initial progress.
	time.Sleep(40 * time.Millisecond)

	stream := doJSONRequest(
		t,
		router,
		http.MethodGet,
		"/api/translations/"+created.TranslationID+"/stream",
		nil,
		sessionCookie,
	)
	if stream.Code != http.StatusOK {
		t.Fatalf("expected stream status 200, got %d", stream.Code)
	}
	if contentType := stream.Header().Get("Content-Type"); contentType == "" {
		t.Fatal("expected SSE content type header")
	}

	dataLines := extractSSEDataLines(stream.Body.String())
	if len(dataLines) == 0 {
		t.Fatal("expected at least one SSE data event")
	}

	var first map[string]any
	if err := json.Unmarshal([]byte(dataLines[0]), &first); err != nil {
		t.Fatalf("invalid first SSE event: %v", err)
	}
	if first["type"] != "start" {
		t.Fatalf("expected first SSE type start, got %v", first["type"])
	}

	progressCount := 0
	for _, line := range dataLines {
		var evt map[string]any
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			t.Fatalf("invalid SSE event: %v", err)
		}
		eventType, _ := evt["type"].(string)
		if eventType == "error" {
			t.Fatalf("unexpected SSE error event: %v", evt)
		}
		if eventType == "progress" {
			progressCount++
		}
	}
	if progressCount == 0 {
		t.Fatal("expected at least one SSE progress event")
	}

	var last map[string]any
	if err := json.Unmarshal([]byte(dataLines[len(dataLines)-1]), &last); err != nil {
		t.Fatalf("invalid last SSE event: %v", err)
	}
	if last["type"] != "complete" {
		t.Fatalf("expected last SSE type complete, got %v", last["type"])
	}
}
