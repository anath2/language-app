package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/anath2/language-app/internal/translation"
)

type saveVocabRequest struct {
	Headword      string  `json:"headword"`
	Pinyin        string  `json:"pinyin"`
	English       string  `json:"english"`
	TranslationID *string `json:"translation_id"`
	Snippet       *string `json:"snippet"`
	Status        string  `json:"status"`
}

type saveVocabResponse struct {
	VocabItemID string `json:"vocab_item_id"`
}

type updateVocabStatusRequest struct {
	VocabItemID string `json:"vocab_item_id"`
	Status      string `json:"status"`
}

type okResponse struct {
	Ok bool `json:"ok"`
}

type recordLookupRequest struct {
	VocabItemID string `json:"vocab_item_id"`
}

type recordLookupResponse struct {
	VocabItemID  string  `json:"vocab_item_id"`
	Opacity      float64 `json:"opacity"`
	IsStruggling bool    `json:"is_struggling"`
}

type vocabSRSInfoResponse struct {
	VocabItemID  string  `json:"vocab_item_id"`
	Headword     string  `json:"headword"`
	Pinyin       string  `json:"pinyin"`
	English      string  `json:"english"`
	Opacity      float64 `json:"opacity"`
	IsStruggling bool    `json:"is_struggling"`
	Status       string  `json:"status"`
	IntervalDays float64 `json:"interval_days"`
	NextDueAt    *string `json:"next_due_at"`
}

type vocabSRSInfoListResponse struct {
	Items []vocabSRSInfoResponse `json:"items"`
}

type reviewCardResponse struct {
	VocabItemID string   `json:"vocab_item_id"`
	Headword    string   `json:"headword"`
	Pinyin      string   `json:"pinyin"`
	English     string   `json:"english"`
	Snippets    []string `json:"snippets"`
}

type reviewQueueResponse struct {
	Cards    []reviewCardResponse `json:"cards"`
	DueCount int                  `json:"due_count"`
}

type reviewAnswerRequest struct {
	VocabItemID string `json:"vocab_item_id"`
	Grade       int    `json:"grade"`
}

type reviewAnswerResponse struct {
	VocabItemID  string  `json:"vocab_item_id"`
	NextDueAt    *string `json:"next_due_at"`
	IntervalDays float64 `json:"interval_days"`
	RemainingDue int     `json:"remaining_due"`
}

type dueCountResponse struct {
	DueCount int `json:"due_count"`
}

type characterExampleWordResponse struct {
	VocabItemID string `json:"vocab_item_id"`
	Headword    string `json:"headword"`
	Pinyin      string `json:"pinyin"`
	English     string `json:"english"`
}

type characterReviewCardResponse struct {
	VocabItemID  string                         `json:"vocab_item_id"`
	Character    string                         `json:"character"`
	Pinyin       string                         `json:"pinyin"`
	English      string                         `json:"english"`
	ExampleWords []characterExampleWordResponse `json:"example_words"`
}

type characterReviewQueueResponse struct {
	Cards    []characterReviewCardResponse `json:"cards"`
	DueCount int                           `json:"due_count"`
}

type translateBatchRequest struct {
	Segments      []string `json:"segments"`
	Context       *string  `json:"context"`
	TranslationID *string  `json:"translation_id"`
	SentenceIdx   *int     `json:"sentence_idx"`
}

type translationResult struct {
	Segment string `json:"segment"`
	Pinyin  string `json:"pinyin"`
	English string `json:"english"`
}

type translateBatchResponse struct {
	Translations []translationResult `json:"translations"`
}

func SaveVocab(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	var req saveVocabRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid JSON payload"})
		return
	}
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	WriteJSON(w, http.StatusOK, saveVocabResponse{VocabItemID: id})
}

func UpdateVocabStatus(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	var req updateVocabStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid JSON payload"})
		return
	}
	if err != nil {
		if err == translation.ErrNotFound {
			WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "Vocab item not found"})
			return
		}
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	WriteJSON(w, http.StatusOK, okResponse{Ok: true})
}

