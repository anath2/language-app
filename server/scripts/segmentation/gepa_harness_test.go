package segmentation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCasesFromCSV_DefaultDataset(t *testing.T) {
	t.Parallel()

	cases, err := LoadDefaultCases()
	if err != nil {
		t.Fatalf("load default csv: %v", err)
	}
	if len(cases) < 20 {
		t.Fatalf("expected at least 20 rows, got %d", len(cases))
	}
	if strings.TrimSpace(cases[0].Text) == "" {
		t.Fatal("first case sentence should not be empty")
	}
	if len(cases[0].Expected) == 0 {
		t.Fatal("first case expected segments should not be empty")
	}
}

func TestLoadCasesFromCSV_InvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "bad.csv")
	payload := "id,sentence,expected_segments_json\nx1,我爱你。,not_json\n"
	if err := os.WriteFile(csvPath, []byte(payload), 0o644); err != nil {
		t.Fatalf("write temp csv: %v", err)
	}

	_, err := LoadCasesFromCSV(csvPath)
	if err == nil {
		t.Fatal("expected parse error for malformed expected_segments_json")
	}
}

func TestBuildGEPASentenceDataset_SentenceOnly(t *testing.T) {
	t.Parallel()

	corpus := []Case{
		{Name: "single", Text: "我喜欢中文。", Expected: []string{"我", "喜欢", "中文", "。"}},
		{Name: "single2", Text: "我们去图书馆。", Expected: []string{"我们", "去", "图书馆", "。"}},
	}

	ds, examples := BuildGEPASentenceDataset(corpus, 10)
	if ds == nil {
		t.Fatal("expected non-nil dataset")
	}
	if len(examples) != 2 {
		t.Fatalf("expected 2 examples, got %d", len(examples))
	}

	for i, ex := range examples {
		gotText, _ := ex.Inputs["text"].(string)
		if gotText != corpus[i].Text {
			t.Fatalf("example %d text mismatch: got %q want %q", i, gotText, corpus[i].Text)
		}
	}
}
