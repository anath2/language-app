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
	UpdateTranslationSegments(translationID string, sentenceIdx int, segments []translation.SegmentResult) error
	UpdateInputTextForReprocessing(id string, newText string) (map[int]string, error)
	EnsureChatForTranslation(translationID string) (translation.ChatThread, error)
	AppendChatMessage(translationID string, role string, content string, selectedSegmentIDs []string) (translation.ChatMessage, error)
	ListChatMessages(translationID string) ([]translation.ChatMessage, error)
	ClearChatMessages(translationID string) error
	LoadSelectedSegmentsByIDs(translationID string, segmentIDs []string) ([]translation.SegmentResult, error)
	SetReviewCard(messageID, chineseText, pinyin, english string) error
	GetMessageReviewCard(messageID string) (*translation.ChatReviewCard, error)
	AcceptMessageReviewCard(messageID string) error
	RejectMessageReviewCard(messageID string) error
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
	ExtractAndLinkCharacters(vocabItemID string, headword string, cedictLookup func(string) (string, string, bool)) error
	GetCharacterReviewQueue(limit int) ([]translation.CharacterReviewCard, error)
	GetCharacterDueCount() int
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
var translationProvider intelligence.TranslationProvider
var chatProvider intelligence.ChatProvider

func ConfigureDependencies(
	ts translationStore,
	te textEventStore,
	ss srsStore,
	ps profileStore,
	manager *queue.Manager,
	tp intelligence.TranslationProvider,
	cp intelligence.ChatProvider,
) {
	sharedTranslations = ts
	sharedTextEvents = te
	sharedSRS = ss
	sharedProfile = ps
	sharedQueue = manager
	translationProvider = tp
	chatProvider = cp
}

func validateDependencies() error {
	if sharedTranslations == nil || sharedTextEvents == nil || sharedSRS == nil || sharedProfile == nil || sharedQueue == nil || translationProvider == nil || chatProvider == nil {
		return errors.New("application dependencies are not configured")
	}
	return nil
}
