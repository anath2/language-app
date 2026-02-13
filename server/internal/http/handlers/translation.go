package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/anath2/language-app/internal/translation"
)

type createTranslationRequest struct {
	InputText  string `json:"input_text"`
	SourceType string `json:"source_type"`
}

type createTranslationResponse struct {
	TranslationID string `json:"translation_id"`
	Status        string `json:"status"`
}

type translationSummary struct {
	ID                     string  `json:"id"`
	CreatedAt              string  `json:"created_at"`
	Status                 string  `json:"status"`
	SourceType             string  `json:"source_type"`
	InputPreview           string  `json:"input_preview"`
	FullTranslationPreview *string `json:"full_translation_preview"`
	SegmentCount           *int    `json:"segment_count"`
	TotalSegments          *int    `json:"total_segments"`
}

type listTranslationsResponse struct {
	Translations []translationSummary `json:"translations"`
	Total        int                  `json:"total"`
}

type translationDetailResponse struct {
	ID              string      `json:"id"`
	CreatedAt       string      `json:"created_at"`
	Status          string      `json:"status"`
	SourceType      string      `json:"source_type"`
	InputText       string      `json:"input_text"`
	FullTranslation *string     `json:"full_translation"`
	ErrorMessage    *string     `json:"error_message"`
	Paragraphs      interface{} `json:"paragraphs"`
}

type translationStatusResponse struct {
	TranslationID string `json:"translation_id"`
	Status        string `json:"status"`
	Progress      *int   `json:"progress"`
	Total         *int   `json:"total"`
}

func CreateTranslation(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	var req createTranslationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid JSON payload"})
		return
	}

	item, err := sharedStore.Create(req.InputText, req.SourceType)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}

	WriteJSON(w, http.StatusOK, createTranslationResponse{
		TranslationID: item.ID,
		Status:        item.Status,
	})

	sharedQueue.Submit(item.ID)
	sharedQueue.StartProcessing(item.ID)
}

func ListTranslations(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	query := r.URL.Query()
	limit := parseIntDefault(query.Get("limit"), 20)
	offset := parseIntDefault(query.Get("offset"), 0)
	status := strings.TrimSpace(query.Get("status"))

	items, total, err := sharedStore.List(limit, offset, status)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}

	summaries := make([]translationSummary, 0, len(items))
	for _, item := range items {
		summaries = append(summaries, translationSummary{
			ID:                     item.ID,
			CreatedAt:              item.CreatedAt,
			Status:                 item.Status,
			SourceType:             item.SourceType,
			InputPreview:           preview(item.InputText, 100),
			FullTranslationPreview: previewPtr(item.FullTranslation, 100),
			SegmentCount:           intPtrIfKnown(item.Progress, item.Status),
			TotalSegments:          intPtrIfKnown(item.Total, item.Status),
		})
	}

	WriteJSON(w, http.StatusOK, listTranslationsResponse{
		Translations: summaries,
		Total:        total,
	})
}

func GetTranslation(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	translationID := pathParam(r, "translation_id")
	item, ok := sharedStore.Get(translationID)
	if !ok {
		WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "Translation not found"})
		return
	}

	WriteJSON(w, http.StatusOK, translationDetailResponse{
		ID:              item.ID,
		CreatedAt:       item.CreatedAt,
		Status:          item.Status,
		SourceType:      item.SourceType,
		InputText:       item.InputText,
		FullTranslation: item.FullTranslation,
		ErrorMessage:    item.ErrorMessage,
		Paragraphs:      item.Paragraphs,
	})
}

func GetTranslationStatus(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	translationID := pathParam(r, "translation_id")
	item, ok := sharedStore.Get(translationID)
	if !ok {
		WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "Translation not found"})
		return
	}

	WriteJSON(w, http.StatusOK, translationStatusResponse{
		TranslationID: item.ID,
		Status:        item.Status,
		Progress:      intPtrIfKnown(item.Progress, item.Status),
		Total:         intPtrIfKnown(item.Total, item.Status),
	})
}

