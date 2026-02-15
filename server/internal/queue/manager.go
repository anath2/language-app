package queue

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/anath2/language-app/internal/intelligence"
	"github.com/anath2/language-app/internal/translation"
)

type SegmentProgress struct {
	Segment       string `json:"segment"`
	Pinyin        string `json:"pinyin"`
	English       string `json:"english"`
	Index         int    `json:"index"`
	SentenceIndex int    `json:"sentence_index"`
}

type Progress struct {
	Status  string            `json:"status"`
	Current int               `json:"current"`
	Total   int               `json:"total"`
	Results []SegmentProgress `json:"results"`
	Error   string            `json:"error,omitempty"`
}

type Manager struct {
	store    translationStore
	provider intelligence.Provider
	mu       sync.RWMutex
	running  map[string]struct{}
}

type translationStore interface {
	ListRestartableTranslationIDs() ([]string, error)
	Get(id string) (translation.Translation, bool)
	ClaimTranslationJob(translationID string, leaseDuration time.Duration) (bool, error)
	Fail(id string, message string) error
	SetProcessing(id string, total int, sentenceCount int) error
	Complete(id string) error
	GetProgressSnapshot(id string) (translation.ProgressSnapshot, bool)
	AddProgressSegment(id string, result translation.SegmentResult, sentenceIndex int) (int, int, error)
}

type queuedSegment struct {
	SentenceIndex int
	SentenceText  string
	Segment       string
}

const jobLeaseDuration = 30 * time.Second

func NewManager(store translationStore, provider intelligence.Provider) *Manager {
	return &Manager{
		store:    store,
		provider: provider,
		running:  make(map[string]struct{}),
	}
}

func (m *Manager) Submit(translationID string) {
	// Progress is persisted in the database; no in-memory state is required.
	_ = translationID
}

func (m *Manager) ResumeRestartableJobs() {
	ids, err := m.store.ListRestartableTranslationIDs()
	if err != nil {
		log.Printf("failed listing restartable translation jobs: %v", err)
		return
	}
	for _, translationID := range ids {
		m.StartProcessing(translationID)
	}
}

func (m *Manager) StartProcessing(translationID string) {
	item, ok := m.store.Get(translationID)
	if !ok {
		return
	}
	if item.Status == "completed" || item.Status == "failed" {
		return
	}

	m.mu.Lock()
	if _, exists := m.running[translationID]; exists {
		m.mu.Unlock()
		return
	}
	m.running[translationID] = struct{}{}
	m.mu.Unlock()

	claimed, err := m.store.ClaimTranslationJob(translationID, jobLeaseDuration)
	if err != nil || !claimed {
		m.removeRunning(translationID)
		return
	}

	go func(item translation.Translation) {
		ctx := context.Background()
		sentences := splitInputSentences(item.InputText)
		if len(sentences) == 0 {
			_ = m.store.Fail(translationID, "No sentences found for segmentation")
			m.removeRunning(translationID)
			return
		}
		queued, err := m.segmentInputBySentence(ctx, sentences)
		if err != nil {
			msg := err.Error()
			if len(msg) > 200 {
				msg = msg[:200] + "..."
			}
			_ = m.store.Fail(translationID, "Failed to segment: "+msg)
			m.removeRunning(translationID)
			return
		}
		total := len(queued)
		if total == 0 {
			_ = m.store.Fail(translationID, "No translatable segments found")
			m.removeRunning(translationID)
			return
		}

		startIndex := item.Progress
		if item.Status == "pending" {
			startIndex = 0
			if err := m.store.SetProcessing(translationID, total, len(sentences)); err != nil {
				m.removeRunning(translationID)
				return
			}
		}

		if startIndex >= len(queued) {
			if err := m.store.Complete(translationID); err != nil {
				_ = m.store.Fail(translationID, "Failed to complete translation")
			}
			m.removeRunning(translationID)
			return
		}

		m.runJob(ctx, translationID, queued, startIndex)
	}(item)
}

func (m *Manager) GetProgress(translationID string) (Progress, bool) {
	snapshot, ok := m.store.GetProgressSnapshot(translationID)
	if !ok {
		return Progress{}, false
	}
	progress := Progress{
		Status:  snapshot.Status,
		Current: snapshot.Current,
		Total:   snapshot.Total,
		Error:   snapshot.Error,
		Results: make([]SegmentProgress, 0, len(snapshot.Results)),
	}
	for _, result := range snapshot.Results {
		progress.Results = append(progress.Results, SegmentProgress{
			Segment:       result.Segment,
			Pinyin:        result.Pinyin,
			English:       result.English,
			Index:         result.Index,
			SentenceIndex: result.SentenceIndex,
		})
	}
	return progress, true
}

func (m *Manager) CleanupProgress(translationID string) {
	// Persisted progress should remain queryable after stream disconnect/restart.
	_ = translationID
}

func (m *Manager) runJob(ctx context.Context, translationID string, segments []queuedSegment, startIndex int) {
	defer m.removeRunning(translationID)

	for idx := startIndex; idx < len(segments); idx++ {
		work := segments[idx]
		translated, err := m.provider.TranslateSegments(ctx, []string{work.Segment}, work.SentenceText)
		if err != nil || len(translated) == 0 {
			_ = m.store.Fail(translationID, "Failed to translate segments")
			return
		}
		segmentResult := translated[0]
		if _, _, err := m.store.AddProgressSegment(translationID, segmentResult, work.SentenceIndex); err != nil {
			_ = m.store.Fail(translationID, "Failed to update translation progress")
			return
		}

		time.Sleep(15 * time.Millisecond)
	}

	if err := m.store.Complete(translationID); err != nil {
		_ = m.store.Fail(translationID, "Failed to complete translation")
		return
	}
}

func (m *Manager) segmentInputBySentence(ctx context.Context, sentences []string) ([]queuedSegment, error) {
	queued := make([]queuedSegment, 0, len(sentences)*4)
	for sentenceIdx, sentence := range sentences {
		segments, err := m.provider.Segment(ctx, sentence)
		if err != nil {
			return nil, err
		}
		for _, seg := range segments {
			seg = strings.TrimSpace(seg)
			if seg == "" {
				continue
			}
			queued = append(queued, queuedSegment{
				SentenceIndex: sentenceIdx,
				SentenceText:  sentence,
				Segment:       seg,
			})
		}
	}
	return queued, nil
}

func splitInputSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var out []string
	var b strings.Builder
	for len(text) > 0 {
		r, size := utf8.DecodeRuneInString(text)
		text = text[size:]
		if r == '\n' || r == '\r' {
			if s := strings.TrimSpace(b.String()); s != "" {
				out = append(out, s)
			}
			b.Reset()
			continue
		}
		b.WriteRune(r)
		if isSentenceDelimiter(r) {
			if s := strings.TrimSpace(b.String()); s != "" {
				out = append(out, s)
			}
			b.Reset()
		}
	}
	if s := strings.TrimSpace(b.String()); s != "" {
		out = append(out, s)
	}
	return out
}

func isSentenceDelimiter(r rune) bool {
	switch r {
	case '。', '！', '？', '!', '?', ';', '；':
		return true
	default:
		return false
	}
}

func (m *Manager) removeRunning(translationID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.running, translationID)
}
