package segmentation

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/XiaoConstantine/dspy-go/pkg/core"
	"github.com/XiaoConstantine/dspy-go/pkg/datasets"
	"github.com/XiaoConstantine/dspy-go/pkg/llms"
	"github.com/XiaoConstantine/dspy-go/pkg/modules"
	"github.com/XiaoConstantine/dspy-go/pkg/optimizers"
	"github.com/anath2/language-app/internal/config"
)

const (
	SegmentationLLMTimeout = 3 * time.Minute
	DefaultCSVPath         = "data/jepa/paragraphs.csv"
	DefaultArtifactsDir    = "data/jepa"
	DefaultReportPath      = DefaultArtifactsDir + "/gepa_segmentation_results_2026-02-14.md"
	DefaultInstructionPath = DefaultArtifactsDir + "/compiled_instruction.txt"
	DefaultMetadataPath    = DefaultArtifactsDir + "/compile_metadata.json"
	DefaultDecisionPath    = DefaultArtifactsDir + "/promotion_decision.json"
	DefaultRunsPath        = DefaultArtifactsDir + "/multi_seed_runs.json"
	DefaultSummaryPath     = DefaultArtifactsDir + "/multi_seed_summary.json"
	HardenedInstruction    = "Segment the Chinese input into an ordered JSON array of contiguous chunks that exactly reconstruct the original text when concatenated. Preserve every character in order, including Chinese/ASCII punctuation, symbols, and line breaks. Do not drop, normalize, paraphrase, or insert characters. Keep common multi-character words together when appropriate (for example, 人工智能, 图书馆, 看书, 为时未晚). Return only the segments array."
	csvHeaderID            = "id"
	csvHeaderParagraph     = "paragraph"

	defaultTranslateInstruction = "Given an array of Chinese word segments from a sentence, produce the pinyin (with tone marks) and a concise English translation for each segment. Use the sentence and full text for context to select the correct reading and meaning. Return a JSON array of objects with \"pinyin\" and \"english\" fields, in the same order as the input segments."
)

