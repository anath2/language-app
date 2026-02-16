package intelligence

import (
	"context"

	"github.com/anath2/language-app/internal/translation"
)

// Provider defines the translation intelligence contract.
type Provider interface {
	Segment(ctx context.Context, text string) ([]string, error)
	TranslateSegments(ctx context.Context, segments []string, sentenceContext string) ([]translation.SegmentResult, error)
	TranslateFull(ctx context.Context, text string) (string, error)
}
