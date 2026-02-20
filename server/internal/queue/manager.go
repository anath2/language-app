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
	provider intelligence.TranslationProvider
	mu       sync.RWMutex
	running  map[string]struct{}
}

type translationStore interface {
	ListRestartableTranslationIDs() ([]string, error)
	Get(id string) (translation.Translation, bool)
	ClaimTranslationJob(translationID string, leaseDuration time.Duration) (bool, error)
	Fail(id string, message string) error
	SetFullTranslation(id string, fullTranslation string) error
	SetProcessing(id string, total int, sentences []translation.SentenceInit) error
	SetReprocessing(id string, total int) error
	Complete(id string) error
	GetProgressSnapshot(id string) (translation.ProgressSnapshot, bool)
	AddProgressSegment(id string, result translation.SegmentResult, sentenceIndex int) (int, int, error)
	AddReprocessedSegment(id string, result translation.SegmentResult, sentenceIdx int, segIdx int) error
}

type queuedSegment struct {
	SentenceIndex int
	SentenceText  string
	Segment       string
}

type sentenceInfo struct {
	Text      string
	Indent    string
	Separator string
}

const jobLeaseDuration = 30 * time.Second

func NewManager(store translationStore, provider intelligence.TranslationProvider) *Manager {
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

		// Generate full translation before segmentation (non-fatal).
		if fullTranslation, err := m.provider.TranslateFull(ctx, item.InputText); err != nil {
			log.Printf("full translation failed (non-fatal): id=%s err=%v", translationID, err)
		} else if fullTranslation != "" {
			if err := m.store.SetFullTranslation(translationID, fullTranslation); err != nil {
				log.Printf("set full translation failed (non-fatal): id=%s err=%v", translationID, err)
			}
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
			sentenceInits := make([]translation.SentenceInit, len(sentences))
			for i, s := range sentences {
				sentenceInits[i] = translation.SentenceInit{Indent: s.Indent, Separator: s.Separator}
			}
			if err := m.store.SetProcessing(translationID, total, sentenceInits); err != nil {
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

// StartReprocessing processes only the sentences in sentencesToProcess (sentenceIdx → sentence text).
// It does not call TranslateFull — the existing full translation is preserved.
func (m *Manager) StartReprocessing(translationID string, sentencesToProcess map[int]string) {
	if len(sentencesToProcess) == 0 {
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

	go func() {
		ctx := context.Background()

		// Load the full input text for generating the full translation.
		item, ok := m.store.Get(translationID)
		if !ok {
			_ = m.store.Fail(translationID, "Translation not found during reprocessing")
			m.removeRunning(translationID)
			return
		}

		// Generate new full translation (non-fatal).
		if fullTranslation, err := m.provider.TranslateFull(ctx, item.InputText); err != nil {
			log.Printf("full translation failed (non-fatal): id=%s err=%v", translationID, err)
		} else if fullTranslation != "" {
			if err := m.store.SetFullTranslation(translationID, fullTranslation); err != nil {
				log.Printf("set full translation failed (non-fatal): id=%s err=%v", translationID, err)
			}
		}

		// Pre-segment all changed sentences to get the total count.
		type reprocessWork struct {
			sentenceIdx  int
			sentenceText string
			segment      string
		}
		var allWork []reprocessWork
		// Process in stable order.
		orderedIdxs := make([]int, 0, len(sentencesToProcess))
		for idx := range sentencesToProcess {
			orderedIdxs = append(orderedIdxs, idx)
		}
		// Sort indices for deterministic ordering.
		for i := 0; i < len(orderedIdxs); i++ {
			for j := i + 1; j < len(orderedIdxs); j++ {
				if orderedIdxs[i] > orderedIdxs[j] {
					orderedIdxs[i], orderedIdxs[j] = orderedIdxs[j], orderedIdxs[i]
				}
			}
		}

		for _, sentenceIdx := range orderedIdxs {
			sentence := sentencesToProcess[sentenceIdx]
			segments, err := m.provider.Segment(ctx, sentence)
			if err != nil {
				_ = m.store.Fail(translationID, "Failed to segment during reprocessing: "+err.Error())
				m.removeRunning(translationID)
				return
			}
			for _, seg := range segments {
				seg = strings.TrimSpace(seg)
				if seg == "" {
					continue
				}
				allWork = append(allWork, reprocessWork{
					sentenceIdx:  sentenceIdx,
					sentenceText: sentence,
					segment:      seg,
				})
			}
		}

		if err := m.store.SetReprocessing(translationID, len(allWork)); err != nil {
			m.removeRunning(translationID)
			return
		}

		// Translate each segment and store with explicit (sentenceIdx, localSegIdx).
		localSegIdx := make(map[int]int) // per-sentence counter
		for _, work := range allWork {
			translated, err := m.provider.TranslateSegments(ctx, []string{work.segment}, work.sentenceText)
			if err != nil || len(translated) == 0 {
				_ = m.store.Fail(translationID, "Failed to translate segment during reprocessing")
				m.removeRunning(translationID)
				return
			}
			segIdx := localSegIdx[work.sentenceIdx]
			if err := m.store.AddReprocessedSegment(translationID, translated[0], work.sentenceIdx, segIdx); err != nil {
				_ = m.store.Fail(translationID, "Failed to store reprocessed segment")
				m.removeRunning(translationID)
				return
			}
			localSegIdx[work.sentenceIdx]++
			time.Sleep(15 * time.Millisecond)
		}

		if err := m.store.Complete(translationID); err != nil {
			_ = m.store.Fail(translationID, "Failed to complete reprocessed translation")
		}
		m.removeRunning(translationID)
	}()
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

func (m *Manager) segmentInputBySentence(ctx context.Context, sentences []sentenceInfo) ([]queuedSegment, error) {
	queued := make([]queuedSegment, 0, len(sentences)*4)
	for sentenceIdx, sent := range sentences {
		segments, err := m.provider.Segment(ctx, sent.Text)
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
				SentenceText:  sent.Text,
				Segment:       seg,
			})
		}
	}
	return queued, nil
}

func splitInputSentences(text string) []sentenceInfo {
	var out []sentenceInfo
	var sentence strings.Builder
	var lineIndent strings.Builder
	atLineStart := true

	addSeparatorChar := func(r rune) {
		if len(out) > 0 {
			out[len(out)-1].Separator += string(r)
		}
	}

	for len(text) > 0 {
		r, size := utf8.DecodeRuneInString(text)
		text = text[size:]

		if atLineStart {
			if r == ' ' || r == '\t' {
				lineIndent.WriteRune(r)
				continue
			}
			if r == '\n' || r == '\r' {
				addSeparatorChar(r)
				lineIndent.Reset()
				// atLineStart stays true
				continue
			}
			atLineStart = false
		}

		if r == '\n' || r == '\r' {
			s := strings.TrimSpace(sentence.String())
			if s != "" {
				out = append(out, sentenceInfo{
					Text:   s,
					Indent: lineIndent.String(),
				})
			}
			addSeparatorChar(r)
			sentence.Reset()
			lineIndent.Reset()
			atLineStart = true
			continue
		}

		sentence.WriteRune(r)
		if isSentenceDelimiter(r) {
			s := strings.TrimSpace(sentence.String())
			if s != "" {
				out = append(out, sentenceInfo{
					Text:   s,
					Indent: lineIndent.String(),
				})
				sentence.Reset()
				lineIndent.Reset()
			}
		}
	}

	if s := strings.TrimSpace(sentence.String()); s != "" {
		out = append(out, sentenceInfo{
			Text:   s,
			Indent: lineIndent.String(),
		})
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
