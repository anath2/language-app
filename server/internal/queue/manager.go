package queue

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/anath2/language-app/internal/intelligence"
	"github.com/anath2/language-app/internal/translation"
)

type SegmentProgress struct {
	Segment        string `json:"segment"`
	Pinyin         string `json:"pinyin"`
	English        string `json:"english"`
	Index          int    `json:"index"`
	ParagraphIndex int    `json:"paragraph_index"`
}

type Progress struct {
	Status  string            `json:"status"`
	Current int               `json:"current"`
	Total   int               `json:"total"`
	Results []SegmentProgress `json:"results"`
	Error   string            `json:"error,omitempty"`
}

type Manager struct {
	store    *translation.Store
	provider intelligence.Provider
	mu       sync.RWMutex
	running  map[string]struct{}
}

const jobLeaseDuration = 30 * time.Second

func NewManager(store *translation.Store, provider intelligence.Provider) *Manager {
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
		segments, err := m.provider.Segment(ctx, item.InputText)
		if err != nil {
			_ = m.store.Fail(translationID, "Failed to segment translation input")
			m.removeRunning(translationID)
			return
		}
		total := len(segments)
		if total == 0 {
			_ = m.store.Fail(translationID, "No translatable segments found")
			m.removeRunning(translationID)
			return
		}

		startIndex := item.Progress
		if item.Status == "pending" {
			startIndex = 0
			if err := m.store.SetProcessing(translationID, total); err != nil {
				m.removeRunning(translationID)
				return
			}
		}

		if startIndex >= len(segments) {
			if err := m.store.Complete(translationID); err != nil {
				_ = m.store.Fail(translationID, "Failed to complete translation")
			}
			m.removeRunning(translationID)
			return
		}

		m.runJob(ctx, translationID, item.InputText, segments, startIndex)
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
			Segment:        result.Segment,
			Pinyin:         result.Pinyin,
			English:        result.English,
			Index:          result.Index,
			ParagraphIndex: result.ParagraphIndex,
		})
	}
	return progress, true
}

func (m *Manager) CleanupProgress(translationID string) {
	// Persisted progress should remain queryable after stream disconnect/restart.
	_ = translationID
}

func (m *Manager) runJob(ctx context.Context, translationID string, sentenceContext string, segments []string, startIndex int) {
	defer m.removeRunning(translationID)

	for idx := startIndex; idx < len(segments); idx++ {
		segment := segments[idx]
		translated, err := m.provider.TranslateSegments(ctx, []string{segment}, sentenceContext)
		if err != nil || len(translated) == 0 {
			_ = m.store.Fail(translationID, "Failed to translate segments")
			return
		}
		segmentResult := translated[0]
		if _, _, err := m.store.AddProgressSegment(translationID, segmentResult); err != nil {
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

func (m *Manager) removeRunning(translationID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.running, translationID)
}