func DeleteTranslation(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	translationID := pathParam(r, "translation_id")
	if !sharedStore.Delete(translationID) {
		WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "Translation not found"})
		return
	}
	sharedQueue.CleanupProgress(translationID)

	WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func TranslationStream(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
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

	translationID := pathParam(r, "translation_id")
	item, exists := sharedStore.Get(translationID)
	if !exists {
		emitSSE(w, map[string]any{"type": "error", "message": "Translation not found"})
		flusher.Flush()
		return
	}

	if item.Status == "failed" {
		emitSSE(w, map[string]any{"type": "error", "message": derefOr(item.ErrorMessage, "Translation failed")})
		flusher.Flush()
		return
	}

	if item.Status == "completed" {
		replayCompletedStream(w, flusher, item)
		sharedQueue.CleanupProgress(translationID)
		return
	}

	sharedQueue.StartProcessing(translationID)
	streamLiveProgress(r.Context(), w, flusher, translationID)
}

func streamLiveProgress(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, translationID string) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	startSent := false
	lastProgress := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			item, exists := sharedStore.Get(translationID)
			if !exists {
				emitSSE(w, map[string]any{"type": "error", "message": "Translation not found"})
				flusher.Flush()
				return
			}

			if item.Status == "failed" {
				emitSSE(w, map[string]any{"type": "error", "message": derefOr(item.ErrorMessage, "Translation failed")})
				flusher.Flush()
				sharedQueue.CleanupProgress(translationID)
				return
			}

			progress, ok := sharedQueue.GetProgress(translationID)
			if !ok {
				continue
			}

			if !startSent && progress.Total > 0 {
				emitSSE(w, map[string]any{
					"type":           "start",
					"translation_id": translationID,
					"total":          progress.Total,
					"paragraphs":     paragraphInfo(item.Paragraphs),
				})
				flusher.Flush()
				startSent = true
			}

			for i := lastProgress; i < len(progress.Results); i++ {
				result := progress.Results[i]
				emitSSE(w, map[string]any{
					"type":    "progress",
					"current": i + 1,
					"total":   progress.Total,
					"result": map[string]any{
						"segment":         result.Segment,
						"pinyin":          result.Pinyin,
						"english":         result.English,
						"index":           result.Index,
						"paragraph_index": result.ParagraphIndex,
					},
				})
				flusher.Flush()
			}
			lastProgress = len(progress.Results)

			if progress.Status == "completed" || item.Status == "completed" {
				fresh, _ := sharedStore.Get(translationID)
				emitSSE(w, map[string]any{
					"type":            "complete",
					"paragraphs":      fresh.Paragraphs,
					"fullTranslation": fresh.FullTranslation,
				})
				flusher.Flush()
				sharedQueue.CleanupProgress(translationID)
				return
			}
		}
	}
}

func replayCompletedStream(w http.ResponseWriter, flusher http.Flusher, item translation.Translation) {
	emitSSE(w, map[string]any{
		"type":            "start",
		"translation_id":  item.ID,
		"total":           item.Total,
		"paragraphs":      paragraphInfo(item.Paragraphs),
		"fullTranslation": item.FullTranslation,
	})
	flusher.Flush()

	current := 0
	for paraIdx, para := range item.Paragraphs {
		for _, seg := range para.Translations {
			current++
			emitSSE(w, map[string]any{
				"type":    "progress",
				"current": current,
				"total":   item.Total,
				"result": map[string]any{
					"segment":         seg.Segment,
					"pinyin":          seg.Pinyin,
					"english":         seg.English,
					"index":           current - 1,
					"paragraph_index": paraIdx,
				},
			})
			flusher.Flush()
		}
	}

	emitSSE(w, map[string]any{
		"type":            "complete",
		"paragraphs":      item.Paragraphs,
		"fullTranslation": item.FullTranslation,
	})
	flusher.Flush()
}

func emitSSE(w http.ResponseWriter, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		_, _ = fmt.Fprint(w, "data: {\"type\":\"error\",\"message\":\"Failed to encode SSE payload\"}\n\n")
		return
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}

func paragraphInfo(paragraphs []translation.ParagraphResult) []map[string]any {
	out := make([]map[string]any, 0, len(paragraphs))
	for _, para := range paragraphs {
		out = append(out, map[string]any{
			"segment_count": len(para.Translations),
			"indent":        para.Indent,
			"separator":     para.Separator,
		})
	}
	return out
}

func intPtrIfKnown(value int, status string) *int {
	if status == "pending" && value == 0 {
		return nil
	}
	v := value
	return &v
}

func previewPtr(value *string, max int) *string {
	if value == nil {
		return nil
	}
	out := preview(*value, max)
	return &out
}

func derefOr(v *string, fallback string) string {
	if v == nil || *v == "" {
		return fallback
	}
	return *v
}
