package translation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShouldSkipSegment(t *testing.T) {
	t.Parallel()

	cases := []struct {
		segment string
		skip    bool
	}{
		{segment: "", skip: true},
		{segment: "   ", skip: true},
		{segment: "!!!", skip: true},
		{segment: "abc", skip: true},
		{segment: "，。！？", skip: true},
		{segment: "你好", skip: false},
		{segment: "你！", skip: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.segment, func(t *testing.T) {
			t.Parallel()
			got := shouldSkipSegment(tc.segment)
			if got != tc.skip {
				t.Fatalf("shouldSkipSegment(%q)=%v want=%v", tc.segment, got, tc.skip)
			}
		})
	}
}

func TestNumberedPinyinToToneMarks(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{in: "ni3 hao3", want: "nǐ hǎo"},
		{in: "lu:4", want: "lǜ"},
		{in: "nv3", want: "nǚ"},
		{in: "shi4 jie4", want: "shì jiè"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got := numberedPinyinToToneMarks(tc.in)
			if got != tc.want {
				t.Fatalf("numberedPinyinToToneMarks(%q)=%q want=%q", tc.in, got, tc.want)
			}
		})
	}
}

func TestLoadCedictDictionary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "cedict_ts.u8")
	content := "# comment\n你好 你好 [ni3 hao3] /hello/hi/\n世界 世界 [shi4 jie4] /world/\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp cedict: %v", err)
	}

	dict, err := loadCedictDictionary(path)
	if err != nil {
		t.Fatalf("loadCedictDictionary error: %v", err)
	}

	entries, ok := dict.Lookup("你好")
	if !ok || len(entries) == 0 {
		t.Fatalf("expected 你好 entry")
	}
	if entries[0].Pinyin != "nǐ hǎo" {
		t.Fatalf("unexpected pinyin: %q", entries[0].Pinyin)
	}
	if entries[0].Definition != "hello / hi" {
		t.Fatalf("unexpected definition: %q", entries[0].Definition)
	}

	// LookupFirst should return the same first entry.
	first, ok := dict.LookupFirst("你好")
	if !ok {
		t.Fatalf("expected LookupFirst to find 你好")
	}
	if first.Pinyin != entries[0].Pinyin {
		t.Fatalf("LookupFirst pinyin mismatch: %q vs %q", first.Pinyin, entries[0].Pinyin)
	}
}

func TestMultiEntryLoading(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "cedict_ts.u8")
	// 行 has two distinct readings: háng (row) and xíng (walk).
	content := "" +
		"行 行 [hang2] /row/line/\n" +
		"行 行 [xing2] /to walk/to go/\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp cedict: %v", err)
	}

	dict, err := loadCedictDictionary(path)
	if err != nil {
		t.Fatalf("loadCedictDictionary error: %v", err)
	}

	entries, ok := dict.Lookup("行")
	if !ok {
		t.Fatalf("expected 行 entries")
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries for 行, got %d", len(entries))
	}
	if entries[0].PinyinNumbered != "hang2" {
		t.Fatalf("expected first entry numbered pinyin hang2, got %q", entries[0].PinyinNumbered)
	}
	if entries[1].PinyinNumbered != "xing2" {
		t.Fatalf("expected second entry numbered pinyin xing2, got %q", entries[1].PinyinNumbered)
	}
}

func TestIsCharAmbiguous(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "cedict_ts.u8")
	content := "" +
		// 行 has two genuinely different readings.
		"行 行 [hang2] /row/\n" +
		"行 行 [xing2] /to walk/\n" +
		// 好 has hao3 and hao4 — same base syllable, different tones → not ambiguous.
		"好 好 [hao3] /good/\n" +
		"好 好 [hao4] /to like/\n" +
		// 吗 has ma3 (what) and ma5 (particle) — particle reading makes it unambiguous.
		"吗 吗 [ma3] /what/\n" +
		"吗 吗 [ma5] /question particle/\n" +
		// 的 has only a tone-5 reading — single entry, not ambiguous.
		"的 的 [de5] /possessive particle/\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp cedict: %v", err)
	}

	dict, err := loadCedictDictionary(path)
	if err != nil {
		t.Fatalf("loadCedictDictionary error: %v", err)
	}

	cases := []struct {
		char      rune
		ambiguous bool
	}{
		{'行', true},  // hang2 vs xing2 — genuinely different bases
		{'好', false}, // hao3 vs hao4 — same base syllable, only tone differs
		{'吗', false}, // ma3 + ma5 — tone-5 excluded, only ma3 left → not ambiguous
		{'的', false}, // single entry
		{'X', false}, // not in dict
	}

	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.char), func(t *testing.T) {
			t.Parallel()
			got := dict.IsCharAmbiguous(tc.char)
			if got != tc.ambiguous {
				t.Fatalf("IsCharAmbiguous(%c)=%v want=%v", tc.char, got, tc.ambiguous)
			}
		})
	}
}

func TestComposeSegmentPinyin(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "cedict_ts.u8")
	content := "" +
		"你好 你好 [ni3 hao3] /hello/\n" +
		"你 你 [ni3] /you/\n" +
		"好 好 [hao3] /good/\n" +
		"好 好 [hao4] /to like/\n" +
		"吗 吗 [ma3] /what/\n" +
		"吗 吗 [ma5] /question particle/\n" +
		"行 行 [hang2] /row/\n" +
		"行 行 [xing2] /to walk/\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp cedict: %v", err)
	}

	dict, err := loadCedictDictionary(path)
	if err != nil {
		t.Fatalf("loadCedictDictionary error: %v", err)
	}

	cases := []struct {
		name    string
		segment string
		wantPy  string
		wantOK  bool
	}{
		{
			name:    "word-level unambiguous",
			segment: "你好",
			wantPy:  "nǐ hǎo",
			wantOK:  true,
		},
		{
			name:    "ambiguous char in segment",
			segment: "行人",
			wantPy:  "",
			wantOK:  false,
		},
		{
			name:    "particle char — uses tone-5 preferred",
			segment: "你吗",
			wantPy:  "nǐ ma",
			wantOK:  true,
		},
		{
			name:    "missing char falls back to false",
			segment: "你龘",
			wantPy:  "",
			wantOK:  false,
		},
		{
			name:    "nil dict",
			segment: "你好",
			wantPy:  "",
			wantOK:  false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := dict
			if tc.name == "nil dict" {
				d = nil
			}
			py, ok := d.ComposeSegmentPinyin(tc.segment)
			if ok != tc.wantOK {
				t.Fatalf("ComposeSegmentPinyin(%q) ok=%v want=%v", tc.segment, ok, tc.wantOK)
			}
			if py != tc.wantPy {
				t.Fatalf("ComposeSegmentPinyin(%q)=%q want=%q", tc.segment, py, tc.wantPy)
			}
		})
	}
}
