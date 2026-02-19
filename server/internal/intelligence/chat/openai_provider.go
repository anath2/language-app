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

var reviewCardTool = map[string]any{
	"type": "function",
	"function": map[string]any{
		"name":        "create_review_card",
		"description": `Generate a Chinese practice sentence as a review card. 
Call this when the user asks to create either a 
- review card
- srs segment
- practice word/character/sentence/phrase/segment
- example word/character/sentence/phrase/segment
- character review card`,
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"chinese_text": map[string]any{"type": "string", "description": "A short Chinese practice sentence"},
				"pinyin":       map[string]any{"type": "string", "description": "Pinyin romanization of the Chinese text"},
				"english":      map[string]any{"type": "string", "description": "English translation of the Chinese text"},
			},
			"required": []string{"chinese_text", "pinyin", "english"},
		},
	},
}

// ChatWithTranslationContext implements intelligence.ChatProvider.
// It builds a messages array with a system prompt containing the article and
// highlighted segments, appends prior history turns, then streams the response
// token-by-token via onChunk.
func (p *Provider) ChatWithTranslationContext(ctx context.Context, req intelligence.ChatWithTranslationRequest, onChunk func(string) error) (intelligence.ChatResult, error) {
	userMessage := strings.TrimSpace(req.UserMessage)
	if userMessage == "" {
		return intelligence.ChatResult{}, fmt.Errorf("chat user message is required")
	}
	translationText := strings.TrimSpace(req.TranslationText)
	if translationText == "" {
		return intelligence.ChatResult{}, fmt.Errorf("translation text is required")
	}

	selectedJSON, err := json.Marshal(req.Selected)
	if err != nil {
		return intelligence.ChatResult{}, fmt.Errorf("marshal selected segments: %w", err)
	}

	systemPrompt := fmt.Sprintf(
		`You are a Chinese language learning tutor responding in a chat context.
Answer questions grounded in the following article and highlighted segments if available.
You will be provided a chat history of previous messages. Use the chat history for context only — respond solely to the most recent user message and do not re-answer prior messages.
Make sure you answer the question in a concise manner. When answering questions in target language, always provide pinyin or english translation.
When the user asks to:
- create a practice sentence, example sentence, or review card, use the create_review_card function.
- create a practice word, character, sentence, phrase, or segment, use the create_review_card function.
- create a example word, character, sentence, phrase, or segment, use the create_review_card function.
- create a character review card, use the create_review_card function.

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
		"tools":       []any{reviewCardTool},
		"tool_choice": "auto",
	})
	if err != nil {
		return intelligence.ChatResult{}, fmt.Errorf("marshal chat request: %w", err)
	}

	endpoint := strings.TrimRight(p.baseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return intelligence.ChatResult{}, fmt.Errorf("create chat request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return intelligence.ChatResult{}, fmt.Errorf("chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return intelligence.ChatResult{}, fmt.Errorf("chat upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var fullReply strings.Builder
	var toolCallName string
	var toolCallArgs strings.Builder

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
		content, toolName, toolArgs, err := extractDelta(payload)
		if err != nil {
			log.Printf("chat SSE parse error: %v payload=%q", err, payload)
			continue
		}
		if toolName != "" && toolCallName == "" {
			toolCallName = toolName
		}
		if toolArgs != "" {
			toolCallArgs.WriteString(toolArgs)
		}
		if content == "" {
			continue
		}
		fullReply.WriteString(content)
		if onChunk != nil {
			if err := onChunk(content); err != nil {
				return intelligence.ChatResult{Content: fullReply.String()}, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return intelligence.ChatResult{Content: fullReply.String()}, fmt.Errorf("reading chat stream: %w", err)
	}

	// If a tool call was accumulated, return it as a ToolCall result.
	// Use json.Decoder (not Unmarshal) so that if some providers send duplicate or
	// trailing JSON objects (e.g. "{"..."}{}"), we decode only the first valid one.
	if argsStr := toolCallArgs.String(); argsStr != "" {
		var args map[string]any
		if err := json.NewDecoder(strings.NewReader(argsStr)).Decode(&args); err != nil {
			return intelligence.ChatResult{}, fmt.Errorf("parse tool call arguments: %w", err)
		}
		return intelligence.ChatResult{
			ToolCall: &intelligence.ToolCallResult{
				Name:      toolCallName,
				Arguments: args,
			},
		}, nil
	}

	reply := fullReply.String()
	if reply == "" {
		return intelligence.ChatResult{}, fmt.Errorf("chat with translation context: empty response")
	}
	return intelligence.ChatResult{Content: reply}, nil
}

// sseChunk is the minimal structure needed to extract delta content and tool calls from an SSE line.
type sseChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
}

// extractDelta returns (content, toolCallName, toolCallArgs, error).
func extractDelta(payload string) (string, string, string, error) {
	var chunk sseChunk
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
		return "", "", "", fmt.Errorf("unmarshal SSE chunk: %w", err)
	}
	if len(chunk.Choices) == 0 {
		return "", "", "", nil
	}
	delta := chunk.Choices[0].Delta
	if len(delta.ToolCalls) > 0 {
		tc := delta.ToolCalls[0]
		return "", tc.Function.Name, tc.Function.Arguments, nil
	}
	return delta.Content, "", "", nil
}
