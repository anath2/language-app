package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/anath2/language-app/internal/intelligence"
	"github.com/anath2/language-app/internal/translation"
)

type createChatMessageRequest struct {
	Message            string   `json:"message"`
	SelectedSegmentIDs []string `json:"selected_segment_ids"`
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

	selected, err := sharedTranslations.LoadSelectedSegmentsByIDs(translationID, req.SelectedSegmentIDs)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": "One or more selected segments are invalid"})
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

	userMsg, err := sharedTranslations.AppendChatMessage(translationID, translation.ChatRoleUser, req.Message, req.SelectedSegmentIDs)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}

	history, err := sharedTranslations.ListChatMessages(translationID)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	selectedContext := make([]intelligence.ChatSegmentContext, 0, len(selected))
	for i, seg := range selected {
		if i >= len(userMsg.SelectedSegmentIDs) {
			break
		}
		selectedContext = append(selectedContext, intelligence.ChatSegmentContext{
			ID:      userMsg.SelectedSegmentIDs[i],
			Segment: seg.Segment,
			Pinyin:  seg.Pinyin,
			English: seg.English,
		})
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

	aiText, err := sharedProvider.ChatWithTranslationContext(r.Context(), intelligence.ChatWithTranslationRequest{
		TranslationText: item.InputText,
		UserMessage:     req.Message,
		Selected:        selectedContext,
		History:         history,
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
	})
	if err != nil {
		emitSSE(w, map[string]any{"type": "error", "message": err.Error()})
		flusher.Flush()
		return
	}

	aiMsg, err := sharedTranslations.AppendChatMessage(translationID, translation.ChatRoleAI, aiText, nil)
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
