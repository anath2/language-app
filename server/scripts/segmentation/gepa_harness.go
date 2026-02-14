package segmentation

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/XiaoConstantine/dspy-go/pkg/core"
	"github.com/XiaoConstantine/dspy-go/pkg/datasets"
	"github.com/XiaoConstantine/dspy-go/pkg/llms"
	"github.com/XiaoConstantine/dspy-go/pkg/modules"
	"github.com/XiaoConstantine/dspy-go/pkg/optimizers"
	"github.com/anath2/language-app/internal/config"
)

const (
	SegmentationLLMTimeout   = 3 * time.Minute
	DefaultCSVPath           = "data/jepa/sentences_20.csv"
	DefaultArtifactsDir      = "data/jepa"
	DefaultReportPath        = DefaultArtifactsDir + "/gepa_segmentation_results_2026-02-14.md"
	DefaultInstructionPath   = DefaultArtifactsDir + "/compiled_instruction.txt"
	DefaultMetadataPath      = DefaultArtifactsDir + "/compile_metadata.json"
	HardenedInstruction      = "Segment the Chinese input into an ordered JSON array of contiguous chunks that exactly reconstruct the original text when concatenated. Preserve every character in order, including Chinese/ASCII punctuation, symbols, and line breaks. Do not drop, normalize, paraphrase, or insert characters. Keep common multi-character words together when appropriate (for example, 人工智能, 图书馆, 看书, 为时未晚). Return only the segments array."
	minCSVRowFieldCount      = 3
	csvHeaderID              = "id"
	csvHeaderSentence        = "sentence"
	csvHeaderExpectedSegJSON = "expected_segments_json"
)

type Case struct {
	Name     string
	Text     string
	Expected []string
}

type EvalSummary struct {
	ExactMatches       int
	TotalCases         int
	ReconstructionFail int
	TotalLatency       time.Duration
	Errors             int
}

type CompileResult struct {
	CompileElapsed   time.Duration
	BestInstruction  string
	OptimizedProgram core.Program
	State            *optimizers.GEPAState
	DatasetUnits     int
}

type CompileMetadata struct {
	ModelID         string                 `json:"model_id"`
	DatasetPath     string                 `json:"dataset_path"`
	DatasetUnits    int                    `json:"dataset_units"`
	GeneratedAtUTC  string                 `json:"generated_at_utc"`
	CompileElapsed  string                 `json:"compile_elapsed"`
	BestFitness     float64                `json:"best_fitness"`
	Generations     int                    `json:"generations_executed"`
	PopulationSize  int                    `json:"population_size"`
	MaxGenerations  int                    `json:"max_generations"`
	EvalBatchSize   int                    `json:"evaluation_batch_size"`
	ReflectionFreq  int                    `json:"reflection_frequency"`
	StagnationLimit int                    `json:"stagnation_limit"`
	Extra           map[string]interface{} `json:"extra,omitempty"`
}

type stickyPredict struct {
	*modules.Predict
}

func (s *stickyPredict) Clone() core.Module {
	// Keep module pointer stable so candidate instruction updates are observed
	// by Program.Forward execution during optimization.
	return s
}

