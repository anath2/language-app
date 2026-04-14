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

	"github.com/anath2/language-app/internal/config"
	store "github.com/anath2/language-app/internal/translation"
)

const llmTimeout = 10 * time.Minute
const defaultSegmentationInstruction = "Split the Chinese text into meaningful segments of words and return segments as an ordered JSON array."

// Provider calls an OpenAI-compatible /chat/completions endpoint directly
// and uses response_format: json_schema for structured output.
// It implements intelligence.TranslationProvider.
type Provider struct {
	client      *http.Client
	baseURL     string
	apiKey      string
	model       string
	instruction string
}

func NewProvider(cfg config.Config) (*Provider, error) {
	baseURL, _, err := normalizeOpenAIEndpoint(cfg.OpenAIBaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid OPENAI_BASE_URL %q: %w", cfg.OpenAIBaseURL, err)
	}
	var transport http.RoundTripper = http.DefaultTransport
	if cfg.OpenAIDebugLog {
		transport = &openAIDebugRoundTripper{base: transport}
		log.Printf("openai-compatible debug enabled: base_url=%s model=%s", baseURL, cfg.OpenAITranslationModel)
	}
	return &Provider{
		client:      &http.Client{Timeout: llmTimeout, Transport: transport},
		baseURL:     baseURL,
		apiKey:      cfg.OpenAIAPIKey,
		model:       strings.TrimSpace(cfg.OpenAITranslationModel),
		instruction: loadCompiledSegmentationInstruction(cfg),
	}, nil
}

// ---- JSON schemas ----

var segmentationSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"segments": map[string]any{
			"type":  "array",
			"items": map[string]any{"type": "string"},
		},
	},
	"required":             []string{"segments"},
	"additionalProperties": false,
}

var batchTranslationSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"translations": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pinyin":  map[string]any{"type": "string"},
					"english": map[string]any{"type": "string"},
				},
				"required":             []string{"pinyin", "english"},
				"additionalProperties": false,
			},
		},
	},
	"required":             []string{"translations"},
	"additionalProperties": false,
}

var fullTranslationSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"translation": map[string]any{"type": "string"},
	},
	"required":             []string{"translation"},
	"additionalProperties": false,
}

// ---- TranslationProvider implementation ----

func (p *Provider) Segment(ctx context.Context, text string) ([]string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{}, nil
	}
	content, err := p.complete(ctx, p.instruction, text, segmentationSchema, "segmentation_result")
	if err != nil {
		log.Printf("segment failed: err=%v text_preview=%q", err, preview(text, 40))
		return nil, fmt.Errorf("segment text: %w", err)
	}
	segments, err := parseSegmentsResult(content)
	if err != nil {
		log.Printf("segment parse failed: err=%v text_preview=%q content=%q", err, preview(text, 40), content)
		return nil, fmt.Errorf("segment text: %w", err)
	}
	return segments, nil
}

func (p *Provider) TranslateSegments(ctx context.Context, segments []string, sentence string, fullText string) ([]store.SegmentResult, error) {
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
	userMsg, err := json.Marshal(map[string]any{
		"segments":  string(segJSON),
		"sentence":  sentence,
		"full_text": fullText,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal translate request: %w", err)
	}

	const systemPrompt = "Given an array of Chinese word segments from a sentence, produce the pinyin (with tone marks) and a concise English translation for each segment. Use the sentence and full text for context to select the correct reading and meaning. Return a JSON object with a \"translations\" array of objects with \"pinyin\" and \"english\" fields, in the same order as the input segments."
	content, err := p.complete(ctx, systemPrompt, string(userMsg), batchTranslationSchema, "batch_translation_result")
	if err != nil {
		return nil, fmt.Errorf("batch translate: %w", err)
	}
	translations, err := parseBatchTranslationsResult(content)
	if err != nil {
		return nil, fmt.Errorf("batch translate: %w", err)
	}

	for i, cs := range cjkSegments {
		result := store.SegmentResult{Segment: cs.segment}
		if i < len(translations) {
			result.Pinyin = normalizeModelField(translations[i].Pinyin)
			result.English = normalizeModelField(translations[i].English)
		}
		out[cs.originalIdx] = result
	}
	return out, nil
}

func (p *Provider) TranslateFull(ctx context.Context, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	const systemPrompt = "Return concise translation data for the full text as a JSON object with a \"translation\" field."
	content, err := p.complete(ctx, systemPrompt, text, fullTranslationSchema, "full_translation_result")
	if err != nil {
		return "", fmt.Errorf("translate full text: %w", err)
	}
	return parseFullTranslationResult(content)
}

// ---- HTTP helper ----

type chatCompletionRequest struct {
	Model          string         `json:"model"`
	Messages       []chatMessage  `json:"messages"`
	ResponseFormat responseFormat `json:"response_format"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type       string     `json:"type"`
	JSONSchema jsonSchema `json:"json_schema"`
}

type jsonSchema struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

func (p *Provider) complete(ctx context.Context, systemPrompt, userPrompt string, schema map[string]any, schemaName string) (string, error) {
	reqBody := chatCompletionRequest{
		Model: p.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		ResponseFormat: responseFormat{
			Type: "json_schema",
			JSONSchema: jsonSchema{
				Name:   schemaName,
				Strict: true,
				Schema: schema,
			},
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("upstream request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read upstream response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		snippet := strings.TrimSpace(string(respBody))
		if len(snippet) > 300 {
			snippet = snippet[:300] + "..."
		}
		return "", fmt.Errorf("upstream returned status %d: %s", resp.StatusCode, snippet)
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("parse upstream response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in upstream response")
	}
	return chatResp.Choices[0].Message.Content, nil
}

// ---- Startup helpers ----

func loadCompiledSegmentationInstruction(cfg config.Config) string {
	for _, path := range []string{
		filepath.Join("data", "jepa", "compiled_instruction.txt"),
		filepath.Join("server", "data", "jepa", "compiled_instruction.txt"),
	} {
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
	log.Printf("compiled segmentation instruction not found, using default")
	return defaultSegmentationInstruction
}

// ---- Debug transport ----

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
		log.Printf("openai-compatible upstream request failed: method=%s url=%s err=%v elapsed_ms=%d",
			req.Method, req.URL.String(), err, time.Since(start).Milliseconds())
		return nil, err
	}
	log.Printf("openai-compatible upstream response: method=%s url=%s status=%d elapsed_ms=%d",
		req.Method, req.URL.String(), resp.StatusCode, time.Since(start).Milliseconds())
	if resp.StatusCode >= 200 && resp.StatusCode < 300 || resp.Body == nil {
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
	baseURL := strings.TrimRight(strings.TrimSpace(rawBaseURL), "/")
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
