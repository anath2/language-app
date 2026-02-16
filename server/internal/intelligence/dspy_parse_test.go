package intelligence

import (
	"testing"
)

func TestParseSegments(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  any
		expect []string
	}{
		{
			name:   "array of strings",
			input:  []any{"春节", "聚会", "，"},
			expect: []string{"春节", "聚会", "，"},
		},
		{
			name:   "filters metadata segment",
			input:  []any{"segments:", "春节", "聚会"},
			expect: []string{"春节", "聚会"},
		},
		{
			name:   "filters segments: with space",
			input:  []any{"segments: ", "你好"},
			expect: []string{"你好"},
		},
		{
			name:   "string with segments prefix",
			input:  `segments: ["春节", "聚会"]`,
			expect: []string{"春节", "聚会"},
		},
		{
			name:   "json object with segments key",
			input:  `{"segments": ["有", "哪些"]}`,
			expect: []string{"有", "哪些"},
		},
		{
			name:   "freeform text with array",
			input:  `Here are the segments: ["好的", "做法"]`,
			expect: []string{"好的", "做法"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			switch v := tc.input.(type) {
			case string:
				got = parseSegmentsString(v)
			default:
				got = parseSegments(tc.input)
			}
			if len(got) != len(tc.expect) {
				t.Fatalf("got %d segments %q, want %d %q", len(got), got, len(tc.expect), tc.expect)
			}
			for i := range got {
				if got[i] != tc.expect[i] {
					t.Fatalf("got[%d]=%q want %q", i, got[i], tc.expect[i])
				}
			}
		})
	}
}

func TestParseSegmentsFromResponse_NewlineFormat(t *testing.T) {
	// Model returns newline-separated segments instead of JSON (e.g. gemini with "Return only the segments array").
	input := "segments:\n如何\n评价\n《\n互联网\n已\n死\n，\nAgent\n永生\n》\n一\n文\n？"
	got := parseSegmentsFromResponse(input)
	expect := []string{"如何", "评价", "《", "互联网", "已", "死", "，", "Agent", "永生", "》", "一", "文", "？"}
	if len(got) != len(expect) {
		t.Fatalf("got %d segments %q, want %d %q", len(got), got, len(expect), expect)
	}
	for i := range got {
		if got[i] != expect[i] {
			t.Fatalf("got[%d]=%q want %q", i, got[i], expect[i])
		}
	}
}

func TestIsMetadataSegment(t *testing.T) {
	t.Parallel()

	metadata := []string{"segments", "segments:", "segments: ", "Segments:", "  segments  "}
	for _, s := range metadata {
		if !isMetadataSegment(s) {
			t.Errorf("isMetadataSegment(%q)=false, want true", s)
		}
	}

	real := []string{"春节", "你好", "segments 春节"} // segment text containing "segments" is kept
	for _, s := range real {
		if isMetadataSegment(s) {
			t.Errorf("isMetadataSegment(%q)=true, want false", s)
		}
	}
}
