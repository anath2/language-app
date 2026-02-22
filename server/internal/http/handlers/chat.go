package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/anath2/language-app/internal/intelligence"
	"github.com/anath2/language-app/internal/translation"
)

type createChatMessageRequest struct {
	Message      string `json:"message"`
	SelectedText string `json:"selected_text"`
}

type chatListResponse struct {
	ChatID   string                    `json:"chat_id"`
	Messages []translation.ChatMessage `json:"messages"`
}

func CreateChatMessage(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	translationID := pathParam(r, "translation_id")
	item, exists := sharedTranslations.Get(translationID)
	if !exists {
		WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "Translation not found"})
		return
	}

	var req createChatMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid JSON payload"})
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": "message is required"})
		return
	}

	thread, err := sharedTranslations.EnsureChatForTranslation(translationID)
	if err != nil {
		if err == translation.ErrNotFound {
			WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "Translation not found"})
			return
		}
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	userMsg, err := sharedTranslations.AppendChatMessage(translationID, translation.ChatRoleUser, req.Message, req.SelectedText)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}

	history, err := sharedTranslations.ListChatMessages(translationID)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		emitSSE(w, map[string]any{"type": "error", "message": "Streaming is not supported"})
		return
	}

	emitSSE(w, map[string]any{
		"type":            "start",
		"translation_id":  translationID,
		"chat_id":         thread.ID,
		"user_message_id": userMsg.ID,
	})
	flusher.Flush()

	result, err := chatProvider.ChatWithTranslationContext(r.Context(), intelligence.ChatWithTranslationRequest{
		TranslationText: item.InputText,
		UserMessage:     req.Message,
		History:         history,
		SelectedText:    req.SelectedText,
	}, func(chunk string) error {
		if strings.TrimSpace(chunk) == "" {
			return nil
		}
		emitSSE(w, map[string]any{
			"type":  "chunk",
			"delta": chunk,
		})
		flusher.Flush()
		return nil
	}, func(toolName string) {
		emitSSE(w, map[string]any{
			"type":      "tool_call_start",
			"tool_name": toolName,
		})
		flusher.Flush()
	})
	if err != nil {
		emitSSE(w, map[string]any{"type": "error", "message": err.Error()})
		flusher.Flush()
		return
	}

	if len(result.ToolCalls) > 0 {
		// One AI text message for the whole turn.
		aiMsg, err := sharedTranslations.AppendChatMessage(translationID, translation.ChatRoleAI, "Here's a practice card for you:", "")
		if err != nil {
			emitSSE(w, map[string]any{"type": "error", "message": err.Error()})
			flusher.Flush()
			return
		}

		// One tool message per tool call — each owns its own review card.
		toolResults := make([]map[string]any, 0, len(result.ToolCalls))
		for _, tc := range result.ToolCalls {
			if tc.Name != "create_review_card" {
				continue
			}
			chineseText, _ := tc.Arguments["chinese_text"].(string)
			pinyin, _ := tc.Arguments["pinyin"].(string)
			english, _ := tc.Arguments["english"].(string)

			toolMsg, err := sharedTranslations.AppendChatMessage(translationID, translation.ChatRoleTool, chineseText, "")
			if err != nil {
				emitSSE(w, map[string]any{"type": "error", "message": err.Error()})
				flusher.Flush()
				return
			}
			if err := sharedTranslations.SetReviewCard(toolMsg.ID, chineseText, pinyin, english); err != nil {
				emitSSE(w, map[string]any{"type": "error", "message": err.Error()})
				flusher.Flush()
				return
			}
			toolResults = append(toolResults, map[string]any{
				"message_id": toolMsg.ID,
				"review_card": translation.ChatReviewCard{
					ChineseText: chineseText,
					Pinyin:      pinyin,
					English:     english,
					Status:      "pending",
				},
			})
		}
		emitSSE(w, map[string]any{
			"type":         "complete",
			"message_id":   aiMsg.ID,
			"content":      aiMsg.Content,
			"tool_results": toolResults,
		})
		flusher.Flush()
		return
	}

	aiMsg, err := sharedTranslations.AppendChatMessage(translationID, translation.ChatRoleAI, result.Content, "")
	if err != nil {
		emitSSE(w, map[string]any{"type": "error", "message": err.Error()})
		flusher.Flush()
		return
	}

	emitSSE(w, map[string]any{
		"type":       "complete",
		"message_id": aiMsg.ID,
		"content":    aiMsg.Content,
	})
	flusher.Flush()
}

func ListChatMessages(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	translationID := pathParam(r, "translation_id")
	thread, err := sharedTranslations.EnsureChatForTranslation(translationID)
	if err != nil {
		if err == translation.ErrNotFound {
			WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "Translation not found"})
			return
		}
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	items, err := sharedTranslations.ListChatMessages(translationID)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	WriteJSON(w, http.StatusOK, chatListResponse{
		ChatID:   thread.ID,
		Messages: items,
	})
}

func ClearChatMessages(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	translationID := pathParam(r, "translation_id")
	if err := sharedTranslations.ClearChatMessages(translationID); err != nil {
		if err == translation.ErrNotFound {
			WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "Translation not found"})
			return
		}
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func AcceptReviewCard(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	translationID := pathParam(r, "translation_id")
	messageID := pathParam(r, "message_id")

	card, err := sharedTranslations.GetMessageReviewCard(messageID)
	if err != nil {
		if err == translation.ErrNotFound {
			WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "Message not found"})
			return
		}
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if card == nil {
		WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "No review card on this message"})
		return
	}
	if card.Status == "accepted" {
		WriteJSON(w, http.StatusConflict, map[string]string{"detail": "Review card already accepted"})
		return
	}

	deduplicated := false
	existingItems, err := sharedSRS.GetVocabSRSInfo([]string{card.ChineseText})
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if len(existingItems) > 0 {
		deduplicated = true
	} else {
		if _, err := sharedSRS.SaveVocabItem(card.ChineseText, card.Pinyin, card.English, &translationID, nil, nil, "learning"); err != nil {
			WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
	}

	if err := sharedTranslations.AcceptMessageReviewCard(messageID); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "deduplicated": deduplicated})
}

func RejectReviewCard(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	messageID := pathParam(r, "message_id")

	card, err := sharedTranslations.GetMessageReviewCard(messageID)
	if err != nil {
		if err == translation.ErrNotFound {
			WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "Message not found"})
			return
		}
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if card == nil {
		WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "No review card on this message"})
		return
	}
	if card.Status == "accepted" {
		WriteJSON(w, http.StatusConflict, map[string]string{"detail": "Cannot reject an already accepted review card"})
		return
	}

	if err := sharedTranslations.RejectMessageReviewCard(messageID); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
