package translation

import (
	"testing"
)

func TestParseSegmentsResult(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		want    []string
		wantErr bool
	}{
		{
			name:    "valid segments",
			content: `{"segments":["春节","聚会","，"]}`,
			want:    []string{"春节", "聚会", "，"},
		},
		{
			name:    "single segment",
			content: `{"segments":["你好世界"]}`,
			want:    []string{"你好世界"},
		},
		{
			name:    "invalid json",
			content: `not json`,
			wantErr: true,
		},
		{
			name:    "empty segments array",
			content: `{"segments":[]}`,
			wantErr: true,
		},
		{
			name:    "missing segments key",
			content: `{"words":["你","好"]}`,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseSegmentsResult(tc.content)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result=%v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d segments %q, want %d %q", len(got), got, len(tc.want), tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got[%d]=%q want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseBatchTranslationsResult(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		wantN   int
		wantErr bool
	}{
		{
			name:    "two translations",
			content: `{"translations":[{"pinyin":"nǐ hǎo","english":"hello"},{"pinyin":"shì jiè","english":"world"}]}`,
			wantN:   2,
		},
		{
			name:    "empty translations array",
			content: `{"translations":[]}`,
			wantN:   0,
		},
		{
			name:    "invalid json",
			content: `not json`,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseBatchTranslationsResult(tc.content)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tc.wantN {
				t.Fatalf("got %d translations, want %d", len(got), tc.wantN)
			}
		})
	}
}

func TestParseFullTranslationResult(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{
			name:    "valid translation",
			content: `{"translation":"Hello, world!"}`,
			want:    "Hello, world!",
		},
		{
			name:    "trims whitespace",
			content: `{"translation":"  Hello  "}`,
			want:    "Hello",
		},
		{
			name:    "empty translation",
			content: `{"translation":""}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			content: `not json`,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseFullTranslationResult(tc.content)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
