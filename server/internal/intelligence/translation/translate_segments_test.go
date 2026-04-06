package translation

import (
	"context"
	"testing"
)

func TestTranslateSegments_SkipsNonCJK(t *testing.T) {
	t.Parallel()
	provider := &DSPyProvider{}
	results, err := provider.TranslateSegments(context.Background(), []string{"。", "!", " "}, "test", "test")
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

func TestParseBatchTranslations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
		want  int
	}{
		{"nil", nil, 0},
		{"empty string", "", 0},
		{"valid json string", `[{"pinyin":"nǐ hǎo","english":"hello"},{"pinyin":"shì jiè","english":"world"}]`, 2},
		{"slice of any", []any{
			map[string]any{"pinyin": "nǐ", "english": "you"},
			map[string]any{"pinyin": "hǎo", "english": "good"},
		}, 2},
		{"invalid json", "not json", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBatchTranslations(tt.input)
			if len(got) != tt.want {
				t.Fatalf("parseBatchTranslations(%v) returned %d items, want %d", tt.input, len(got), tt.want)
			}
		})
	}
}
