package integration_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/anath2/language-app/internal/translation"
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

func TestUpstreamChatReviewCard(t *testing.T) {
	requireUpstream(t)

	cfg := newUpstreamConfig(t)
	router := newRouterWithConfig(cfg)
	sessionCookie := loginSessionCookie(t, router, cfg.AppPassword)

	createRes := doJSONRequest(t, router, http.MethodPost, "/api/translations", map[string]any{
		"input_text":  "我每天学习中文。",
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

	time.Sleep(100 * time.Millisecond)

	chatRes := doJSONRequest(t, router, http.MethodPost, "/api/translations/"+created.TranslationID+"/chat/new", map[string]any{
		"message": "Create a practice sentence using 学习",
	}, sessionCookie)
	if chatRes.Code != http.StatusOK {
		t.Fatalf("expected chat new 200, got %d: %s", chatRes.Code, chatRes.Body.String())
	}

	dataLines := extractSSEDataLines(chatRes.Body.String())
	if len(dataLines) == 0 {
		t.Fatal("expected at least one SSE data event")
	}

	var completeEvt map[string]any
	for _, line := range dataLines {
		var evt map[string]any
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}
		if evt["type"] == "complete" {
			completeEvt = evt
		}
	}
	if completeEvt == nil {
		t.Fatalf("expected a complete SSE event; got lines: %v", dataLines)
	}

	reviewCardRaw, hasCard := completeEvt["review_card"]
	if !hasCard || reviewCardRaw == nil {
		t.Fatal("expected review_card in complete event")
	}
	cardBytes, _ := json.Marshal(reviewCardRaw)
	var card translation.ChatReviewCard
	if err := json.Unmarshal(cardBytes, &card); err != nil {
		t.Fatalf("decode review_card: %v", err)
	}
	if strings.TrimSpace(card.ChineseText) == "" {
		t.Fatal("expected non-empty chinese_text in review card")
	}
	if strings.TrimSpace(card.Pinyin) == "" {
		t.Fatal("expected non-empty pinyin in review card")
	}
	if strings.TrimSpace(card.English) == "" {
		t.Fatal("expected non-empty english in review card")
	}
	if card.Status != "pending" {
		t.Fatalf("expected review card status pending, got %q", card.Status)
	}

	// Confirm card is persisted via list endpoint.
	msgID, _ := completeEvt["message_id"].(string)
	if msgID == "" {
		t.Fatal("expected message_id in complete event")
	}
	listRes := doJSONRequest(t, router, http.MethodGet, "/api/translations/"+created.TranslationID+"/chat/list", nil, sessionCookie)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected chat list 200, got %d", listRes.Code)
	}
	var listPayload struct {
		Messages []translation.ChatMessage `json:"messages"`
	}
	decodeBodyJSON(t, listRes, &listPayload)
	var found bool
	for _, msg := range listPayload.Messages {
		if msg.ID == msgID {
			if msg.ReviewCard == nil {
				t.Fatal("expected review_card on persisted ai message")
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("ai message %q not found in list", msgID)
	}
}

func TestUpstreamChatReviewCardAccept(t *testing.T) {
	requireUpstream(t)

	cfg := newUpstreamConfig(t)
	router := newRouterWithConfig(cfg)
	sessionCookie := loginSessionCookie(t, router, cfg.AppPassword)

	createRes := doJSONRequest(t, router, http.MethodPost, "/api/translations", map[string]any{
		"input_text":  "我喜欢读书。",
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
		t.Fatal("expected translation_id")
	}

	time.Sleep(100 * time.Millisecond)

	chatRes := doJSONRequest(t, router, http.MethodPost, "/api/translations/"+created.TranslationID+"/chat/new", map[string]any{
		"message": "Create a practice sentence using 读书",
	}, sessionCookie)
	if chatRes.Code != http.StatusOK {
		t.Fatalf("expected chat new 200, got %d: %s", chatRes.Code, chatRes.Body.String())
	}

	dataLines := extractSSEDataLines(chatRes.Body.String())
	var completeEvt map[string]any
	for _, line := range dataLines {
		var evt map[string]any
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}
		if evt["type"] == "complete" {
			completeEvt = evt
		}
	}
	if completeEvt == nil {
		t.Fatal("expected complete SSE event")
	}
	if _, hasCard := completeEvt["review_card"]; !hasCard {
		t.Fatal("expected review_card in complete event")
	}
	msgID, _ := completeEvt["message_id"].(string)
	if msgID == "" {
		t.Fatal("expected message_id in complete event")
	}

	acceptRes := doJSONRequest(t, router, http.MethodPost,
		"/api/translations/"+created.TranslationID+"/chat/messages/"+msgID+"/accept",
		map[string]any{}, sessionCookie)
	if acceptRes.Code != http.StatusOK {
		t.Fatalf("expected accept 200, got %d: %s", acceptRes.Code, acceptRes.Body.String())
	}
	var acceptOut struct {
		OK           bool `json:"ok"`
		Deduplicated bool `json:"deduplicated"`
	}
	decodeBodyJSON(t, acceptRes, &acceptOut)
	if !acceptOut.OK {
		t.Fatal("expected ok: true from accept")
	}

	// Confirm status updated to accepted.
	listRes := doJSONRequest(t, router, http.MethodGet, "/api/translations/"+created.TranslationID+"/chat/list", nil, sessionCookie)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected chat list 200, got %d", listRes.Code)
	}
	var listPayload struct {
		Messages []translation.ChatMessage `json:"messages"`
	}
	decodeBodyJSON(t, listRes, &listPayload)
	var found bool
	for _, msg := range listPayload.Messages {
		if msg.ID == msgID {
			if msg.ReviewCard == nil {
				t.Fatal("expected review_card on ai message after accept")
			}
			if msg.ReviewCard.Status != "accepted" {
				t.Fatalf("expected status accepted, got %q", msg.ReviewCard.Status)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("ai message %q not found in list", msgID)
	}

	// Word should appear in SRS review queue.
	queueRes := doJSONRequest(t, router, http.MethodGet, "/api/review/words/queue", nil, sessionCookie)
	if queueRes.Code != http.StatusOK {
		t.Fatalf("expected review queue 200, got %d", queueRes.Code)
	}
	var queueOut struct {
		Cards []struct {
			Headword string `json:"headword"`
		} `json:"cards"`
	}
	decodeBodyJSON(t, queueRes, &queueOut)
	// The accepted word should be present (new words are immediately due).
	if len(queueOut.Cards) == 0 {
		t.Fatal("expected at least one card in review queue after accepting review card")
	}
}
