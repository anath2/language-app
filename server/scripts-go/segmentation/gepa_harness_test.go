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
	if len(cases) < 12 {
		t.Fatalf("expected at least 12 rows, got %d", len(cases))
	}
	if strings.TrimSpace(cases[0].Paragraph) == "" {
		t.Fatal("first case paragraph should not be empty")
	}
}

func TestLoadDefaultCases_PrefersRepoRootDatasetPath(t *testing.T) {
	tempDir := t.TempDir()
	rootCSVPath := filepath.Join(tempDir, "data", "jepa", "datasets", "paragraphs.csv")
	legacyCSVPath := filepath.Join(tempDir, "server", "data", "jepa", "paragraphs.csv")

	if err := os.MkdirAll(filepath.Dir(rootCSVPath), 0o755); err != nil {
		t.Fatalf("mkdir root dataset dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyCSVPath), 0o755); err != nil {
		t.Fatalf("mkdir legacy dataset dir: %v", err)
	}
	if err := os.WriteFile(rootCSVPath, []byte("id,paragraph\nroot,根路径数据集。\n"), 0o644); err != nil {
		t.Fatalf("write root dataset: %v", err)
	}
	if err := os.WriteFile(legacyCSVPath, []byte("id,paragraph\nlegacy,旧路径数据集。\n"), 0o644); err != nil {
		t.Fatalf("write legacy dataset: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	cases, err := LoadDefaultCases()
	if err != nil {
		t.Fatalf("load default cases: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(cases))
	}
	if cases[0].Name != "root" {
		t.Fatalf("expected root dataset to win, got first case %q", cases[0].Name)
	}
}

func TestLoadCasesFromCSV_SkipsEmptyParagraph(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "mixed.csv")
	payload := "id,paragraph\np01,我喜欢中文。\np02,\np03,人工智能。\n"
	if err := os.WriteFile(csvPath, []byte(payload), 0o644); err != nil {
		t.Fatalf("write temp csv: %v", err)
	}

	cases, err := LoadCasesFromCSV(csvPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("expected 2 cases (empty paragraph skipped), got %d", len(cases))
	}
	if cases[0].Name != "p01" || cases[1].Name != "p03" {
		t.Fatalf("unexpected case names: %s, %s", cases[0].Name, cases[1].Name)
	}
}

func TestBuildGEPAParagraphDataset(t *testing.T) {
	t.Parallel()

	corpus := []Case{
		{Name: "p01", Paragraph: "我喜欢中文。人工智能改变世界。"},
		{Name: "p02", Paragraph: "今天下午我们一起去图书馆看书。"},
	}

	ds, examples := BuildGEPAParagraphDataset(corpus, 10)
	if ds == nil {
		t.Fatal("expected non-nil dataset")
	}
	if len(examples) != 2 {
		t.Fatalf("expected 2 examples, got %d", len(examples))
	}

	for i, ex := range examples {
		gotParagraph, _ := ex.Inputs["paragraph"].(string)
		if gotParagraph != corpus[i].Paragraph {
			t.Fatalf("example %d paragraph mismatch: got %q want %q", i, gotParagraph, corpus[i].Paragraph)
		}
	}
}

func TestSplitCasesDeterministic(t *testing.T) {
	t.Parallel()

	cases := []Case{
		{Name: "a", Paragraph: "我喜欢中文。"},
		{Name: "b", Paragraph: "人工智能改变世界。"},
		{Name: "c", Paragraph: "我们去图书馆。"},
		{Name: "d", Paragraph: "研究生命起源。"},
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

func TestSplitParagraphSentences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  []string
	}{
		{
			input: "我喜欢学习中文。人工智能改变世界。",
			want:  []string{"我喜欢学习中文。", "人工智能改变世界。"},
		},
		{
			input: "你好！你怎么样？我很好。",
			want:  []string{"你好！", "你怎么样？", "我很好。"},
		},
		{
			input: "第一行\n第二行",
			want:  []string{"第一行", "第二行"},
		},
		{
			input: "没有句号的文本",
			want:  []string{"没有句号的文本"},
		},
	}

	for _, tt := range tests {
		got := splitParagraphSentences(tt.input)
		if len(got) != len(tt.want) {
			t.Fatalf("splitParagraphSentences(%q): got %d sentences %q, want %d %q", tt.input, len(got), got, len(tt.want), tt.want)
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Fatalf("splitParagraphSentences(%q)[%d]: got %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestParseJudgeScore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  float64
	}{
		{"8", 8.0},
		{"7.5", 7.5},
		{"The score is 9 out of 10", 9.0},
		{"", 5.0},
		{"no numbers here", 5.0},
		{"15", 10.0}, // clamped
		{"-3", 3.0},  // regex finds "3" in "-3"
		{"0.5", 0.5},
	}

	for _, tt := range tests {
		got := parseJudgeScore(tt.input)
		if got != tt.want {
			t.Errorf("parseJudgeScore(%q) = %.1f, want %.1f", tt.input, got, tt.want)
		}
	}
}

func TestBoundFloat(t *testing.T) {
	t.Parallel()

	if got := boundFloat(-1, 0, 1); got != 0 {
		t.Errorf("boundFloat(-1,0,1) = %f, want 0", got)
	}
	if got := boundFloat(2, 0, 1); got != 1 {
		t.Errorf("boundFloat(2,0,1) = %f, want 1", got)
	}
	if got := boundFloat(0.5, 0, 1); got != 0.5 {
		t.Errorf("boundFloat(0.5,0,1) = %f, want 0.5", got)
	}
}

func TestWriteOptimizationCampaignArtifacts_UsesRootAndRunsLayout(t *testing.T) {
	tempDir := t.TempDir()
	cfg := ModerateFastGEPAConfig()
	selectedSeed := 101

	runs := []SeedRunResult{
		{
			Seed:       selectedSeed,
			Promotable: true,
			CompiledResult: CompileResult{
				BestInstruction: "repo root instruction",
			},
			BaselineEval: EvalSummary{TotalCases: 1},
			CompiledEval: EvalSummary{TotalCases: 1},
		},
	}

	summary := CampaignSummary{Seeds: 1}
	decision := PromotionDecision{Promoted: true, SelectedSeed: &selectedSeed}

	if err := WriteOptimizationCampaignArtifacts(
		tempDir,
		"test-model",
		"data/jepa/datasets/paragraphs.csv",
		cfg,
		runs,
		summary,
		decision,
	); err != nil {
		t.Fatalf("write campaign artifacts: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tempDir, "compiled_instruction.txt")); err != nil {
		t.Fatalf("expected compiled instruction at root artifact dir: %v", err)
	}
	for _, path := range []string{
		filepath.Join(tempDir, "runs", "compile_metadata.json"),
		filepath.Join(tempDir, "runs", "promotion_decision.json"),
		filepath.Join(tempDir, "runs", "multi_seed_runs.json"),
		filepath.Join(tempDir, "runs", "multi_seed_summary.json"),
		filepath.Join(tempDir, "runs", "gepa_segmentation_results_2026-02-14.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected run artifact %q: %v", path, err)
		}
	}
}

func TestFormatTranslationsForJudge(t *testing.T) {
	t.Parallel()

	// Empty translations returns empty.
	if got := formatTranslationsForJudge("hello", ""); got != "" {
		t.Errorf("expected empty for empty translations, got %q", got)
	}
	if got := formatTranslationsForJudge("hello", "null"); got != "" {
		t.Errorf("expected empty for null translations, got %q", got)
	}

	// Non-empty returns formatted prompt.
	got := formatTranslationsForJudge("我好", `[{"pinyin":"wǒ hǎo","english":"I'm good"}]`)
	if !strings.Contains(got, "我好") || !strings.Contains(got, "pinyin") {
		t.Errorf("formatted judge prompt missing expected content: %q", got)
	}
}