func RecordLookup(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	var req recordLookupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid JSON payload"})
		return
	}
	if !ok {
		WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "Vocab item not found"})
		return
	}
	WriteJSON(w, http.StatusOK, recordLookupResponse{
		VocabItemID:  info.VocabItemID,
		Opacity:      info.Opacity,
		IsStruggling: info.IsStruggling,
	})
}

func GetVocabSRSInfo(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	headwords := strings.TrimSpace(r.URL.Query().Get("headwords"))
	if headwords == "" {
		WriteJSON(w, http.StatusOK, vocabSRSInfoListResponse{Items: []vocabSRSInfoResponse{}})
		return
	}
	parts := strings.Split(headwords, ",")
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	resp := make([]vocabSRSInfoResponse, 0, len(items))
	for _, it := range items {
		resp = append(resp, vocabSRSInfoResponse{
			VocabItemID:  it.VocabItemID,
			Headword:     it.Headword,
			Pinyin:       it.Pinyin,
			English:      it.English,
			Opacity:      it.Opacity,
			IsStruggling: it.IsStruggling,
			Status:       it.Status,
			IntervalDays: it.IntervalDays,
			NextDueAt:    it.NextDueAt,
		})
	}
	WriteJSON(w, http.StatusOK, vocabSRSInfoListResponse{Items: resp})
}

func GetReviewQueue(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	limit := parseIntDefault(r.URL.Query().Get("limit"), 10)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	respCards := make([]reviewCardResponse, 0, len(cards))
	for _, c := range cards {
		respCards = append(respCards, reviewCardResponse{
			VocabItemID: c.VocabItemID,
			Headword:    c.Headword,
			Pinyin:      c.Pinyin,
			English:     c.English,
			Snippets:    c.Snippets,
		})
	}
	WriteJSON(w, http.StatusOK, reviewQueueResponse{
		Cards:    respCards,
		DueCount: srs.GetDueCount(),
	})
}

func RecordReviewAnswer(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	var req reviewAnswerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid JSON payload"})
		return
	}
	res, ok, err := srs.RecordReviewAnswer(req.VocabItemID, req.Grade)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	if !ok {
		WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "Vocab item not found"})
		return
	}
	WriteJSON(w, http.StatusOK, reviewAnswerResponse{
		VocabItemID:  res.VocabItemID,
		NextDueAt:    res.NextDueAt,
		IntervalDays: res.IntervalDays,
		RemainingDue: res.RemainingDue,
	})
}

func GetReviewCount(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
}

func GetCharacterReviewQueue(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	limit := parseIntDefault(r.URL.Query().Get("limit"), 10)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	respCards := make([]characterReviewCardResponse, 0, len(cards))
	for _, c := range cards {
		examples := make([]characterExampleWordResponse, 0, len(c.ExampleWords))
		for _, ex := range c.ExampleWords {
			examples = append(examples, characterExampleWordResponse{
				VocabItemID: ex.VocabItemID,
				Headword:    ex.Headword,
				Pinyin:      ex.Pinyin,
				English:     ex.English,
			})
		}
		respCards = append(respCards, characterReviewCardResponse{
			VocabItemID:  c.VocabItemID,
			Character:    c.Character,
			Pinyin:       c.Pinyin,
			English:      c.English,
			ExampleWords: examples,
		})
	}
	WriteJSON(w, http.StatusOK, characterReviewQueueResponse{
		Cards:    respCards,
	})
}

func GetCharacterReviewCount(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
}

func TranslateBatch(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	var req translateBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid JSON payload"})
		return
	}
	translations := make([]translationResult, 0, len(req.Segments))
	segmentResults, err := translationProvider.TranslateSegments(context.Background(), req.Segments, derefOr(req.Context, ""))
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	storeSegments := make([]translation.SegmentResult, 0, len(segmentResults))
	for _, translated := range segmentResults {
		item := translationResult{
			Segment: translated.Segment,
			Pinyin:  translated.Pinyin,
			English: translated.English,
		}
		translations = append(translations, item)
		storeSegments = append(storeSegments, translated)
	}
	if req.TranslationID != nil && req.SentenceIdx != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
			return
		}
	}
	WriteJSON(w, http.StatusOK, translateBatchResponse{Translations: translations})
}
