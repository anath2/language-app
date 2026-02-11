package intelligence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/XiaoConstantine/dspy-go/pkg/core"
	"github.com/XiaoConstantine/dspy-go/pkg/llms"
	"github.com/XiaoConstantine/dspy-go/pkg/modules"
	"github.com/anath2/language-app/internal/config"
	"github.com/anath2/language-app/internal/translation"
)

const llmTimeout = 2 * time.Second

type DSPyProvider struct {
	segmenter  *modules.Predict
	translator *modules.Predict
	cedict     *cedictDictionary
}

func NewDSPyProvider(cfg config.Config) (*DSPyProvider, error) {
	llms.EnsureFactory()

	modelID := core.ModelID(strings.TrimSpace(cfg.OpenRouterModel))
	baseURL, path := normalizeOpenAIEndpoint(cfg.OpenRouterBaseURL)
	options := []llms.OpenAIOption{
		llms.WithAPIKey(cfg.OpenRouterAPIKey),
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
	if cfg.OpenRouterDebugLog {
		client := openAILLM.GetHTTPClient()
		client.Timeout = llmTimeout
		client.Transport = &openRouterDebugRoundTripper{base: client.Transport}
		log.Printf("openrouter debug enabled: base_url=%s path=%s model=%s", baseURL, path, modelID)
	}

	segmentSig := core.NewSignature(
		[]core.InputField{
			{Field: core.NewField("text", core.WithDescription("Chinese sentence to segment"))},
		},
		[]core.OutputField{
			{Field: core.NewField("segments", core.WithDescription("Array of segmented words in order"))},
		},
	).WithInstruction("Split the Chinese text into meaningful segments and return segments as an ordered JSON array.")

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

func (p *DSPyProvider) Segment(ctx context.Context, text string) ([]string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{}, nil
	}

	res, err := p.segmenter.Process(ctx, map[string]any{"text": text})
	if err != nil {
		log.Printf("dspy segment fallback activated: err=%v text_preview=%q", err, preview(text, 40))
		return fallbackSegments(text), nil
	}
	segments := parseSegments(res["segments"])
	if len(segments) == 0 {
		segments = parseSegmentsFromResponse(res["response"])
	}
	if len(segments) == 0 {
		log.Printf("dspy segment fallback activated: empty segments text_preview=%q raw_response=%v", preview(text, 40), res)
		return fallbackSegments(text), nil
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
		s := strings.TrimSpace(toString(v))
		if s == "" {
			return nil
		}
		return fallbackSegments(s)
	}
}

func parseSegmentsFromResponse(v any) []string {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		segments := parseSegments(m["segments"])
		if len(segments) > 0 {
			return segments
		}
	}
	raw := strings.TrimSpace(toString(v))
	if raw == "" {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return parseSegments(payload["segments"])
}

func parseTranslationFromResponse(v any) (string, string) {
	if v == nil {
		return "", ""
	}
	if m, ok := v.(map[string]any); ok {
		return normalizeModelField(toString(m["pinyin"])), normalizeModelField(toString(m["english"]))
	}
	raw := strings.TrimSpace(toString(v))
	if raw == "" {
		return "", ""
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", ""
	}
	return normalizeModelField(toString(payload["pinyin"])), normalizeModelField(toString(payload["english"]))
}

func normalizeModelField(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "pinyin:") {
		value = strings.TrimSpace(value[len("pinyin:"):])
	}
	lower = strings.ToLower(value)
	if strings.HasPrefix(lower, "english:") {
		value = strings.TrimSpace(value[len("english:"):])
	}
	if strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")") && len(value) > 2 {
		value = strings.TrimSpace(value[1 : len(value)-1])
	}
	return value
}

func fallbackSegments(text string) []string {
	out := make([]string, 0, len([]rune(text)))
	for _, r := range []rune(text) {
		if unicode.IsSpace(r) {
			continue
		}
		out = append(out, string(r))
	}
	return out
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

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprintf("%v", t)
	}
}

func preview(s string, max int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "..."
}

func newOpenRouterDebugHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &openRouterDebugRoundTripper{
			base: http.DefaultTransport,
		},
	}
}

type openRouterDebugRoundTripper struct {
	base http.RoundTripper
}

func (rt *openRouterDebugRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := rt.base
	if base == nil {
		base = http.DefaultTransport
	}
	start := time.Now()
	resp, err := base.RoundTrip(req)
	if err != nil {
		log.Printf("openrouter upstream request failed: method=%s url=%s err=%v elapsed_ms=%d", req.Method, req.URL.String(), err, time.Since(start).Milliseconds())
		return nil, err
	}

	log.Printf("openrouter upstream response: method=%s url=%s status=%d elapsed_ms=%d", req.Method, req.URL.String(), resp.StatusCode, time.Since(start).Milliseconds())
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}

	if resp.Body == nil {
		return resp, nil
	}
	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		log.Printf("openrouter upstream non-2xx body read failed: status=%d err=%v", resp.StatusCode, readErr)
		resp.Body = io.NopCloser(bytes.NewReader(nil))
		return resp, nil
	}
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	snippet := strings.TrimSpace(string(bodyBytes))
	if len(snippet) > 1000 {
		snippet = snippet[:1000] + "..."
	}
	log.Printf("openrouter upstream non-2xx body: status=%d body=%s", resp.StatusCode, snippet)
	return resp, nil
}

func normalizeOpenAIEndpoint(rawBaseURL string) (string, string) {
	baseURL := strings.TrimSpace(rawBaseURL)
	baseURL = strings.TrimRight(baseURL, "/")
	defaultPath := "/v1/chat/completions"
	if baseURL == "" {
		return "https://api.openai.com", defaultPath
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return baseURL, defaultPath
	}
	path := strings.TrimRight(parsed.Path, "/")

	if strings.Contains(strings.ToLower(parsed.Host), "openrouter.ai") {
		switch {
		case path == "":
			parsed.Path = "/api/v1"
			return strings.TrimRight(parsed.String(), "/"), "/chat/completions"
		case strings.HasSuffix(path, "/api/v1"), strings.HasSuffix(path, "/v1"):
			return strings.TrimRight(parsed.String(), "/"), "/chat/completions"
		default:
			return strings.TrimRight(parsed.String(), "/"), defaultPath
		}
	}

	if strings.HasSuffix(path, "/v1") {
		return strings.TrimRight(parsed.String(), "/"), "/chat/completions"
	}
	return strings.TrimRight(parsed.String(), "/"), defaultPath
}
