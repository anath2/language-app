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

func TestSplitCasesDeterministic(t *testing.T) {
	t.Parallel()

	cases := []Case{
		{Name: "a", Text: "我喜欢中文。", Expected: []string{"我", "喜欢", "中文", "。"}},
		{Name: "b", Text: "人工智能改变世界。", Expected: []string{"人工智能", "改变", "世界", "。"}},
		{Name: "c", Text: "我们去图书馆。", Expected: []string{"我们", "去", "图书馆", "。"}},
		{Name: "d", Text: "研究生命起源。", Expected: []string{"研究", "生命", "起源", "。"}},
	}

	train1, eval1 := SplitCasesDeterministic(cases, 0.75, 42, 0)
	train2, eval2 := SplitCasesDeterministic(cases, 0.75, 42, 0)
	if len(train1) != len(train2) || len(eval1) != len(eval2) {
		t.Fatal("deterministic split should produce same lengths for same seed")
	}
	for i := range train1 {
		if train1[i].Name != train2[i].Name {
			t.Fatalf("train split mismatch at %d: %s vs %s", i, train1[i].Name, train2[i].Name)
		}
	}
}

func TestEvaluatePromotionGate_Strict(t *testing.T) {
	t.Parallel()

	baseline := EvalSummary{ExactMatches: 4, TotalCases: 10, ReconstructionFail: 2, Errors: 1}
	compiledPass := EvalSummary{ExactMatches: 5, TotalCases: 10, ReconstructionFail: 2, Errors: 1}
	pass, reasons := EvaluatePromotionGate(baseline, compiledPass)
	if !pass || len(reasons) > 0 {
		t.Fatalf("expected pass, got pass=%v reasons=%v", pass, reasons)
	}

	compiledFail := EvalSummary{ExactMatches: 5, TotalCases: 10, ReconstructionFail: 3, Errors: 1}
	pass, reasons = EvaluatePromotionGate(baseline, compiledFail)
	if pass {
		t.Fatal("expected fail when reconstruction failures increase")
	}
}

func TestParseSegmentsFromResponse_NewlineFormat(t *testing.T) {
	t.Parallel()

	// Aligns with production: model returns newline-separated segments instead of JSON.
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

func TestSelectPromotionDecision_TieBreakers(t *testing.T) {
	t.Parallel()

	runs := []SeedRunResult{
		{Seed: 1, Promotable: true, AccuracyDelta: 0.10, ReconDelta: 0, ErrorsDelta: 0, LatencyDeltaMS: 50},
		{Seed: 2, Promotable: true, AccuracyDelta: 0.10, ReconDelta: -1, ErrorsDelta: 0, LatencyDeltaMS: 100},
		{Seed: 3, Promotable: false, AccuracyDelta: 0.20, ReconDelta: 1, ErrorsDelta: 1, LatencyDeltaMS: -10},
	}
	decision := SelectPromotionDecision(runs)
	if !decision.Promoted || decision.SelectedSeed == nil {
		t.Fatalf("expected promoted decision, got %+v", decision)
	}
	if *decision.SelectedSeed != 2 {
		t.Fatalf("expected seed 2 via recon tie-break, got %d", *decision.SelectedSeed)
	}
}