func LoadDefaultCases() ([]Case, error) {
	candidates := []string{
		DefaultCSVPath,
		filepath.Join("scripts", "segmentation", DefaultCSVPath),
	}
	var lastErr error
	for _, p := range candidates {
		cases, err := LoadCasesFromCSV(p)
		if err == nil {
			return cases, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func LoadCasesFromCSV(path string) ([]Case, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv %q: %w", path, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv %q: %w", path, err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("csv %q must include header and at least one row", path)
	}

	header := rows[0]
	colIdx, err := csvColumnIndices(header)
	if err != nil {
		return nil, err
	}

	cases := make([]Case, 0, len(rows)-1)
	for i, row := range rows[1:] {
		rowNum := i + 2
		if len(row) < minCSVRowFieldCount {
			return nil, fmt.Errorf("csv row %d has %d fields, expected at least %d", rowNum, len(row), minCSVRowFieldCount)
		}

		name := strings.TrimSpace(row[colIdx[csvHeaderID]])
		text := strings.TrimSpace(row[colIdx[csvHeaderSentence]])
		rawExpected := strings.TrimSpace(row[colIdx[csvHeaderExpectedSegJSON]])

		if name == "" {
			return nil, fmt.Errorf("csv row %d has empty id", rowNum)
		}
		if text == "" {
			return nil, fmt.Errorf("csv row %d has empty sentence", rowNum)
		}

		var expected []string
		if err := json.Unmarshal([]byte(rawExpected), &expected); err != nil {
			return nil, fmt.Errorf("csv row %d invalid expected_segments_json: %w", rowNum, err)
		}
		if len(expected) == 0 {
			return nil, fmt.Errorf("csv row %d has empty expected_segments_json", rowNum)
		}

		cases = append(cases, Case{
			Name:     name,
			Text:     text,
			Expected: expected,
		})
	}

	return cases, nil
}

func csvColumnIndices(header []string) (map[string]int, error) {
	indices := map[string]int{}
	for i, col := range header {
		indices[strings.TrimSpace(strings.ToLower(col))] = i
	}
	required := []string{csvHeaderID, csvHeaderSentence, csvHeaderExpectedSegJSON}
	for _, k := range required {
		if _, ok := indices[k]; !ok {
			return nil, fmt.Errorf("csv header missing required column %q", k)
		}
	}
	return indices, nil
}

func QuickBudgetGEPAConfig() *optimizers.GEPAConfig {
	cfg := optimizers.DefaultGEPAConfig()
	cfg.PopulationSize = 2
	cfg.MaxGenerations = 1
	cfg.EvaluationBatchSize = 1
	cfg.ConcurrencyLevel = 1
	cfg.ReflectionFreq = 2
	cfg.StagnationLimit = 2
	cfg.ConvergenceThreshold = 0.005
	return cfg
}

func NewSegmentationLLM(cfg config.Config, modelID string) (core.LLM, error) {
	llms.EnsureFactory()
	baseURL, path, err := normalizeOpenAIEndpoint(cfg.OpenAIBaseURL)
	if err != nil {
		return nil, err
	}
	openAILLM, err := llms.NewOpenAILLM(
		core.ModelID(strings.TrimSpace(modelID)),
		llms.WithAPIKey(cfg.OpenAIAPIKey),
		llms.WithOpenAIBaseURL(baseURL),
		llms.WithOpenAIPath(path),
		llms.WithOpenAITimeout(SegmentationLLMTimeout),
	)
	if err != nil {
		return nil, fmt.Errorf("new openai llm: %w", err)
	}
	return openAILLM, nil
}

func NewGEPASegmentationProgram(llm core.LLM, instruction string) core.Program {
	mod := &stickyPredict{Predict: modules.NewPredict(buildSegmentSignature(instruction)).WithStructuredOutput()}
	mod.SetLLM(llm)
	return core.Program{
		Modules: map[string]core.Module{"segmenter": mod},
		Forward: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
			text, _ := inputs["text"].(string)
			text = strings.TrimSpace(text)
			start := time.Now()
			callCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
			defer cancel()

			res, err := mod.Process(callCtx, map[string]any{"text": text})
			if err != nil {
				return nil, err
			}

			parseFailed := false
			segments := parseSegments(res["segments"])
			if len(segments) == 0 {
				segments = parseSegmentsFromResponse(res["response"])
				parseFailed = true
			}
			if len(segments) == 0 {
				segments = parseLooseSegments(toString(res["segments"]))
				parseFailed = true
			}
			if len(segments) == 0 {
				segments = parseLooseSegments(toString(res["response"]))
				parseFailed = true
			}

			reconstructionOK := normalizeForReconstruction(strings.Join(segments, "")) == normalizeForReconstruction(text)
			return map[string]interface{}{
				"segments":          segments,
				"text":              text,
				"parse_failed":      parseFailed || len(segments) == 0,
				"reconstruction_ok": reconstructionOK,
				"latency_ms":        float64(time.Since(start).Milliseconds()),
			}, nil
		},
	}
}

func BuildGEPASentenceDataset(corpus []Case, maxUnits int) (*datasets.SimpleDataset, []core.Example) {
	examples := make([]core.Example, 0, maxUnits)
	for _, tc := range corpus {
		text := strings.TrimSpace(tc.Text)
		if text == "" || len(tc.Expected) == 0 {
			continue
		}
		examples = append(examples, core.Example{
			Inputs:  map[string]interface{}{"text": text},
			Outputs: map[string]interface{}{"text": text, "segments": tc.Expected},
		})
		if len(examples) >= maxUnits {
			return datasets.NewSimpleDataset(examples), examples
		}
	}
	return datasets.NewSimpleDataset(examples), examples
}

func CompileGEPASentenceLevel(
	ctx context.Context,
	llm core.LLM,
	corpus []Case,
	baseInstruction string,
	cfg *optimizers.GEPAConfig,
	maxDatasetUnits int,
) (CompileResult, error) {
	dataset, units := BuildGEPASentenceDataset(corpus, maxDatasetUnits)
	if len(units) == 0 {
		return CompileResult{}, fmt.Errorf("empty GEPA dataset")
	}

	program := NewGEPASegmentationProgram(llm, baseInstruction)
	gepa, err := optimizers.NewGEPA(cfg)
	if err != nil {
		return CompileResult{}, fmt.Errorf("new GEPA: %w", err)
	}

	compileCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	start := time.Now()
	optimizedProgram, err := gepa.Compile(compileCtx, program, dataset, gepaSentenceMetric)
	if err != nil {
		return CompileResult{}, err
	}
	elapsed := time.Since(start)

	state := gepa.GetOptimizationState()
	bestInstruction := extractInstructionFromProgram(optimizedProgram, "segmenter")
	if state != nil && state.BestCandidate != nil && strings.TrimSpace(state.BestCandidate.Instruction) != "" {
		bestInstruction = state.BestCandidate.Instruction
	}
	if strings.TrimSpace(bestInstruction) == "" {
		return CompileResult{}, fmt.Errorf("compiled instruction is empty")
	}

	return CompileResult{
		CompileElapsed:   elapsed,
		BestInstruction:  bestInstruction,
		OptimizedProgram: optimizedProgram,
		State:            state,
		DatasetUnits:     len(units),
	}, nil
}

func EvaluateSentenceLevelProgram(ctx context.Context, program core.Program, corpus []Case) EvalSummary {
	summary := EvalSummary{TotalCases: len(corpus)}
	for _, tc := range corpus {
		start := time.Now()
		res, err := program.Execute(ctx, map[string]interface{}{"text": tc.Text})
		latency := time.Since(start)
		if err != nil {
			summary.Errors++
			summary.ReconstructionFail++
			summary.TotalLatency += latency
			continue
		}
		summary.TotalLatency += latency

		segments := parseSegments(res["segments"])
		if len(segments) == 0 {
			segments = parseSegmentsFromResponse(res["response"])
		}
		if len(segments) == 0 {
			segments = parseLooseSegments(toString(res["segments"]))
		}
		if len(segments) == 0 {
			segments = parseLooseSegments(toString(res["response"]))
		}
		if len(segments) == 0 {
			summary.Errors++
			summary.ReconstructionFail++
			continue
		}

		if equalSegments(segments, tc.Expected) {
			summary.ExactMatches++
		}
		if normalizeForReconstruction(strings.Join(segments, "")) != normalizeForReconstruction(tc.Text) {
			summary.ReconstructionFail++
		}
	}
	return summary
}

func WriteGEPAArtifacts(
	artifactDir string,
	modelID string,
	datasetPath string,
	cfg *optimizers.GEPAConfig,
	result CompileResult,
	baseline EvalSummary,
	compiled EvalSummary,
) error {
	if artifactDir == "" {
		artifactDir = DefaultArtifactsDir
	}
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return err
	}

	bestFitness := 0.0
	generations := 0
	if result.State != nil {
		bestFitness = result.State.BestFitness
		generations = result.State.CurrentGeneration + 1
	}

	metadata := CompileMetadata{
		ModelID:         modelID,
		DatasetPath:     datasetPath,
		DatasetUnits:    result.DatasetUnits,
		GeneratedAtUTC:  time.Now().UTC().Format(time.RFC3339),
		CompileElapsed:  result.CompileElapsed.String(),
		BestFitness:     bestFitness,
		Generations:     generations,
		PopulationSize:  cfg.PopulationSize,
		MaxGenerations:  cfg.MaxGenerations,
		EvalBatchSize:   cfg.EvaluationBatchSize,
		ReflectionFreq:  cfg.ReflectionFreq,
		StagnationLimit: cfg.StagnationLimit,
		Extra: map[string]interface{}{
			"baseline_accuracy": AccuracyOf(baseline),
			"compiled_accuracy": AccuracyOf(compiled),
			"accuracy_delta":    AccuracyOf(compiled) - AccuracyOf(baseline),
		},
	}

	metaJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata json: %w", err)
	}

	if err := os.WriteFile(filepath.Join(artifactDir, filepath.Base(DefaultInstructionPath)), []byte(result.BestInstruction), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(artifactDir, filepath.Base(DefaultMetadataPath)), metaJSON, 0o644); err != nil {
		return err
	}
	return WriteGEPAResultsReport(
		filepath.Join(artifactDir, filepath.Base(DefaultReportPath)),
		modelID,
		cfg,
		datasetPath,
		result,
		baseline,
		compiled,
	)
}

