package handlers

import (
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
	SegmentID string `json:"segment_id"`
}

type updateVocabStatusRequest struct {
	SegmentID   string `json:"segment_id"`
	CharacterID string `json:"character_id"`
	Status      string `json:"status"`
}

type okResponse struct {
	Ok bool `json:"ok"`
}

type recordLookupRequest struct {
	SegmentID string `json:"segment_id"`
}

type recordLookupResponse struct {
	SegmentID    string  `json:"segment_id"`
	Opacity      float64 `json:"opacity"`
	IsStruggling bool    `json:"is_struggling"`
}

type vocabSRSInfoResponse struct {
	SegmentID    string  `json:"segment_id"`
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
	SegmentID string   `json:"segment_id"`
	Headword  string   `json:"headword"`
	Pinyin    string   `json:"pinyin"`
	English   string   `json:"english"`
	Snippets  []string `json:"snippets"`
}

type reviewQueueResponse struct {
	Cards    []reviewCardResponse `json:"cards"`
	DueCount int                  `json:"due_count"`
}

type reviewAnswerRequest struct {
	SegmentID   string `json:"segment_id"`
	CharacterID string `json:"character_id"`
	EntityType  string `json:"entity_type"`
	Grade       int    `json:"grade"`
}

type reviewAnswerResponse struct {
	SegmentID    *string `json:"segment_id,omitempty"`
	CharacterID  *string `json:"character_id,omitempty"`
	NextDueAt    *string `json:"next_due_at"`
	IntervalDays float64 `json:"interval_days"`
	RemainingDue int     `json:"remaining_due"`
}

type dueCountResponse struct {
	DueCount int `json:"due_count"`
}

type characterExampleSegmentResponse struct {
	SegmentID          string `json:"segment_id,omitempty"`
	Segment            string `json:"segment"`
	SegmentPinyin      string `json:"segment_pinyin"`
	SegmentTranslation string `json:"segment_translation"`
}

type characterReviewCardResponse struct {
	CharacterID     string                            `json:"character_id"`
	Character       string                            `json:"character"`
	Pinyin          string                            `json:"pinyin"`
	English         string                            `json:"english"`
	ExampleSegments []characterExampleSegmentResponse `json:"example_segments"`
}

type characterReviewQueueResponse struct {
	Cards    []characterReviewCardResponse `json:"cards"`
	DueCount int                           `json:"due_count"`
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
	id, err := srs.SaveSegment(req.Headword, req.Pinyin, req.English, req.TranslationID, req.Snippet, req.Status)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	_ = srs.ExtractAndLinkCharacters(id, req.Headword, req.Pinyin, req.English, nil)
	WriteJSON(w, http.StatusOK, saveVocabResponse{SegmentID: id})
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
	var err error
	if strings.TrimSpace(req.CharacterID) != "" {
		err = srs.UpdateCharacterStatus(req.CharacterID, req.Status)
	} else {
		err = srs.UpdateSegmentStatus(req.SegmentID, req.Status)
	}
	if err != nil {
		if err == translation.ErrNotFound {
			WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "Saved item not found"})
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
	info, ok := srs.RecordLookup(req.SegmentID)
	if !ok {
		WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "Segment not found"})
		return
	}
	WriteJSON(w, http.StatusOK, recordLookupResponse{
		SegmentID:    info.SegmentID,
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
	items, err := srs.GetSegmentSRSInfo(parts)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	resp := make([]vocabSRSInfoResponse, 0, len(items))
	for _, it := range items {
		resp = append(resp, vocabSRSInfoResponse{
			SegmentID:    it.SegmentID,
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
	cards, err := srs.GetSegmentReviewQueue(limit)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	respCards := make([]reviewCardResponse, 0, len(cards))
	for _, c := range cards {
		respCards = append(respCards, reviewCardResponse{
			SegmentID: c.SegmentID,
			Headword:  c.Headword,
			Pinyin:    c.Pinyin,
			English:   c.English,
			Snippets:  c.Snippets,
		})
	}
	WriteJSON(w, http.StatusOK, reviewQueueResponse{
		Cards:    respCards,
		DueCount: srs.GetSegmentDueCount(),
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
	entityID := strings.TrimSpace(req.SegmentID)
	entityType := strings.TrimSpace(req.EntityType)
	if strings.TrimSpace(req.CharacterID) != "" {
		entityID = strings.TrimSpace(req.CharacterID)
		if entityType == "" {
			entityType = "character"
		}
	} else if entityType == "" {
		entityType = "segment"
	}
	res, ok, err := srs.RecordReviewAnswer(entityID, entityType, req.Grade)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	if !ok {
		WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "Saved item not found"})
		return
	}
	WriteJSON(w, http.StatusOK, reviewAnswerResponse{
		SegmentID:    res.SegmentID,
		CharacterID:  res.CharacterID,
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
	WriteJSON(w, http.StatusOK, dueCountResponse{DueCount: srs.GetSegmentDueCount()})
}

func GetCharacterReviewQueue(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	limit := parseIntDefault(r.URL.Query().Get("limit"), 10)
	cards, err := srs.GetCharacterReviewQueue(limit)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	respCards := make([]characterReviewCardResponse, 0, len(cards))
	for _, c := range cards {
		examples := make([]characterExampleSegmentResponse, 0, len(c.ExampleSegments))
		for _, ex := range c.ExampleSegments {
			examples = append(examples, characterExampleSegmentResponse{
				SegmentID:          ex.SegmentID,
				Segment:            ex.Segment,
				SegmentPinyin:      ex.SegmentPinyin,
				SegmentTranslation: ex.SegmentTranslation,
			})
		}
		respCards = append(respCards, characterReviewCardResponse{
			CharacterID:     c.CharacterID,
			Character:       c.Character,
			Pinyin:          c.Pinyin,
			English:         c.English,
			ExampleSegments: examples,
		})
	}
	WriteJSON(w, http.StatusOK, characterReviewQueueResponse{
		Cards:    respCards,
		DueCount: srs.GetCharacterDueCount(),
	})
}

func GetCharacterReviewCount(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	WriteJSON(w, http.StatusOK, dueCountResponse{DueCount: srs.GetCharacterDueCount()})
}
