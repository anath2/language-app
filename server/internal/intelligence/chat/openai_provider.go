package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/anath2/language-app/internal/config"
	"github.com/anath2/language-app/internal/intelligence"
)

const chatHTTPTimeout = 10 * time.Minute

// Provider implements intelligence.ChatProvider using raw OpenAI SSE streaming.
type Provider struct {
	httpClient *http.Client
	baseURL    string
	model      string
	apiKey     string
}

// New creates a chat Provider from config.
func New(cfg config.Config) *Provider {
	return &Provider{
		httpClient: &http.Client{Timeout: chatHTTPTimeout},
		baseURL:    cfg.OpenAIBaseURL,
		model:      cfg.OpenAIChatModel,
		apiKey:     cfg.OpenAIAPIKey,
	}
}

// ChatWithTranslationContext implements intelligence.ChatProvider.
// It builds a messages array with a system prompt containing the article and
// highlighted segments, appends prior history turns, then streams the response
// token-by-token via onChunk.
func (p *Provider) ChatWithTranslationContext(ctx context.Context, req intelligence.ChatWithTranslationRequest, onChunk func(string) error) (string, error) {
	userMessage := strings.TrimSpace(req.UserMessage)
	if userMessage == "" {
		return "", fmt.Errorf("chat user message is required")
	}
	translationText := strings.TrimSpace(req.TranslationText)
	if translationText == "" {
		return "", fmt.Errorf("translation text is required")
	}

	selectedJSON, err := json.Marshal(req.Selected)
	if err != nil {
		return "", fmt.Errorf("marshal selected segments: %w", err)
	}

	systemPrompt := fmt.Sprintf(
		`You are a Chinese language learning tutor responding in a chat context. 
Answer questions grounded in the following article and highlighted segments if available.
You will be provided a chat history of previous messages. Use the chat history for context only — respond solely to the most recent user message and do not re-answer prior messages.
Make sure you answer the question in a concise manner. When answering questions in target language, always provide pinyin or english translation.

## ARTICLE: 
%s
## HIGHLIGHTED SEGMENTS:
%s
`,
		translationText,
		string(selectedJSON),
	)

	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
	}
	for _, msg := range req.History {
		role := strings.ToLower(msg.Role)
		if role != "user" && role != "assistant" {
			continue
		}
		messages = append(messages, map[string]string{
			"role":    role,
			"content": msg.Content,
		})
	}
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": userMessage,
	})

	reasoning := map[string]any{
		"enabled": false,
	}

	body, err := json.Marshal(map[string]any{
		"model":       p.model,
		"messages":    messages,
		"stream":      true,
		"thinking":    false,
		"temperature": 0.7,
		"reasoning":   reasoning,
	})
	if err != nil {
		return "", fmt.Errorf("marshal chat request: %w", err)
	}

	endpoint := strings.TrimRight(p.baseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create chat request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("chat upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var fullReply strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}
		content, err := extractDeltaContent(payload)
		if err != nil {
			log.Printf("chat SSE parse error: %v payload=%q", err, payload)
			continue
		}
		if content == "" {
			continue
		}
		fullReply.WriteString(content)
		if onChunk != nil {
			if err := onChunk(content); err != nil {
				return fullReply.String(), err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fullReply.String(), fmt.Errorf("reading chat stream: %w", err)
	}

	reply := fullReply.String()
	if reply == "" {
		return "", fmt.Errorf("chat with translation context: empty response")
	}
	return reply, nil
}

// sseChunk is the minimal structure needed to extract delta content from an SSE line.
type sseChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func extractDeltaContent(payload string) (string, error) {
	var chunk sseChunk
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
		return "", fmt.Errorf("unmarshal SSE chunk: %w", err)
	}
	if len(chunk.Choices) == 0 {
		return "", nil
	}
	return chunk.Choices[0].Delta.Content, nil
}
