package translation

import (
	"database/sql"
	"errors"
)

var ErrNotFound = errors.New("translation not found")

type Translation struct {
	ID              string
	CreatedAt       string
	Status          string
	SourceType      string
	InputText       string
	Title           string
	FullTranslation *string
	ErrorMessage    *string
	Sentences       []SentenceResult
	Progress        int
	Total           int
}

type SegmentResult struct {
	Segment string `json:"segment"`
	Pinyin  string `json:"pinyin"`
	English string `json:"english"`
}

type SentenceResult struct {
	Translations []SegmentResult `json:"translations"`
	Indent       string          `json:"indent"`
	Separator    string          `json:"separator"`
}

// SentenceInit carries formatting metadata for a sentence when creating sentence rows.
type SentenceInit struct {
	Indent    string
	Separator string
}

type SegmentProgressEntry struct {
	Segment       string
	Pinyin        string
	English       string
	Index         int
	SentenceIndex int
}

const (
	ChatRoleUser = "user"
	ChatRoleAI   = "ai"
	ChatRoleTool = "tool" // tool result message; one per tool call, owns review_card_json
)

type ProgressSnapshot struct {
	Status  string
	Current int
	Total   int
	Results []SegmentProgressEntry
	Error   string
}

type ChatThread struct {
	ID            string
	TranslationID string
	CreatedAt     string
	UpdatedAt     string
}

// ChatReviewCard is stored as JSON in translation_chat_messages.review_card_json.
// Status is either "pending" (awaiting user action) or "accepted" (saved to SRS).
// A NULL column means rejected or never generated, so ChatReviewCard is nil.
type ChatReviewCard struct {
	ChineseText string `json:"chinese_text"`
	Pinyin      string `json:"pinyin"`
	English     string `json:"english"`
	Status      string `json:"status"` // "pending" | "accepted"
}

type ChatMessage struct {
	ID            string          `json:"id"`
	ChatID        string          `json:"chat_id"`
	TranslationID string          `json:"translation_id"`
	MessageIdx    int             `json:"message_idx"`
	Role          string          `json:"role"`
	Content       string          `json:"content"`
	SelectedText  *string         `json:"selected_text,omitempty"`
	CreatedAt     string          `json:"created_at"`
	ReviewCard    *ChatReviewCard `json:"review_card,omitempty"`
}

type TextRecord struct {
	ID             string
	CreatedAt      string
	SourceType     string
	RawText        string
	NormalizedText string
	Metadata       map[string]any
}

type VocabRecord struct {
	ID       string
	Headword string
	Pinyin   string
	English  string
	Status   string
}

type VocabSRSInfo struct {
	VocabItemID  string
	Headword     string
	Pinyin       string
	English      string
	Opacity      float64
	IsStruggling bool
	Status       string
}

type ReviewCard struct {
	VocabItemID string
	Headword    string
	Pinyin      string
	English     string
	Snippets    []string
}

type ReviewAnswerResult struct {
	VocabItemID  string
	NextDueAt    *string
	IntervalDays float64
	RemainingDue int
}

type CharacterReviewCard struct {
	VocabItemID  string
	Character    string
	Pinyin       string
	English      string
	ExampleWords []CharacterExampleWord
}

type CharacterExampleWord struct {
	VocabItemID string
	Headword    string
	Pinyin      string
	English     string
}

type UserProfile struct {
	Name      string
	Email     string
	Language  string
	CreatedAt string
	UpdatedAt string
}

type DB struct {
	Conn *sql.DB
}

type TranslationStore struct {
	db *sql.DB
}

type SRSStore struct {
	db *sql.DB
}

type TextEventStore struct {
	db *sql.DB
}

type ProfileStore struct {
	db *sql.DB
}

func NewTranslationStore(db *DB) *TranslationStore {
	return &TranslationStore{db: db.Conn}
}

func NewSRSStore(db *DB) *SRSStore {
	return &SRSStore{db: db.Conn}
}

func NewTextEventStore(db *DB) *TextEventStore {
	return &TextEventStore{db: db.Conn}
}

func NewProfileStore(db *DB) *ProfileStore {
	return &ProfileStore{db: db.Conn}
}
