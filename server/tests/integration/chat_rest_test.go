package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/anath2/language-app/internal/config"
	"github.com/anath2/language-app/internal/http/handlers"
	"github.com/anath2/language-app/internal/intelligence"
	"github.com/anath2/language-app/internal/queue"
	"github.com/anath2/language-app/internal/translation"
)

type mockTranslationProvider struct{}

func (m mockTranslationProvider) Segment(_ context.Context, text string) ([]string, error) {
	return []string{text}, nil
}

func (m mockTranslationProvider) TranslateSegments(_ context.Context, segments []string, _ string) ([]translation.SegmentResult, error) {
	out := make([]translation.SegmentResult, 0, len(segments))
	for _, seg := range segments {
		out = append(out, translation.SegmentResult{
			Segment: seg,
			Pinyin:  "",
			English: "mock-" + seg,
		})
	}
	return out, nil
}

func (m mockTranslationProvider) TranslateFull(_ context.Context, text string) (string, error) {
	return "mock full: " + text, nil
}

func (m mockTranslationProvider) LookupCharacter(_ string) (string, string, bool) {
	return "", "", false
}

type mockChatProvider struct{}

func (m mockChatProvider) ChatWithTranslationContext(_ context.Context, req intelligence.ChatWithTranslationRequest, onChunk func(string) error, _ func(string)) (intelligence.ChatResult, error) {
	reply := "mock answer: " + req.UserMessage
	if onChunk != nil {
		_ = onChunk("mock ")
		_ = onChunk("answer: ")
		_ = onChunk(req.UserMessage)
	}
	return intelligence.ChatResult{Content: reply}, nil
}

func overrideDepsWithMockProvider(t *testing.T, cfg config.Config) *translation.TranslationStore {
	t.Helper()
	db, err := translation.NewDB(cfg.TranslationDBPath)
	if err != nil {
		t.Fatalf("new db for override deps: %v", err)
	}
	translationStore := translation.NewTranslationStore(db)
	textEventStore := translation.NewTextEventStore(db)
	srsStore := translation.NewSRSStore(db)
	profileStore := translation.NewProfileStore(db)
	transProv := mockTranslationProvider{}
	chatProv := mockChatProvider{}
	manager := queue.NewManager(translationStore, transProv)
	handlers.ConfigureDependencies(translationStore, textEventStore, srsStore, profileStore, manager, transProv, chatProv)
	return translationStore
}

func TestTranslationChatSSELifecycleAndClear(t *testing.T) {
	cfg := newLocalConfig(t)
	router := newRouterWithConfig(cfg)
	store := overrideDepsWithMockProvider(t, cfg)
	sessionCookie := loginSessionCookie(t, router, cfg.AppPassword)

	tr, err := store.Create("人工智能改变世界", "text")
	if err != nil {
		t.Fatalf("create translation: %v", err)
	}
	if err := store.UpdateTranslationSegments(tr.ID, 0, []translation.SegmentResult{
		{Segment: "人工智能", Pinyin: "ren gong zhi neng", English: "artificial intelligence"},
		{Segment: "改变", Pinyin: "gai bian", English: "change"},
	}); err != nil {
		t.Fatalf("seed segments: %v", err)
	}

	createRes := doJSONRequest(t, router, http.MethodPost, "/api/translations/"+tr.ID+"/chat/new", map[string]any{
		"message":       "Explain this",
		"selected_text": "人工智能",
	}, sessionCookie)
	if createRes.Code != http.StatusOK {
		t.Fatalf("expected chat new 200, got %d body=%s", createRes.Code, createRes.Body.String())
	}

	lines := extractSSEDataLines(createRes.Body.String())
	if len(lines) < 4 {
		t.Fatalf("expected >=4 SSE events, got %d body=%s", len(lines), createRes.Body.String())
	}
	var gotStart, gotChunk, gotComplete bool
	for _, line := range lines {
		var evt map[string]any
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			t.Fatalf("decode sse payload: %v line=%s", err, line)
		}
		switch evt["type"] {
		case "start":
			gotStart = true
		case "chunk":
			gotChunk = true
		case "complete":
			gotComplete = true
		}
	}
	if !gotStart || !gotChunk || !gotComplete {
		t.Fatalf("expected start/chunk/complete events, got start=%v chunk=%v complete=%v", gotStart, gotChunk, gotComplete)
	}

	listRes := doJSONRequest(t, router, http.MethodGet, "/api/translations/"+tr.ID+"/chat/list", nil, sessionCookie)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected chat list 200, got %d", listRes.Code)
	}
	var listPayload struct {
		ChatID   string                    `json:"chat_id"`
		Messages []translation.ChatMessage `json:"messages"`
	}
	decodeBodyJSON(t, listRes, &listPayload)
	if listPayload.ChatID == "" {
		t.Fatal("expected chat_id in chat list response")
	}
	if len(listPayload.Messages) != 2 {
		t.Fatalf("expected 2 chat messages, got %d", len(listPayload.Messages))
	}

	clearRes := doJSONRequest(t, router, http.MethodPost, "/api/translations/"+tr.ID+"/chat/clear", map[string]any{}, sessionCookie)
	if clearRes.Code != http.StatusOK {
		t.Fatalf("expected clear chat 200, got %d", clearRes.Code)
	}
	listAfterClear := doJSONRequest(t, router, http.MethodGet, "/api/translations/"+tr.ID+"/chat/list", nil, sessionCookie)
	if listAfterClear.Code != http.StatusOK {
		t.Fatalf("expected chat list after clear 200, got %d", listAfterClear.Code)
	}
	decodeBodyJSON(t, listAfterClear, &listPayload)
	if len(listPayload.Messages) != 0 {
		t.Fatalf("expected no messages after clear, got %d", len(listPayload.Messages))
	}
}
