package intelligence

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

	entry, ok := dict.Lookup("你好")
	if !ok {
		t.Fatalf("expected 你好 entry")
	}
	if entry.Pinyin != "nǐ hǎo" {
		t.Fatalf("unexpected pinyin: %q", entry.Pinyin)
	}
	if entry.Definition != "hello / hi" {
		t.Fatalf("unexpected definition: %q", entry.Definition)
	}
}
