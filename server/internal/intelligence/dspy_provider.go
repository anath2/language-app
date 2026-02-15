// TODO: prompt hardeining for structured outputs
package intelligence

import (
	"bytes"
	"context"
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
	"github.com/anath2/language-app/internal/translation"
)

const llmTimeout = 10 * time.Minute
const defaultSegmentationInstruction = "Split the Chinese text into meaningful segments of words and return segments as an ordered JSON array."

type DSPyProvider struct {
	segmenter  *modules.Predict
	translator *modules.Predict
	cedict     *cedictDictionary
}

func NewDSPyProvider(cfg config.Config) (*DSPyProvider, error) {
	llms.EnsureFactory()

	modelID := core.ModelID(strings.TrimSpace(cfg.OpenAIModel))
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

	translateSig := core.NewSignature(
		[]core.InputField{
			{Field: core.NewField("segment", core.WithDescription("Single Chinese segment"))},
			{Field: core.NewField("sentence_context", core.WithDescription("Original sentence context that may help disambiguation"))},
			{Field: core.NewField("dictionary_entry", core.WithDescription("CC-CEDICT dictionary definition for the segment, if available"))},
		},
		[]core.OutputField{
			{Field: core.NewField("pinyin", core.WithDescription("Pinyin transliteration for the segment"))},
			{Field: core.NewField("english", core.WithDescription("Short natural English translation for the segment"))},
		},
	).WithInstruction("Return concise translation data for the segment. Keep output JSON structured.")

	segmenter := modules.NewPredict(segmentSig).WithStructuredOutput()
	segmenter.SetLLM(openAILLM)

	translator := modules.NewPredict(translateSig).WithStructuredOutput()
	translator.SetLLM(openAILLM)

	cedict, err := loadCedictDictionary(cfg.CedictPath)
	if err != nil {
		log.Printf("cedict load warning: path=%s err=%v", cfg.CedictPath, err)
	}

	return &DSPyProvider{
		segmenter:  segmenter,
		translator: translator,
		cedict:     cedict,
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
	out := make([]string, 0, 3)
	if cedictPath := strings.TrimSpace(cfg.CedictPath); cedictPath != "" {
		// Typical layout: server/data/cedict_ts.u8 -> server/data/jepa/compiled_instruction.txt.
		out = append(out, filepath.Join(filepath.Dir(cedictPath), "jepa", "compiled_instruction.txt"))
	}
	out = append(out,
		filepath.Join("server", "data", "jepa", "compiled_instruction.txt"),
		filepath.Join("data", "jepa", "compiled_instruction.txt"),
	)
	return out
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

func (p *DSPyProvider) TranslateSegments(ctx context.Context, segments []string, sentenceContext string) ([]translation.SegmentResult, error) {
	out := make([]translation.SegmentResult, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		if shouldSkipSegment(segment) {
			out = append(out, translation.SegmentResult{
				Segment: segment,
				Pinyin:  "",
				English: "",
			})
			continue
		}

		dictPinyin, dictEntry := p.lookupCedict(segment)
		res, err := p.translator.Process(ctx, map[string]any{
			"segment":          segment,
			"sentence_context": sentenceContext,
			"dictionary_entry": dictEntry,
		})
		if err != nil {
			log.Printf("dspy translate fallback activated: err=%v segment=%q context_preview=%q", err, segment, preview(sentenceContext, 60))
			out = append(out, fallbackTranslationWithPinyin(segment, dictPinyin))
			continue
		}

		pinyin := strings.TrimSpace(dictPinyin)
		if pinyin == "" {
			pinyin = strings.TrimSpace(toString(res["pinyin"]))
		}
		english := strings.TrimSpace(toString(res["english"]))
		if english == "" {
			respPinyin, respEnglish := parseTranslationFromResponse(res["response"])
			if pinyin == "" && respPinyin != "" {
				pinyin = respPinyin
			}
			if respEnglish != "" {
				english = respEnglish
			}
		}
		if english == "" {
			log.Printf("dspy translate fallback activated: empty english segment=%q raw_response=%v", segment, res)
			out = append(out, fallbackTranslationWithPinyin(segment, dictPinyin))
			continue
		}

		out = append(out, translation.SegmentResult{
			Segment: segment,
			Pinyin:  pinyin,
			English: english,
		})
	}
	return out, nil
}

func fallbackTranslation(segment string) translation.SegmentResult {
	return fallbackTranslationWithPinyin(segment, "")
}

func fallbackTranslationWithPinyin(segment string, pinyin string) translation.SegmentResult {
	return translation.SegmentResult{
		Segment: segment,
		Pinyin:  strings.TrimSpace(pinyin),
		English: "translation_of_" + segment,
	}
}

func (p *DSPyProvider) lookupCedict(segment string) (string, string) {
	if p == nil || p.cedict == nil {
		return "", "Not in dictionary"
	}
	entry, ok := p.cedict.Lookup(segment)
	if !ok {
		return "", "Not in dictionary"
	}
	definition := strings.TrimSpace(entry.Definition)
	if definition == "" {
		definition = "Not in dictionary"
	}
	return strings.TrimSpace(entry.Pinyin), definition
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
