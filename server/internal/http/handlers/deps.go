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
	UpdateTitle(id string, title string) error
	UpdateInputTextForReprocessing(id string, newText string) (map[int]string, error)
}

type chatStore interface {
	EnsureChatForTranslation(translationID string) (translation.ChatThread, error)
	AppendChatMessage(translationID string, role string, content string, selectedText string) (translation.ChatMessage, error)
	ListChatMessages(translationID string) ([]translation.ChatMessage, error)
	ClearChatMessages(translationID string) error
	SetReviewCard(messageID, chineseText, pinyin, english string) error
	GetMessageReviewCard(messageID string) (*translation.ChatReviewCard, error)
	AcceptMessageReviewCard(messageID string) error
	RejectMessageReviewCard(messageID string) error
}

type srsStore interface {
	SaveSegment(headword string, pinyin string, english string, translationID *string, snippet *string, status string) (string, error)
	UpdateSegmentStatus(segmentID string, status string) error
	UpdateCharacterStatus(characterID string, status string) error
	RecordLookup(segmentID string) (translation.SegmentSRSInfo, bool)
	GetSegmentSRSInfo(headwords []string) ([]translation.SegmentSRSInfo, error)
	GetSegmentReviewQueue(limit int) ([]translation.SegmentReviewCard, error)
	GetSegmentDueCount() int
	RecordReviewAnswer(entityID string, entityType string, grade int) (translation.ReviewAnswerResult, bool, error)
	CountSegmentsByStatus(status string) int
	CountTotalSegments() int
	ExportProgressJSON() (string, error)
	ImportProgressJSON(input string) (map[string]int, error)
	ExtractAndLinkCharacters(segmentID string, segment string, segmentPinyin string, segmentEnglish string, charData []translation.CharTranslation) error
	GetCharacterReviewQueue(limit int) ([]translation.CharacterReviewCard, error)
	GetCharacterDueCount() int
}

type profileStore interface {
	GetUserProfile() (translation.UserProfile, bool)
	UpsertUserProfile(name string, email string, language string) (translation.UserProfile, error)
}

var translations translationStore
var chats chatStore
var srs srsStore
var profiles profileStore
var jobQueue *queue.Manager
var transProvider intelligence.TranslationProvider
var chatProvider intelligence.ChatProvider

func ConfigureDependencies(
	ts translationStore,
	cs chatStore,
	ss srsStore,
	ps profileStore,
	manager *queue.Manager,
	tp intelligence.TranslationProvider,
	cp intelligence.ChatProvider,
) {
	translations = ts
	chats = cs
	srs = ss
	profiles = ps
	jobQueue = manager
	transProvider = tp
	chatProvider = cp
}

func validateDependencies() error {
	if translations == nil || chats == nil || srs == nil || profiles == nil || jobQueue == nil || transProvider == nil || chatProvider == nil {
		return errors.New("application dependencies are not configured")
	}
	return nil
}
