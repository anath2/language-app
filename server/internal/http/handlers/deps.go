package handlers

import (
	"errors"

	"github.com/anath2/language-app/internal/intelligence"
	"github.com/anath2/language-app/internal/queue"
	"github.com/anath2/language-app/internal/translation"
)

type translationStore interface {
	Create(inputText string, sourceType string) (translation.Translation, error)
	List(limit int, offset int, status string) ([]translation.Translation, int, error)
	Get(id string) (translation.Translation, bool)
	Delete(id string) bool
	UpdateTranslationSegments(translationID string, paragraphIdx int, segments []translation.SegmentResult) error
}

type textEventStore interface {
	CreateText(rawText string, sourceType string, metadata map[string]any) (translation.TextRecord, error)
	GetText(textID string) (translation.TextRecord, bool)
	CreateEvent(eventType string, textID *string, segmentID *string, payload map[string]any) (string, error)
}

type srsStore interface {
	SaveVocabItem(headword string, pinyin string, english string, textID *string, segmentID *string, snippet *string, status string) (string, error)
	UpdateVocabStatus(vocabItemID string, status string) error
	RecordLookup(vocabItemID string) (translation.VocabSRSInfo, bool)
	GetVocabSRSInfo(headwords []string) ([]translation.VocabSRSInfo, error)
	GetReviewQueue(limit int) ([]translation.ReviewCard, error)
	GetDueCount() int
	RecordReviewAnswer(vocabItemID string, grade int) (translation.ReviewAnswerResult, bool, error)
	CountVocabByStatus(status string) int
	CountTotalVocab() int
	ExportProgressJSON() (string, error)
	ImportProgressJSON(input string) (map[string]int, error)
}

type profileStore interface {
	GetUserProfile() (translation.UserProfile, bool)
	UpsertUserProfile(name string, email string, language string) (translation.UserProfile, error)
}

var sharedTranslations translationStore
var sharedTextEvents textEventStore
var sharedSRS srsStore
var sharedProfile profileStore
var sharedQueue *queue.Manager
var sharedProvider intelligence.Provider

func ConfigureDependencies(
	translationStore translationStore,
	textEventStore textEventStore,
	srsStore srsStore,
	profileStore profileStore,
	manager *queue.Manager,
	provider intelligence.Provider,
) {
	sharedTranslations = translationStore
	sharedTextEvents = textEventStore
	sharedSRS = srsStore
	sharedProfile = profileStore
	sharedQueue = manager
	sharedProvider = provider
}

func validateDependencies() error {
	if sharedTranslations == nil || sharedTextEvents == nil || sharedSRS == nil || sharedProfile == nil || sharedQueue == nil || sharedProvider == nil {
		return errors.New("application dependencies are not configured")
	}
	return nil
}
