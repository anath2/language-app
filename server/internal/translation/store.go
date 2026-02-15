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
	FullTranslation *string
	ErrorMessage    *string
	Paragraphs      []ParagraphResult
	Progress        int
	Total           int
}

type SegmentResult struct {
	Segment string `json:"segment"`
	Pinyin  string `json:"pinyin"`
	English string `json:"english"`
}

type ParagraphResult struct {
	Translations []SegmentResult `json:"translations"`
	Indent       string          `json:"indent"`
	Separator    string          `json:"separator"`
}

type SegmentProgressEntry struct {
	Segment       string
	Pinyin        string
	English       string
	Index         int
	SentenceIndex int
}

type ProgressSnapshot struct {
	Status  string
	Current int
	Total   int
	Results []SegmentProgressEntry
	Error   string
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
