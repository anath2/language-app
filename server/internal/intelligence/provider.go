package intelligence

import (
	"context"

	"github.com/anath2/language-app/internal/translation"
)

type ChatSegmentContext struct {
	ID      string `json:"id"`
	Segment string `json:"segment"`
	Pinyin  string `json:"pinyin"`
	English string `json:"english"`
}

type ChatWithTranslationRequest struct {
	TranslationText string               `json:"translation_text"`
	UserMessage     string               `json:"user_message"`
	Selected        []ChatSegmentContext `json:"selected"`
	History         []translation.ChatMessage
}

// Provider defines the translation intelligence contract.
type Provider interface {
	Segment(ctx context.Context, text string) ([]string, error)
	TranslateSegments(ctx context.Context, segments []string, sentenceContext string) ([]translation.SegmentResult, error)
	TranslateFull(ctx context.Context, text string) (string, error)
	LookupCharacter(char string) (pinyin string, english string, found bool)
	ChatWithTranslationContext(ctx context.Context, req ChatWithTranslationRequest, onChunk func(string) error) (string, error)
}