func WriteGEPAResultsReport(
	reportPath string,
	modelID string,
	cfg *optimizers.GEPAConfig,
	datasetPath string,
	result CompileResult,
	baseline EvalSummary,
	compiled EvalSummary,
) error {
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		return err
	}

	bestFitness := 0.0
	generations := 0
	if result.State != nil {
		bestFitness = result.State.BestFitness
		generations = result.State.CurrentGeneration + 1
	}

	content := fmt.Sprintf(`# GEPA Segmentation Results (2026-02-14)

## Setup
- model: %s
- optimizer: GEPA
- objective: sentence-level segmentation prompt optimization
- dataset source: %s
- dataset size (sentence units): %d

## Quick-Budget Config
- population_size: %d
- max_generations: %d
- evaluation_batch_size: %d
- concurrency_level: %d
- reflection_frequency: %d
- stagnation_limit: %d
- convergence_threshold: %.4f

## Compile Artifacts
- elapsed: %s
- best_fitness: %.4f
- generations_executed: %d

### Best Compiled Instruction
%s

## Post-Compile Comparison
- baseline_accuracy: %.2f (%d/%d)
- compiled_accuracy: %.2f (%d/%d)
- accuracy_delta: %.2f
- baseline_reconstruction_failures: %d
- compiled_reconstruction_failures: %d
- baseline_errors: %d
- compiled_errors: %d
- baseline_avg_latency: %s
- compiled_avg_latency: %s
- latency_delta: %s
`,
		modelID,
		datasetPath,
		result.DatasetUnits,
		cfg.PopulationSize,
		cfg.MaxGenerations,
		cfg.EvaluationBatchSize,
		cfg.ConcurrencyLevel,
		cfg.ReflectionFreq,
		cfg.StagnationLimit,
		cfg.ConvergenceThreshold,
		result.CompileElapsed,
		bestFitness,
		generations,
		result.BestInstruction,
		AccuracyOf(baseline),
		baseline.ExactMatches,
		max(1, baseline.TotalCases),
		AccuracyOf(compiled),
		compiled.ExactMatches,
		max(1, compiled.TotalCases),
		AccuracyOf(compiled)-AccuracyOf(baseline),
		baseline.ReconstructionFail,
		compiled.ReconstructionFail,
		baseline.Errors,
		compiled.Errors,
		AvgLatencyOf(baseline),
		AvgLatencyOf(compiled),
		AvgLatencyOf(compiled)-AvgLatencyOf(baseline),
	)

	return os.WriteFile(reportPath, []byte(content), 0o644)
}

