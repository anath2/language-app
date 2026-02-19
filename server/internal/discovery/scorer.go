package discovery

import (
	"context"

	"github.com/anath2/language-app/internal/intelligence"
)

const sampleCharLimit = 500

func ScoreArticle(ctx context.Context, page FetchedPage, provider intelligence.TranslationProvider, knownVocab map[string]string) (ScoredArticle, error) {
	sample := page.Body
	runes := []rune(sample)
	if len(runes) > sampleCharLimit {
		sample = string(runes[:sampleCharLimit])
	}

	segments, err := provider.Segment(ctx, sample)
	if err != nil {
		return ScoredArticle{}, err
	}

	unique := make(map[string]bool)
	var unknown, learning, known int
	for _, seg := range segments {
		if unique[seg] {
			continue
		}
		unique[seg] = true
		switch knownVocab[seg] {
		case "known":
			known++
		case "learning":
			learning++
		default:
			unknown++
		}
	}

	total := len(unique)
	var difficulty float64
	if total > 0 {
		difficulty = (float64(unknown) + 0.5*float64(learning)) / float64(total)
	}

	return ScoredArticle{
		FetchedPage:     page,
		DifficultyScore: difficulty,
		TotalWords:      total,
		UnknownWords:    unknown,
		LearningWords:   learning,
		KnownWords:      known,
	}, nil
}
