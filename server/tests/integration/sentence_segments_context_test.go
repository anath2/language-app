package integration_test

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/anath2/language-app/internal/config"
	"github.com/anath2/language-app/internal/http/handlers"
	"github.com/anath2/language-app/internal/intelligence"
	"github.com/anath2/language-app/internal/queue"
	"github.com/anath2/language-app/internal/translation"
)

type captureSentenceContextProvider struct {
	lastSegments []string
	lastSentence string
	lastFullText string
}

func (p *captureSentenceContextProvider) Segment(_ context.Context, text string) ([]string, error) {
	return []string{text}, nil
}

func (p *captureSentenceContextProvider) TranslateSentenceSegments(_ context.Context, segments []string, sentence string, fullText string) ([]translation.SegmentResult, error) {
	p.lastSegments = append([]string(nil), segments...)
	p.lastSentence = sentence
	p.lastFullText = fullText

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

func (p *captureSentenceContextProvider) TranslateFull(_ context.Context, text string) (string, error) {
	return "mock full: " + text, nil
}

func overrideDepsWithTranslationProvider(t *testing.T, cfg config.Config, transProv intelligence.TranslationProvider) {
	t.Helper()

	db, err := translation.NewDB(cfg.TranslationDBPath)
	if err != nil {
		t.Fatalf("new db for override deps: %v", err)
	}
	translationStore := translation.NewTranslationStore(db)
	chatStore := translation.NewChatStore(db)
	srsStore := translation.NewSRSStore(db)
	profileStore := translation.NewProfileStore(db)
	manager := queue.NewManager(translationStore, transProv)
	handlers.ConfigureDependencies(translationStore, chatStore, srsStore, profileStore, manager, transProv, mockChatProvider{})
}

func TestTranslateSentenceSegmentsUsesDistinctSentenceAndFullText(t *testing.T) {
	cfg := newLocalConfig(t)
	router := newRouterWithConfig(cfg)
	provider := &captureSentenceContextProvider{}
	overrideDepsWithTranslationProvider(t, cfg, provider)
	sessionCookie := loginSessionCookie(t, router, cfg.AppPassword)

	segments := []string{"人工智能", "改变", "世界"}
	fullText := "人工智能改变世界。科技正在进步。"
	res := doJSONRequest(t, router, http.MethodPost, "/api/translations/sentence-segments/translate", map[string]any{
		"segments":  segments,
		"full_text": fullText,
	}, sessionCookie)
	if res.Code != http.StatusOK {
		t.Fatalf("expected translate-sentence-segments 200, got %d: %s", res.Code, res.Body.String())
	}

	if !reflect.DeepEqual(provider.lastSegments, segments) {
		t.Fatalf("expected provider segments %v, got %v", segments, provider.lastSegments)
	}

	wantSentence := strings.Join(segments, "")
	if provider.lastSentence != wantSentence {
		t.Fatalf("expected provider sentence %q, got %q", wantSentence, provider.lastSentence)
	}

	if provider.lastFullText != fullText {
		t.Fatalf("expected provider fullText %q, got %q", fullText, provider.lastFullText)
	}
}
