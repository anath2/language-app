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
	segmenter         *modules.Predict
	pinyinTranslator  *modules.Predict
	meaningTranslator *modules.Predict
	fullTranslator    *modules.Predict
	articleSuggester  *modules.Predict
	cedict            *cedictDictionary
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

	pinyinSig := core.NewSignature(
		[]core.InputField{
			{Field: core.NewField("segment", core.WithDescription("Single Chinese segment"))},
			{Field: core.NewField("sentence_context", core.WithDescription("Original sentence context for disambiguation"))},
		},
		[]core.OutputField{
			{Field: core.NewField("pinyin", core.WithDescription("Pinyin transliteration for the segment"))},
		},
	).WithInstruction("Return the pinyin transliteration for the Chinese segment given the sentence context. Keep output JSON structured.")

	meaningSig := core.NewSignature(
		[]core.InputField{
			{Field: core.NewField("segment", core.WithDescription("Single Chinese segment"))},
			{Field: core.NewField("sentence_context", core.WithDescription("Original sentence context for disambiguation"))},
		},
		[]core.OutputField{
			{Field: core.NewField("english", core.WithDescription("Short natural English translation for the segment"))},
		},
	).WithInstruction("Return a concise English translation for the Chinese segment given the sentence context. Keep output JSON structured.")

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

	pinyinTranslator := modules.NewPredict(pinyinSig).WithStructuredOutput()
	pinyinTranslator.SetLLM(openAILLM)

	meaningTranslator := modules.NewPredict(meaningSig).WithStructuredOutput()
	meaningTranslator.SetLLM(openAILLM)

	fullTranslator := modules.NewPredict(fullTranslateSig).WithStructuredOutput()
	fullTranslator.SetLLM(openAILLM)

	articleSuggestSig := core.NewSignature(
		[]core.InputField{
			{Field: core.NewField("topics", core.WithDescription("Comma-separated list of topics to find Chinese-language articles about"))},
			{Field: core.NewField("exclude_urls", core.WithDescription("Comma-separated list of URLs to exclude (already seen)"))},
		},
		[]core.OutputField{
			{Field: core.NewField("urls", core.WithDescription("JSON array of 10 real Chinese-language article URLs from major Chinese news/blog sites"))},
		},
	).WithInstruction("Suggest approximately 10 real, currently accessible Chinese-language article URLs from well-known Chinese websites (e.g. zhihu.com, 36kr.com, sspai.com, ifanr.com, people.com.cn) for the given topics. Return only a JSON array of URL strings. Do not include any URLs from the exclude list.")

	articleSuggester := modules.NewPredict(articleSuggestSig).WithStructuredOutput()
	articleSuggester.SetLLM(openAILLM)

	cedict, err := loadCedictDictionary(cfg.CedictPath)
	if err != nil {
		log.Printf("cedict load warning: path=%s err=%v", cfg.CedictPath, err)
	}

	return &DSPyProvider{
		segmenter:         segmenter,
		pinyinTranslator:  pinyinTranslator,
		meaningTranslator: meaningTranslator,
		fullTranslator:    fullTranslator,
		articleSuggester:  articleSuggester,
		cedict:            cedict,
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

func (p *DSPyProvider) TranslateSegments(ctx context.Context, segments []string, sentenceContext string) ([]store.SegmentResult, error) {
	out := make([]store.SegmentResult, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		if shouldSkipSegment(segment) {
			out = append(out, store.SegmentResult{
				Segment: segment,
				Pinyin:  "",
				English: "",
			})
			continue
		}

		pinyin := p.resolvePinyin(ctx, segment, sentenceContext)
		english := p.resolveMeaning(ctx, segment, sentenceContext)

		out = append(out, store.SegmentResult{
			Segment: segment,
			Pinyin:  pinyin,
			English: english,
		})
	}
	return out, nil
}

// resolvePinyin returns pinyin for a segment, using CEDICT when possible and
// falling back to the LLM only when CEDICT can't resolve it.
func (p *DSPyProvider) resolvePinyin(ctx context.Context, segment, sentenceContext string) string {
	if p.cedict != nil {
		if pinyin, ok := p.cedict.ComposeSegmentPinyin(segment); ok {
			return pinyin
		}
	}

	// CEDICT couldn't resolve — call LLM.
	res, err := p.pinyinTranslator.Process(ctx, map[string]any{
		"segment":          segment,
		"sentence_context": sentenceContext,
	})
	if err != nil {
		log.Printf("dspy pinyin failed: err=%v segment=%q", err, segment)
		return p.fallbackCedictPinyin(segment)
	}

	if pinyin := strings.TrimSpace(toString(res["pinyin"])); pinyin != "" {
		return normalizeModelField(pinyin)
	}
	respPinyin, _ := parseTranslationFromResponse(res["response"])
	if respPinyin != "" {
		return respPinyin
	}
	return p.fallbackCedictPinyin(segment)
}

// fallbackCedictPinyin returns the first CEDICT entry's pinyin even if ambiguous,
// as a last resort when LLM also failed.
func (p *DSPyProvider) fallbackCedictPinyin(segment string) string {
	if p.cedict == nil {
		return ""
	}
	entry, ok := p.cedict.LookupFirst(segment)
	if !ok {
		return ""
	}
	return entry.Pinyin
}

// resolveMeaning returns an English translation for a segment, using CEDICT
// when available and falling back to the LLM otherwise.
func (p *DSPyProvider) resolveMeaning(ctx context.Context, segment, sentenceContext string) string {
	if p.cedict != nil {
		entries, ok := p.cedict.Lookup(segment)
		if ok && len(entries) > 0 {
			return entries[0].Definition
		}
	}

	// Not in CEDICT — call LLM.
	res, err := p.meaningTranslator.Process(ctx, map[string]any{
		"segment":          segment,
		"sentence_context": sentenceContext,
	})
	if err != nil {
		log.Printf("dspy meaning failed: err=%v segment=%q", err, segment)
		return "Not in dictionary"
	}

	if english := strings.TrimSpace(toString(res["english"])); english != "" {
		return normalizeModelField(english)
	}
	_, respEnglish := parseTranslationFromResponse(res["response"])
	if respEnglish != "" {
		return respEnglish
	}
	return "Not in dictionary"
}

func (p *DSPyProvider) LookupCharacter(char string) (string, string, bool) {
	if p.cedict == nil {
		return "", "", false
	}
	runes := []rune(char)
	if len(runes) != 1 {
		return "", "", false
	}
	pinyin, hasPinyin := p.cedict.PreferredCharPinyin(runes[0])
	entry, hasEntry := p.cedict.LookupFirst(char)
	if !hasPinyin && !hasEntry {
		return "", "", false
	}
	english := ""
	if hasEntry {
		english = entry.Definition
	}
	return pinyin, english, true
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
	if t := strings.TrimSpace(toString(res["translation"])); t != "" {
		return t, nil
	}
	if t := parseFullTranslationFromResponse(res["response"]); t != "" {
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

func (p *DSPyProvider) SuggestArticleURLs(ctx context.Context, topics []string, existingURLs []string) ([]string, error) {
	topicsStr := strings.Join(topics, ", ")
	excludeStr := strings.Join(existingURLs, ", ")

	res, err := p.articleSuggester.Process(ctx, map[string]any{
		"topics":       topicsStr,
		"exclude_urls": excludeStr,
	})
	if err != nil {
		return nil, fmt.Errorf("suggest article urls: %w", err)
	}

	urls := parseURLList(res["urls"])
	if len(urls) == 0 {
		urls = parseURLList(res["response"])
	}
	return urls, nil
}

func parseURLList(raw any) []string {
	if raw == nil {
		return nil
	}
	s := toString(raw)
	s = strings.TrimSpace(s)
	// Try JSON array parse
	var urls []string
	if err := json.Unmarshal([]byte(s), &urls); err == nil {
		var valid []string
		for _, u := range urls {
			u = strings.TrimSpace(u)
			if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
				valid = append(valid, u)
			}
		}
		return valid
	}
	// Fallback: line-separated
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			out = append(out, line)
		}
	}
	return out
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
