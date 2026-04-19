package translation

import (
	"context"
	"testing"
)

func TestTranslateSentenceSegments_SkipsNonCJK(t *testing.T) {
	t.Parallel()
	provider := &Provider{}
	results, err := provider.TranslateSentenceSegments(context.Background(), []string{"。", "!", " "}, "test", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Pinyin != "" || r.English != "" {
			t.Fatalf("result[%d] should have empty pinyin/english, got pinyin=%q english=%q", i, r.Pinyin, r.English)
		}
	}
}
