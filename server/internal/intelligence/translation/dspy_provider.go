// TODO: Add a way to allow the user to create new segments for AI interactions in chat

// TODO: prompt hardening for structured outputs
package translation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/XiaoConstantine/dspy-go/pkg/core"
	"github.com/XiaoConstantine/dspy-go/pkg/llms"
	"github.com/XiaoConstantine/dspy-go/pkg/modules"
	"github.com/anath2/language-app/internal/config"
	store "github.com/anath2/language-app/internal/translation"
)

const llmTimeout = 10 * time.Minute
const defaultSegmentationInstruction = "Split the Chinese text into meaningful segments of words and return segments as an ordered JSON array."

type DSPyProvider struct {
	segmenter       *modules.Predict
	batchTranslator *modules.Predict
	fullTranslator  *modules.Predict
}

func NewDSPyProvider(cfg config.Config) (*DSPyProvider, error) {
	llms.EnsureFactory()

	modelID := core.ModelID(strings.TrimSpace(cfg.OpenAITranslationModel))
	baseURL, path, err := normalizeOpenAIEndpoint(cfg.OpenAIBaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid OPENAI_BASE_URL %q: %w", cfg.OpenAIBaseURL, err)
	}
	options := []llms.OpenAIOption{
		llms.WithAPIKey(cfg.OpenAIAPIKey),
		llms.WithOpenAIBaseURL(baseURL),
		llms.WithOpenAIPath(path),
		llms.WithOpenAITimeout(llmTimeout),
	}
	openAILLM, err := llms.NewOpenAILLM(
		modelID,
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize dspy-go llm: %w", err)
	}
	if cfg.OpenAIDebugLog {
		client := openAILLM.GetHTTPClient()
		client.Timeout = llmTimeout
		client.Transport = &openAIDebugRoundTripper{base: client.Transport}
		log.Printf("openai-compatible debug enabled: base_url=%s path=%s model=%s", baseURL, path, modelID)
	}

	segmentInstruction := loadCompiledSegmentationInstruction(cfg)
	segmentSig := core.NewSignature(
		[]core.InputField{
			{Field: core.NewField("text", core.WithDescription("Chinese sentence to segment"))},
		},
		[]core.OutputField{
			{Field: core.NewField("segments", core.WithDescription("Array of segmented words in order"))},
		},
	).WithInstruction(segmentInstruction)

	fullTranslateSig := core.NewSignature(
		[]core.InputField{
			{Field: core.NewField("text", core.WithDescription("Full Chinese text to translate"))},
		},
		[]core.OutputField{
			{Field: core.NewField("translation", core.WithDescription("English translation of the input text"))},
		},
	).WithInstruction("Return concise translation data for the full text. Keep output JSON structured.")

	segmenter := modules.NewPredict(segmentSig).WithStructuredOutput()
	segmenter.SetLLM(openAILLM)

	fullTranslator := modules.NewPredict(fullTranslateSig).WithStructuredOutput()
	fullTranslator.SetLLM(openAILLM)

	batchTranslateSig := core.NewSignature(
		[]core.InputField{
			{Field: core.NewField("segments_json", core.WithDescription("JSON array of Chinese segments to translate"))},
			{Field: core.NewField("sentence", core.WithDescription("The sentence containing the segments"))},
			{Field: core.NewField("full_text", core.WithDescription("The complete input text for broader context"))},
		},
		[]core.OutputField{
			{Field: core.NewField("translations_json", core.WithDescription("JSON array of {pinyin, english} objects in same order as input segments"))},
		},
	).WithInstruction("Given an array of Chinese word segments from a sentence, produce the pinyin (with tone marks) and a concise English translation for each segment. Use the sentence and full text for context to select the correct reading and meaning. Return a JSON array of objects with \"pinyin\" and \"english\" fields, in the same order as the input segments.")

	batchTranslator := modules.NewPredict(batchTranslateSig).WithStructuredOutput()
	batchTranslator.SetLLM(openAILLM)

	return &DSPyProvider{
		segmenter:       segmenter,
		batchTranslator: batchTranslator,
		fullTranslator:  fullTranslator,
	}, nil
}

func loadCompiledSegmentationInstruction(cfg config.Config) string {
	paths := candidateCompiledInstructionPaths(cfg)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		instruction := strings.TrimSpace(string(data))
		if instruction == "" {
			log.Printf("segmentation instruction file empty, falling back: path=%s", path)
			continue
		}
		log.Printf("loaded compiled segmentation instruction: path=%s", path)
		return instruction
	}
	log.Printf("compiled segmentation instruction not found, using default instruction")
	return defaultSegmentationInstruction
}

func candidateCompiledInstructionPaths(cfg config.Config) []string {
	return []string{
		filepath.Join("server", "data", "jepa", "compiled_instruction.txt"),
		filepath.Join("data", "jepa", "compiled_instruction.txt"),
	}
}

func (p *DSPyProvider) Segment(ctx context.Context, text string) ([]string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{}, nil
	}

	res, err := p.segmenter.Process(ctx, map[string]any{"text": text})
	if err != nil {
		log.Printf("dspy segment failed: err=%v text_preview=%q", err, preview(text, 40))
		return nil, fmt.Errorf("segment text with dspy: %w", err)
	}
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
		log.Printf("dspy segment failed: empty segments text_preview=%q raw_response=%v", preview(text, 40), res)
		return nil, fmt.Errorf("segment text with dspy: empty or invalid segments response")
	}
	return segments, nil
}