func AccuracyOf(summary EvalSummary) float64 {
	return float64(summary.ExactMatches) / float64(max(1, summary.TotalCases))
}

func AvgLatencyOf(summary EvalSummary) time.Duration {
	return summary.TotalLatency / time.Duration(max(1, summary.TotalCases))
}

func normalizeOpenAIEndpoint(rawBaseURL string) (string, string, error) {
	baseURL := strings.TrimSpace(rawBaseURL)
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return "", "", fmt.Errorf("must be a full URL ending with /v1")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", "", fmt.Errorf("parse URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", "", fmt.Errorf("must include scheme and host")
	}
	path := strings.TrimRight(parsed.Path, "/")
	if path == "" || !strings.HasSuffix(path, "/v1") {
		return "", "", fmt.Errorf("path must end with /v1")
	}
	parsed.Path = path
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), "/chat/completions", nil
}

func buildSegmentSignature(instruction string) core.Signature {
	return core.NewSignature(
		[]core.InputField{{Field: core.NewField("text", core.WithDescription("Chinese sentence to segment"))}},
		[]core.OutputField{{Field: core.NewField("segments", core.WithDescription("Array of segmented words in order"))}},
	).WithInstruction(instruction)
}

func gepaSentenceMetric(expected, actual map[string]interface{}) float64 {
	expectedSegments := parseSegments(expected["segments"])
	actualSegments := parseSegments(actual["segments"])
	text := strings.TrimSpace(toString(expected["text"]))
	if text == "" {
		text = strings.TrimSpace(toString(actual["text"]))
	}
	if len(expectedSegments) == 0 || text == "" || len(actualSegments) == 0 {
		return 0
	}

	score := boundaryF1FromSegments(expectedSegments, actualSegments)
	if equalSegments(expectedSegments, actualSegments) {
		score = 1.0
	}
	if isTruthy(actual["parse_failed"]) {
		score -= 0.35
	}
	reconstructionOK := normalizeForReconstruction(strings.Join(actualSegments, "")) == normalizeForReconstruction(text)
	if !reconstructionOK {
		score -= 0.45
	}
	latencyMs := toFloat64(actual["latency_ms"])
	if latencyMs > 0 {
		score -= minFloat(0.05, latencyMs/10000.0)
	}
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func boundaryF1FromSegments(expected, actual []string) float64 {
	expectedBounds := segmentationBoundaries(expected)
	actualBounds := segmentationBoundaries(actual)
	if len(expectedBounds) == 0 && len(actualBounds) == 0 {
		return 1
	}
	if len(expectedBounds) == 0 || len(actualBounds) == 0 {
		return 0
	}
	tp := 0
	for b := range actualBounds {
		if _, ok := expectedBounds[b]; ok {
			tp++
		}
	}
	precision := float64(tp) / float64(max(1, len(actualBounds)))
	recall := float64(tp) / float64(max(1, len(expectedBounds)))
	if precision+recall == 0 {
		return 0
	}
	return 2 * precision * recall / (precision + recall)
}

func segmentationBoundaries(segments []string) map[int]struct{} {
	bounds := make(map[int]struct{})
	pos := 0
	for idx, seg := range segments {
		pos += len([]rune(seg))
		if idx < len(segments)-1 {
			bounds[pos] = struct{}{}
		}
	}
	return bounds
}

func extractInstructionFromProgram(program core.Program, moduleName string) string {
	mod, ok := program.Modules[moduleName]
	if !ok || mod == nil {
		return ""
	}
	return strings.TrimSpace(mod.GetSignature().Instruction)
}

func parseSegments(v any) []string {
	if v == nil {
		return nil
	}
	switch items := v.(type) {
	case []any:
		out := make([]string, 0, len(items))
		for _, it := range items {
			s := strings.TrimSpace(toString(it))
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(items))
		for _, it := range items {
			s := strings.TrimSpace(it)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return parseSegmentsString(strings.TrimSpace(toString(v)))
	}
}

func parseSegmentsFromResponse(v any) []string {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		if segments := parseSegments(m["segments"]); len(segments) > 0 {
			return segments
		}
	}
	raw := normalizeJSONLikePayload(strings.TrimSpace(toString(v)))
	if raw == "" {
		return nil
	}
	if segments := parseSegmentsString(raw); len(segments) > 0 {
		return segments
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return parseSegments(payload["segments"])
}

func parseSegmentsString(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "segments:") {
		raw = strings.TrimSpace(raw[len("segments:"):])
	}
	if raw == "" {
		return nil
	}

	var listPayload []any
	if err := json.Unmarshal([]byte(raw), &listPayload); err == nil {
		out := make([]string, 0, len(listPayload))
		for _, it := range listPayload {
			s := strings.TrimSpace(toString(it))
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	var mapPayload map[string]any
	if err := json.Unmarshal([]byte(raw), &mapPayload); err == nil {
		return parseSegments(mapPayload["segments"])
	}
	return nil
}

func normalizeJSONLikePayload(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "```") {
		parts := strings.Split(raw, "\n")
		if len(parts) >= 2 {
			parts = parts[1:]
		}
		if len(parts) > 0 && strings.HasPrefix(strings.TrimSpace(parts[len(parts)-1]), "```") {
			parts = parts[:len(parts)-1]
		}
		raw = strings.TrimSpace(strings.Join(parts, "\n"))
	}
	return strings.TrimSpace(strings.TrimPrefix(raw, "json"))
}

func parseLooseSegments(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	raw = strings.ReplaceAll(raw, "\n", " ")
	raw = strings.ReplaceAll(raw, ",", " ")
	raw = strings.ReplaceAll(raw, "|", " ")
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return nil
	}
	return parts
}

func equalSegments(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func normalizeForReconstruction(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}

func toFloat64(v interface{}) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	default:
		return 0
	}
}

func isTruthy(v interface{}) bool {
	b, ok := v.(bool)
	return ok && b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
