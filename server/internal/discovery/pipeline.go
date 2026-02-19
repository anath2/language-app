package discovery

import (
	"context"
	"log"

	"github.com/anath2/language-app/internal/intelligence"
)

var defaultTopics = []string{"technology", "culture", "news"}

type Pipeline struct {
	store    *Store
	provider intelligence.TranslationProvider
}

func NewPipeline(store *Store, provider intelligence.TranslationProvider) *Pipeline {
	return &Pipeline{store: store, provider: provider}
}

func (p *Pipeline) Run(ctx context.Context, trigger string) error {
	run, err := p.store.CreateRun(trigger)
	if err != nil {
		return err
	}

	articlesFound, runErr := p.execute(ctx, run.ID)
	if runErr != nil {
		log.Printf("discovery run failed: id=%s err=%v", run.ID, runErr)
		_ = p.store.FailRun(run.ID, runErr.Error())
		return runErr
	}

	return p.store.CompleteRun(run.ID, articlesFound)
}

func (p *Pipeline) execute(ctx context.Context, runID string) (int, error) {
	topics, err := p.loadTopics()
	if err != nil {
		return 0, err
	}

	existingURLs, err := p.store.ListRecentArticleURLs(200)
	if err != nil {
		return 0, err
	}

	knownVocab, err := p.store.GetKnownHeadwords()
	if err != nil {
		return 0, err
	}

	// Try RSS feeds first (real articles, publicly accessible worldwide), fall back to LLM.
	rssPages, err := fetchRSSPages(ctx, existingURLs)
	if err != nil || len(rssPages) == 0 {
		log.Printf("discovery rss unavailable (err=%v), falling back to LLM", err)
		candidateURLs, err := p.provider.SuggestArticleURLs(ctx, topics, existingURLs)
		if err != nil {
			return 0, err
		}
		log.Printf("discovery sourced %d URLs (LLM) for topics=%v", len(candidateURLs), topics)
		return p.processURLs(ctx, runID, candidateURLs, knownVocab)
	}

	log.Printf("discovery sourced %d pages (RSS)", len(rssPages))
	return p.processPages(ctx, runID, rssPages, knownVocab)
}

// processPages scores and saves pre-fetched pages (e.g. from Juejin API) without
// making additional HTTP requests. The page Body must already contain CJK text.
func (p *Pipeline) processPages(ctx context.Context, runID string, pages []FetchedPage, knownVocab map[string]string) (int, error) {
	var saved int
	for _, page := range pages {
		if !HasCJKContent(page.Body) {
			log.Printf("discovery skip non-CJK: url=%s", page.URL)
			continue
		}
		scored, err := ScoreArticle(ctx, page, p.provider, knownVocab)
		if err != nil {
			log.Printf("discovery score failed: url=%s err=%v", page.URL, err)
			continue
		}
		if _, err := p.store.SaveArticle(runID, scored); err != nil {
			log.Printf("discovery save failed: url=%s err=%v", page.URL, err)
			continue
		}
		saved++
	}
	return saved, nil
}

// processURLs fetches HTML for each URL then scores and saves the result.
// Used for LLM-suggested URLs where the page body is not yet available.
func (p *Pipeline) processURLs(ctx context.Context, runID string, urls []string, knownVocab map[string]string) (int, error) {
	var saved int
	for _, url := range urls {
		page, err := FetchPage(ctx, url)
		if err != nil {
			log.Printf("discovery fetch failed: url=%s err=%v", url, err)
			continue
		}
		if !HasCJKContent(page.Body) {
			log.Printf("discovery skip non-CJK: url=%s", url)
			continue
		}
		scored, err := ScoreArticle(ctx, page, p.provider, knownVocab)
		if err != nil {
			log.Printf("discovery score failed: url=%s err=%v", url, err)
			continue
		}
		if _, err := p.store.SaveArticle(runID, scored); err != nil {
			log.Printf("discovery save failed: url=%s err=%v", url, err)
			continue
		}
		saved++
	}
	return saved, nil
}

func (p *Pipeline) loadTopics() ([]string, error) {
	prefs, err := p.store.ListPreferences()
	if err != nil {
		return nil, err
	}
	if len(prefs) == 0 {
		return defaultTopics, nil
	}
	topics := make([]string, len(prefs))
	for i, pref := range prefs {
		topics[i] = pref.Topic
	}
	return topics, nil
}