type batchTranslation struct {
	Pinyin  string `json:"pinyin"`
	English string `json:"english"`
}

func parseBatchTranslations(v any) []batchTranslation {
	if v == nil {
		return nil
	}
	var raw string
	switch t := v.(type) {
	case string:
		raw = t
	case []any:
		b, err := json.Marshal(t)
		if err != nil {
			return nil
		}
		raw = string(b)
	default:
		raw = normalizeJSONLikePayload(strings.TrimSpace(toString(v)))
	}
	raw = normalizeJSONLikePayload(strings.TrimSpace(raw))
	if raw == "" {
		return nil
	}
	var translations []batchTranslation
	if err := json.Unmarshal([]byte(raw), &translations); err != nil {
		if arr := extractJSONArray(raw); len(arr) > 0 {
			b, _ := json.Marshal(arr)
			_ = json.Unmarshal(b, &translations)
		}
	}
	return translations
}

func (p *DSPyProvider) TranslateSegments(ctx context.Context, segments []string, sentence string, fullText string) ([]store.SegmentResult, error) {
	type indexedSegment struct {
		originalIdx int
		segment     string
	}

	out := make([]store.SegmentResult, len(segments))
	var cjkSegments []indexedSegment

	for i, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" || shouldSkipSegment(seg) {
			out[i] = store.SegmentResult{Segment: seg}
			continue
		}
		cjkSegments = append(cjkSegments, indexedSegment{originalIdx: i, segment: seg})
	}

	if len(cjkSegments) == 0 {
		return out, nil
	}

	segStrings := make([]string, len(cjkSegments))
	for i, cs := range cjkSegments {
		segStrings[i] = cs.segment
	}

	segJSON, err := json.Marshal(segStrings)
	if err != nil {
		return nil, fmt.Errorf("marshal segments: %w", err)
	}

	res, err := p.batchTranslator.Process(ctx, map[string]any{
		"segments_json": string(segJSON),
		"sentence":      sentence,
		"full_text":     fullText,
	})
	if err != nil {
		return nil, fmt.Errorf("batch translate: %w", err)
	}

	translations := parseBatchTranslations(res["translations_json"])
	if len(translations) == 0 {
		translations = parseBatchTranslations(res["response"])
	}

	for i, cs := range cjkSegments {
		result := store.SegmentResult{Segment: cs.segment}
		if i < len(translations) {
			result.Pinyin = translations[i].Pinyin
			result.English = translations[i].English
		}
		out[cs.originalIdx] = result
	}

	return out, nil
}

func cleanFullTranslation(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		var decoded string
		if err := json.Unmarshal([]byte(s), &decoded); err == nil {
			return strings.TrimSpace(decoded)
		}
	}
	return s
}

func (p *DSPyProvider) TranslateFull(ctx context.Context, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	res, err := p.fullTranslator.Process(ctx, map[string]any{"text": text})
	if err != nil {
		return "", fmt.Errorf("translate full text with dspy: %w", err)
	}
	if t := cleanFullTranslation(toString(res["translation"])); t != "" {
		return t, nil
	}
	if t := cleanFullTranslation(parseFullTranslationFromResponse(res["response"])); t != "" {
		return t, nil
	}
	return "", fmt.Errorf("translate full text with dspy: empty translation response")
}

func preview(s string, max int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "..."
}

func newOpenAIDebugHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &openAIDebugRoundTripper{
			base: http.DefaultTransport,
		},
	}
}

type openAIDebugRoundTripper struct {
	base http.RoundTripper
}

func (rt *openAIDebugRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := rt.base
	if base == nil {
		base = http.DefaultTransport
	}
	start := time.Now()
	resp, err := base.RoundTrip(req)
	if err != nil {
		log.Printf("openai-compatible upstream request failed: method=%s url=%s err=%v elapsed_ms=%d", req.Method, req.URL.String(), err, time.Since(start).Milliseconds())
		return nil, err
	}

	log.Printf("openai-compatible upstream response: method=%s url=%s status=%d elapsed_ms=%d", req.Method, req.URL.String(), resp.StatusCode, time.Since(start).Milliseconds())
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}

	if resp.Body == nil {
		return resp, nil
	}
	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		log.Printf("openai-compatible upstream non-2xx body read failed: status=%d err=%v", resp.StatusCode, readErr)
		resp.Body = io.NopCloser(bytes.NewReader(nil))
		return resp, nil
	}
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	snippet := strings.TrimSpace(string(bodyBytes))
	if len(snippet) > 1000 {
		snippet = snippet[:1000] + "..."
	}
	log.Printf("openai-compatible upstream non-2xx body: status=%d body=%s", resp.StatusCode, snippet)
	return resp, nil
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
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", fmt.Errorf("must not include query string or fragment")
	}
	path := strings.TrimRight(parsed.Path, "/")
	if path == "" || !strings.HasSuffix(path, "/v1") {
		return "", "", fmt.Errorf("path must end with /v1")
	}
	if strings.Contains(path, "/chat/completions") {
		return "", "", fmt.Errorf("must be a base URL only; do not include /chat/completions")
	}

	parsed.Path = path
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), "/chat/completions", nil
}