type Case struct {
	Name      string
	Paragraph string
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

type SeedRunResult struct {
	Seed            int           `json:"seed"`
	TrainSize       int           `json:"train_size"`
	EvalSize        int           `json:"eval_size"`
	BaseInstruction string        `json:"base_instruction"`
	CompiledResult  CompileResult `json:"-"`
	BaselineEval    EvalSummary   `json:"baseline_eval"`
	CompiledEval    EvalSummary   `json:"compiled_eval"`
	AccuracyDelta   float64       `json:"accuracy_delta"`
	ReconDelta      int           `json:"reconstruction_delta"`
	ErrorsDelta     int           `json:"errors_delta"`
	LatencyDeltaMS  int64         `json:"latency_delta_ms"`
	Promotable      bool          `json:"promotable"`
	RejectReasons   []string      `json:"reject_reasons,omitempty"`
}

type CampaignSummary struct {
	Seeds              int     `json:"seeds"`
	PromotableCount    int     `json:"promotable_count"`
	AccuracyDeltaMean  float64 `json:"accuracy_delta_mean"`
	AccuracyDeltaStd   float64 `json:"accuracy_delta_std"`
	BestAccuracyDelta  float64 `json:"best_accuracy_delta"`
	WorstAccuracyDelta float64 `json:"worst_accuracy_delta"`
}

type PromotionDecision struct {
	Promoted        bool    `json:"promoted"`
	SelectedSeed    *int    `json:"selected_seed,omitempty"`
	Reason          string  `json:"reason"`
	CandidateCount  int     `json:"candidate_count"`
	PromotableSeeds []int   `json:"promotable_seeds,omitempty"`
	SelectedDelta   float64 `json:"selected_accuracy_delta,omitempty"`
	GeneratedAtUTC  string  `json:"generated_at_utc"`
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

// ---------------------------------------------------------------------------
// Dataset loading
// ---------------------------------------------------------------------------

func LoadDefaultCases() ([]Case, error) {
	candidates := []string{
		DefaultCSVPath,
		filepath.Join("..", "..", DefaultCSVPath),
		filepath.Join("server", DefaultCSVPath),
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
		if len(row) < 2 {
			return nil, fmt.Errorf("csv row %d has %d fields, expected at least 2", rowNum, len(row))
		}

		name := strings.TrimSpace(row[colIdx[csvHeaderID]])
		paragraph := strings.TrimSpace(row[colIdx[csvHeaderParagraph]])

		if name == "" {
			return nil, fmt.Errorf("csv row %d has empty id", rowNum)
		}
		if paragraph == "" {
			continue // skip rows with empty paragraph
		}

		cases = append(cases, Case{
			Name:      name,
			Paragraph: paragraph,
		})
	}

	return cases, nil
}

func csvColumnIndices(header []string) (map[string]int, error) {
	indices := map[string]int{}
	for i, col := range header {
		indices[strings.TrimSpace(strings.ToLower(col))] = i
	}
	required := []string{csvHeaderID, csvHeaderParagraph}
	for _, k := range required {
		if _, ok := indices[k]; !ok {
			return nil, fmt.Errorf("csv header missing required column %q", k)
		}
	}
	return indices, nil
}

// ---------------------------------------------------------------------------
// GEPA configs
// ---------------------------------------------------------------------------

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

func ModerateFastGEPAConfig() *optimizers.GEPAConfig {
	cfg := optimizers.DefaultGEPAConfig()
	cfg.PopulationSize = 8
	cfg.MaxGenerations = 4
	cfg.EvaluationBatchSize = 3
	cfg.ConcurrencyLevel = 1
	cfg.ReflectionFreq = 2
	cfg.StagnationLimit = 3
	cfg.ConvergenceThreshold = 0.003
	return cfg
}

func BuildConstrainedInstruction(segmentationPreference string) string {
	preference := strings.TrimSpace(segmentationPreference)
	if preference == "" {
		preference = "Prefer common lexicalized multi-character words while preserving exact text reconstruction."
	}
	return strings.Join([]string{
		"You are an expert Chinese segmenter.",
		"Non-negotiable constraints:",
		"1) Concatenated output segments must exactly reconstruct original input text.",
		"2) Preserve all punctuation, symbols, ASCII, and whitespace in order.",
		"3) Never insert, delete, normalize, paraphrase, or translate characters.",
		"Segmentation preference:",
		preference,
		"Return only the segments array.",
	}, " ")
}

// ---------------------------------------------------------------------------
// Dataset splitting
// ---------------------------------------------------------------------------

func SplitCasesDeterministic(cases []Case, trainRatio float64, seed int, maxUnits int) ([]Case, []Case) {
	if trainRatio <= 0 || trainRatio >= 1 {
		trainRatio = 0.7
	}
	normalized := make([]Case, 0, len(cases))
	for _, c := range cases {
		if strings.TrimSpace(c.Paragraph) != "" {
			normalized = append(normalized, c)
		}
	}
	if len(normalized) == 0 {
		return nil, nil
	}

	idx := make([]int, len(normalized))
	for i := range idx {
		idx[i] = i
	}
	r := seedMix(seed)
	for i := len(idx) - 1; i > 0; i-- {
		j := int(r % uint64(i+1))
		idx[i], idx[j] = idx[j], idx[i]
		r = r*6364136223846793005 + 1
	}

	ordered := make([]Case, 0, len(normalized))
	for _, i := range idx {
		ordered = append(ordered, normalized[i])
	}
	if maxUnits > 0 && maxUnits < len(ordered) {
		ordered = ordered[:maxUnits]
	}
	if len(ordered) < 2 {
		return ordered, nil
	}

	trainSize := int(math.Round(float64(len(ordered)) * trainRatio))
	if trainSize < 1 {
		trainSize = 1
	}
	if trainSize >= len(ordered) {
		trainSize = len(ordered) - 1
	}
	train := append([]Case(nil), ordered[:trainSize]...)
	eval := append([]Case(nil), ordered[trainSize:]...)
	return train, eval
}

func seedMix(seed int) uint64 {
	s := uint64(seed)
	if s == 0 {
		s = 1
	}
	// FNV-inspired simple mixer for deterministic shuffling without rand package.
	return (s * 1099511628211) ^ 1469598103934665603
}

// ---------------------------------------------------------------------------
// LLM initialisation
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Sentence splitting (mirrors production splitInputSentences)
// ---------------------------------------------------------------------------

func splitParagraphSentences(text string) []string {
	var out []string
	var current strings.Builder
	for len(text) > 0 {
		r, size := utf8.DecodeRuneInString(text)
		text = text[size:]
		if isSentenceEnd(r) {
			current.WriteRune(r)
			s := strings.TrimSpace(current.String())
			if s != "" {
				out = append(out, s)
			}
			current.Reset()
			continue
		}
		if r == '\n' || r == '\r' {
			s := strings.TrimSpace(current.String())
			if s != "" {
				out = append(out, s)
			}
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	s := strings.TrimSpace(current.String())
	if s != "" {
		out = append(out, s)
	}
	return out
}

func isSentenceEnd(r rune) bool {
	return r == '。' || r == '！' || r == '？' || r == '!' || r == '?'
}

// ---------------------------------------------------------------------------
// Local copies of helpers from intelligence/translation (not importable from scripts)
// ---------------------------------------------------------------------------

func isCJKIdeograph(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x20000 && r <= 0x2A6DF) ||
		(r >= 0x2A700 && r <= 0x2CEAF) ||
		(r >= 0x2CEB0 && r <= 0x2EBEF) ||
		(r >= 0x30000 && r <= 0x323AF)
}

var chinesePunctuation = `，。！？；：""''（）【】《》、…—`

func shouldSkipSegment(segment string) bool {
	if strings.TrimSpace(segment) == "" {
		return true
	}
	hasCJK := false
	for _, r := range segment {
		if isCJKIdeograph(r) {
			hasCJK = true
			continue
		}
		if unicode.IsSpace(r) {
			continue
		}
		if r <= unicode.MaxASCII && !unicode.IsLetter(r) {
			continue
		}
		if strings.ContainsRune(chinesePunctuation, r) {
			continue
		}
		if unicode.In(r, unicode.Nd, unicode.No, unicode.Po, unicode.Ps, unicode.Pe, unicode.Pd, unicode.Pc, unicode.Sk, unicode.Sm, unicode.So) {
			continue
		}
	}
	return !hasCJK
}

// ---------------------------------------------------------------------------
// Full pipeline program: segment + translate per sentence
// ---------------------------------------------------------------------------

func buildSegmentSignature(instruction string) core.Signature {
	return core.NewSignature(
		[]core.InputField{{Field: core.NewField("text", core.WithDescription("Chinese sentence to segment"))}},
		[]core.OutputField{{Field: core.NewField("segments", core.WithDescription("Array of segmented words in order"))}},
	).WithInstruction(instruction)
}

func buildTranslateSignature(instruction string) core.Signature {
	return core.NewSignature(
		[]core.InputField{
			{Field: core.NewField("segments_json", core.WithDescription("JSON array of Chinese segments to translate"))},
			{Field: core.NewField("sentence", core.WithDescription("The sentence containing the segments"))},
			{Field: core.NewField("full_text", core.WithDescription("The complete input text for broader context"))},
		},
		[]core.OutputField{
			{Field: core.NewField("translations_json", core.WithDescription("JSON array of {pinyin, english} objects in same order as input segments"))},
		},
	).WithInstruction(instruction)
}

func NewFullPipelineProgram(workerLLM core.LLM, segmentInstruction, translateInstruction string) core.Program {
	segMod := &stickyPredict{Predict: modules.NewPredict(buildSegmentSignature(segmentInstruction)).WithStructuredOutput()}
	segMod.SetLLM(workerLLM)

	transMod := &stickyPredict{Predict: modules.NewPredict(buildTranslateSignature(translateInstruction)).WithStructuredOutput()}
	transMod.SetLLM(workerLLM)

	return core.Program{
		Modules: map[string]core.Module{"segmenter": segMod, "translator": transMod},
		Forward: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
			paragraph, _ := inputs["paragraph"].(string)
			paragraph = strings.TrimSpace(paragraph)
			start := time.Now()

			sentences := splitParagraphSentences(paragraph)
			if len(sentences) == 0 {
				sentences = []string{paragraph}
			}

			type sentenceTranslation struct {
				Sentence     string                   `json:"sentence"`
				Segments     []string                 `json:"segments"`
				Translations []map[string]interface{} `json:"translations"`
			}

			parseFailed := false
			reconstructionOK := true
			var allTranslations []sentenceTranslation

			for _, sent := range sentences {
				callCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
				segRes, err := segMod.Process(callCtx, map[string]any{"text": sent})
				cancel()
				if err != nil {
					parseFailed = true
					continue
				}

				segments := parseSegments(segRes["segments"])
				if len(segments) == 0 {
					parseFailed = true
					continue
				}

				// Reconstruction check for this sentence.
				if normalizeForReconstruction(strings.Join(segments, "")) != normalizeForReconstruction(sent) {
					reconstructionOK = false
				}

				// Filter to CJK-bearing segments for translation.
				var cjkSegments []string
				for _, seg := range segments {
					if !shouldSkipSegment(seg) {
						cjkSegments = append(cjkSegments, seg)
					}
				}

				var translations []map[string]interface{}
				if len(cjkSegments) > 0 {
					segJSON, _ := json.Marshal(cjkSegments)
					transCtx, transCancel := context.WithTimeout(ctx, 40*time.Second)
					transRes, err := transMod.Process(transCtx, map[string]any{
						"segments_json": string(segJSON),
						"sentence":      sent,
						"full_text":     paragraph,
					})
					transCancel()
					if err != nil {
						parseFailed = true
					} else {
						translations = parseBatchTranslationsGeneric(transRes["translations_json"])
					}
				}

				allTranslations = append(allTranslations, sentenceTranslation{
					Sentence:     sent,
					Segments:     segments,
					Translations: translations,
				})
			}

			translationsJSON, _ := json.Marshal(allTranslations)
			return map[string]interface{}{
				"translations_json": string(translationsJSON),
				"paragraph":         paragraph,
				"reconstruction_ok": reconstructionOK,
				"parse_failed":      parseFailed,
				"latency_ms":        float64(time.Since(start).Milliseconds()),
			}, nil
		},
	}
}

// parseBatchTranslationsGeneric extracts a []map[string]interface{} from the
// translations_json output field, which may arrive as a string, []any, or
// other JSON-serialisable shape.
func parseBatchTranslationsGeneric(v any) []map[string]interface{} {
	if v == nil {
		return nil
	}
	var raw []byte
	switch t := v.(type) {
	case string:
		raw = []byte(t)
	case []any:
		b, err := json.Marshal(t)
		if err != nil {
			return nil
		}
		raw = b
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		raw = b
	}
	var out []map[string]interface{}
	_ = json.Unmarshal(raw, &out)
	return out
}

// ---------------------------------------------------------------------------
// Full pipeline metric (judge LLM)
// ---------------------------------------------------------------------------

func fullPipelineMetric(judgeLLM core.LLM) func(expected, actual map[string]interface{}) float64 {
	return func(expected, actual map[string]interface{}) float64 {
		paragraph := strings.TrimSpace(toString(actual["paragraph"]))
		if paragraph == "" {
			paragraph = strings.TrimSpace(toString(expected["paragraph"]))
		}
		if paragraph == "" {
			return 0
		}

		score := 1.0

		// Reconstruction penalty.
		if !isTruthy(actual["reconstruction_ok"]) {
			score -= 0.45
		}

		// Parse failure penalty.
		if isTruthy(actual["parse_failed"]) {
			score -= 0.35
		}

		// Judge LLM scoring 0-10.
		translationsJSON := toString(actual["translations_json"])
		judgeInput := formatTranslationsForJudge(paragraph, translationsJSON)
		if judgeInput != "" && judgeLLM != nil {
			judgeScore := queryJudge(judgeLLM, judgeInput)
			score = score - 1.0 + (judgeScore / 10.0) // replace the base 1.0 with normalised judge score
		}

		// Latency penalty (capped at 0.05).
		latencyMs := toFloat64(actual["latency_ms"])
		if latencyMs > 0 {
			score -= boundFloat(latencyMs/10000.0, 0, 0.05)
		}

		return boundFloat(score, 0, 1)
	}
}

func formatTranslationsForJudge(paragraph, translationsJSON string) string {
	if strings.TrimSpace(translationsJSON) == "" || translationsJSON == "null" {
		return ""
	}
	return fmt.Sprintf("Chinese paragraph:\n%s\n\nSegmentation and translation results (JSON):\n%s\n\nRate the overall translation quality from 0 to 10. Consider: Are segments reasonable Chinese word boundaries? Are pinyin readings correct? Are English translations accurate and contextually appropriate? Reply with ONLY a number 0-10.", paragraph, translationsJSON)
}

func queryJudge(judgeLLM core.LLM, prompt string) float64 {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullPrompt := "You are a Chinese language expert evaluating segmentation and translation quality. Reply with ONLY a number from 0 to 10.\n\n" + prompt

	resp, err := judgeLLM.Generate(ctx, fullPrompt)
	if err != nil {
		return 5.0 // neutral fallback on error
	}
	return parseJudgeScore(resp.Content)
}

func parseJudgeScore(raw string) float64 {
	raw = strings.TrimSpace(raw)
	// Try to parse the first number found.
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)`)
	match := re.FindString(raw)
	if match == "" {
		return 5.0
	}
	f, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 5.0
	}
	return boundFloat(f, 0, 10)
}

func boundFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---------------------------------------------------------------------------
// Paragraph dataset builder
// ---------------------------------------------------------------------------

func BuildGEPAParagraphDataset(corpus []Case, maxUnits int) (*datasets.SimpleDataset, []core.Example) {
	examples := make([]core.Example, 0, maxUnits)
	for _, tc := range corpus {
		p := strings.TrimSpace(tc.Paragraph)
		if p == "" {
			continue
		}
		examples = append(examples, core.Example{
			Inputs:  map[string]interface{}{"paragraph": p},
			Outputs: map[string]interface{}{"paragraph": p},
		})
		if len(examples) >= maxUnits {
			return datasets.NewSimpleDataset(examples), examples
		}
	}
	return datasets.NewSimpleDataset(examples), examples
}

// ---------------------------------------------------------------------------
// Compile / Evaluate / Multi-seed orchestration
// ---------------------------------------------------------------------------

func CompileFullPipeline(
	ctx context.Context,
	workerLLM core.LLM,
	judgeLLM core.LLM,
	corpus []Case,
	baseInstruction string,
	cfg *optimizers.GEPAConfig,
	maxDatasetUnits int,
) (CompileResult, error) {
	dataset, units := BuildGEPAParagraphDataset(corpus, maxDatasetUnits)
	if len(units) == 0 {
		return CompileResult{}, fmt.Errorf("empty GEPA dataset")
	}

	program := NewFullPipelineProgram(workerLLM, baseInstruction, defaultTranslateInstruction)
	gepa, err := optimizers.NewGEPA(cfg)
	if err != nil {
		return CompileResult{}, fmt.Errorf("new GEPA: %w", err)
	}

	metric := fullPipelineMetric(judgeLLM)

	compileCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	start := time.Now()
	optimizedProgram, err := gepa.Compile(compileCtx, program, dataset, metric)
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

func EvaluateFullPipeline(ctx context.Context, program core.Program, judgeLLM core.LLM, corpus []Case) EvalSummary {
	metric := fullPipelineMetric(judgeLLM)

	summary := EvalSummary{TotalCases: len(corpus)}
	for _, tc := range corpus {
		start := time.Now()
		res, err := program.Execute(ctx, map[string]interface{}{"paragraph": tc.Paragraph})
		latency := time.Since(start)
		if err != nil {
			summary.Errors++
			summary.ReconstructionFail++
			summary.TotalLatency += latency
			continue
		}
		summary.TotalLatency += latency

		// Use the metric to score.
		expected := map[string]interface{}{"paragraph": tc.Paragraph}
		score := metric(expected, res)
		if score >= 0.9 {
			summary.ExactMatches++
		}
		if !isTruthy(res["reconstruction_ok"]) {
			summary.ReconstructionFail++
		}
	}
	return summary
}

func RunMultiSeedOptimization(
	ctx context.Context,
	llm core.LLM,
	judgeLLM core.LLM,
	modelID string,
	allCases []Case,
	datasetPath string,
	seeds int,
	baseSeed int,
	trainRatio float64,
	maxUnits int,
	cfg *optimizers.GEPAConfig,
) ([]SeedRunResult, CampaignSummary, PromotionDecision, error) {
	if seeds <= 0 {
		seeds = 3
	}
	if cfg == nil {
		cfg = ModerateFastGEPAConfig()
	}

	preferences := []string{
		"Prefer lexicalized multi-character words and stable named entities when boundaries are ambiguous.",
		"Prefer semantically coherent compounds while preserving exact punctuation attachment.",
		"Prefer natural spoken-word grouping for particles and function words without breaking reconstruction.",
	}

	runs := make([]SeedRunResult, 0, seeds)
	for i := 0; i < seeds; i++ {
		seed := baseSeed + i
		train, eval := SplitCasesDeterministic(allCases, trainRatio, seed, maxUnits)
		if len(train) == 0 || len(eval) == 0 {
			return nil, CampaignSummary{}, PromotionDecision{}, fmt.Errorf("seed %d produced empty train/eval split", seed)
		}

		baseInstruction := BuildConstrainedInstruction(preferences[i%len(preferences)])
		comp, err := CompileFullPipeline(ctx, llm, judgeLLM, train, baseInstruction, cfg, len(train))
		if err != nil {
			return nil, CampaignSummary{}, PromotionDecision{}, fmt.Errorf("seed %d compile failed: %w", seed, err)
		}

		baselineProgram := NewFullPipelineProgram(llm, HardenedInstruction, defaultTranslateInstruction)
		compiledProgram := NewFullPipelineProgram(llm, comp.BestInstruction, defaultTranslateInstruction)
		baselineEval := EvaluateFullPipeline(ctx, baselineProgram, judgeLLM, eval)
		compiledEval := EvaluateFullPipeline(ctx, compiledProgram, judgeLLM, eval)

		promotable, reasons := EvaluatePromotionGate(baselineEval, compiledEval)
		run := SeedRunResult{
			Seed:            seed,
			TrainSize:       len(train),
			EvalSize:        len(eval),
			BaseInstruction: baseInstruction,
			CompiledResult:  comp,
			BaselineEval:    baselineEval,
			CompiledEval:    compiledEval,
			AccuracyDelta:   AccuracyOf(compiledEval) - AccuracyOf(baselineEval),
			ReconDelta:      compiledEval.ReconstructionFail - baselineEval.ReconstructionFail,
			ErrorsDelta:     compiledEval.Errors - baselineEval.Errors,
			LatencyDeltaMS:  (AvgLatencyOf(compiledEval) - AvgLatencyOf(baselineEval)).Milliseconds(),
			Promotable:      promotable,
			RejectReasons:   reasons,
		}
		runs = append(runs, run)
	}

	summary := SummarizeRuns(runs)
	decision := SelectPromotionDecision(runs)
	decision.GeneratedAtUTC = time.Now().UTC().Format(time.RFC3339)
	return runs, summary, decision, nil
}

// ---------------------------------------------------------------------------
// Promotion gate / summary / selection
// ---------------------------------------------------------------------------

func EvaluatePromotionGate(baseline EvalSummary, compiled EvalSummary) (bool, []string) {
	reasons := make([]string, 0, 3)
	if AccuracyOf(compiled)-AccuracyOf(baseline) <= 0 {
		reasons = append(reasons, "accuracy_delta_not_positive")
	}
	if compiled.ReconstructionFail > baseline.ReconstructionFail {
		reasons = append(reasons, "reconstruction_failures_increased")
	}
	if compiled.Errors > baseline.Errors {
		reasons = append(reasons, "errors_increased")
	}
	return len(reasons) == 0, reasons
}

func SummarizeRuns(runs []SeedRunResult) CampaignSummary {
	if len(runs) == 0 {
		return CampaignSummary{}
	}
	deltas := make([]float64, 0, len(runs))
	promoted := 0
	best := runs[0].AccuracyDelta
	worst := runs[0].AccuracyDelta
	for _, run := range runs {
		deltas = append(deltas, run.AccuracyDelta)
		if run.Promotable {
			promoted++
		}
		if run.AccuracyDelta > best {
			best = run.AccuracyDelta
		}
		if run.AccuracyDelta < worst {
			worst = run.AccuracyDelta
		}
	}
	return CampaignSummary{
		Seeds:              len(runs),
		PromotableCount:    promoted,
		AccuracyDeltaMean:  meanFloat(deltas),
		AccuracyDeltaStd:   stddevFloat(deltas),
		BestAccuracyDelta:  best,
		WorstAccuracyDelta: worst,
	}
}

func SelectPromotionDecision(runs []SeedRunResult) PromotionDecision {
	promotable := make([]SeedRunResult, 0, len(runs))
	for _, run := range runs {
		if run.Promotable {
			promotable = append(promotable, run)
		}
	}
	decision := PromotionDecision{
		CandidateCount: len(runs),
	}
	for _, run := range promotable {
		decision.PromotableSeeds = append(decision.PromotableSeeds, run.Seed)
	}
	if len(promotable) == 0 {
		decision.Promoted = false
		decision.Reason = "no_candidate_passed_promotion_gate"
		return decision
	}

	sort.Slice(promotable, func(i, j int) bool {
		a := promotable[i]
		b := promotable[j]
		if a.AccuracyDelta != b.AccuracyDelta {
			return a.AccuracyDelta > b.AccuracyDelta
		}
		if a.ReconDelta != b.ReconDelta {
			return a.ReconDelta < b.ReconDelta
		}
		if a.ErrorsDelta != b.ErrorsDelta {
			return a.ErrorsDelta < b.ErrorsDelta
		}
		return a.LatencyDeltaMS < b.LatencyDeltaMS
	})

	seed := promotable[0].Seed
	decision.Promoted = true
	decision.SelectedSeed = &seed
	decision.SelectedDelta = promotable[0].AccuracyDelta
	decision.Reason = "selected_best_promotable_candidate"
	return decision
}

// ---------------------------------------------------------------------------
// Artifact writers
// ---------------------------------------------------------------------------

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

func WriteOptimizationCampaignArtifacts(
	artifactDir string,
	modelID string,
	datasetPath string,
	cfg *optimizers.GEPAConfig,
	runs []SeedRunResult,
	summary CampaignSummary,
	decision PromotionDecision,
) error {
	if artifactDir == "" {
		artifactDir = DefaultArtifactsDir
	}
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return err
	}

	runsJSON, err := json.MarshalIndent(runs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal runs json: %w", err)
	}
	summaryJSON, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal summary json: %w", err)
	}
	decisionJSON, err := json.MarshalIndent(decision, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal decision json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, filepath.Base(DefaultRunsPath)), runsJSON, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(artifactDir, filepath.Base(DefaultSummaryPath)), summaryJSON, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(artifactDir, filepath.Base(DefaultDecisionPath)), decisionJSON, 0o644); err != nil {
		return err
	}

	if !decision.Promoted || decision.SelectedSeed == nil {
		return nil
	}
	var selected *SeedRunResult
	for i := range runs {
		if runs[i].Seed == *decision.SelectedSeed {
			selected = &runs[i]
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("selected seed %d not found in run set", *decision.SelectedSeed)
	}

	return WriteGEPAArtifacts(
		artifactDir,
		modelID,
		datasetPath,
		cfg,
		selected.CompiledResult,
		selected.BaselineEval,
		selected.CompiledEval,
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

	content := fmt.Sprintf(`# GEPA Full Pipeline Results

## Setup
- model: %s
- optimizer: GEPA
- objective: full pipeline (segment + translate) prompt optimization with judge LLM
- dataset source: %s
- dataset size (paragraph units): %d

## GEPA Config
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

// ---------------------------------------------------------------------------
// Helpers: accuracy, latency, misc
// ---------------------------------------------------------------------------

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

func extractInstructionFromProgram(program core.Program, moduleName string) string {
	mod, ok := program.Modules[moduleName]
	if !ok || mod == nil {
		return ""
	}
	return strings.TrimSpace(mod.GetSignature().Instruction)
}

var segmentsKeyPrefix = regexp.MustCompile(`(?i)^\s*segments\s*:\s*`)

func isMetadataSegment(s string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(s))
	return trimmed == "segments" || trimmed == "segments:" || trimmed == "segments: "
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
			if s != "" && !isMetadataSegment(s) {
				out = append(out, s)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(items))
		for _, it := range items {
			s := strings.TrimSpace(it)
			if s != "" && !isMetadataSegment(s) {
				out = append(out, s)
			}
		}
		return out
	default:
		s := strings.TrimSpace(toString(v))
		if s == "" {
			return nil
		}
		return parseSegmentsString(s)
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
	if segments := parseNewlineSegments(raw); len(segments) > 0 {
		return segments
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return parseSegments(payload["segments"])
}

func extractJSONArray(s string) []any {
	start := strings.Index(s, "[")
	if start < 0 {
		return nil
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				var out []any
				if err := json.Unmarshal([]byte(s[start:i+1]), &out); err == nil {
					return out
				}
				return nil
			}
		}
	}
	return nil
}

func parseSegmentsString(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	raw = strings.TrimSpace(segmentsKeyPrefix.ReplaceAllString(raw, ""))
	if raw == "" {
		return nil
	}

	var listPayload []any
	if err := json.Unmarshal([]byte(raw), &listPayload); err == nil {
		return parseSegments(listPayload)
	}
	var mapPayload map[string]any
	if err := json.Unmarshal([]byte(raw), &mapPayload); err == nil {
		if segs := parseSegments(mapPayload["segments"]); len(segs) > 0 {
			return segs
		}
	}
	if arr := extractJSONArray(raw); len(arr) > 0 {
		return parseSegments(arr)
	}
	return nil
}

func parseNewlineSegments(raw string) []string {
	raw = strings.TrimSpace(segmentsKeyPrefix.ReplaceAllString(raw, ""))
	if raw == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if s != "" && !isMetadataSegment(s) {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
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

// Suppress unused warning for minFloat.
var _ = minFloat

func meanFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func stddevFloat(values []float64) float64 {
	if len(values) <= 1 {
		return 0
	}
	mean := meanFloat(values)
	var sq float64
	for _, v := range values {
		d := v - mean
		sq += d * d
	}
	return math.Sqrt(sq / float64(len(values)))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
