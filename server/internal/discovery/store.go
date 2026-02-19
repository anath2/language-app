package discovery

import (
	"database/sql"
	"fmt"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func newID() string {
	return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// Preferences

func (s *Store) SavePreference(topic string, weight float64) (Preference, error) {
	ts := now()
	id := newID()
	_, err := s.db.Exec(
		`INSERT INTO discovery_preferences (id, topic, weight, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(topic) DO UPDATE SET weight = excluded.weight, updated_at = excluded.updated_at`,
		id, topic, weight, ts, ts,
	)
	if err != nil {
		return Preference{}, fmt.Errorf("save preference: %w", err)
	}
	var p Preference
	err = s.db.QueryRow(
		`SELECT id, topic, weight, created_at, updated_at FROM discovery_preferences WHERE topic = ?`, topic,
	).Scan(&p.ID, &p.Topic, &p.Weight, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return Preference{}, fmt.Errorf("read saved preference: %w", err)
	}
	return p, nil
}

func (s *Store) ListPreferences() ([]Preference, error) {
	rows, err := s.db.Query(`SELECT id, topic, weight, created_at, updated_at FROM discovery_preferences ORDER BY weight DESC, created_at`)
	if err != nil {
		return nil, fmt.Errorf("list preferences: %w", err)
	}
	defer rows.Close()
	var out []Preference
	for rows.Next() {
		var p Preference
		if err := rows.Scan(&p.ID, &p.Topic, &p.Weight, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan preference: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) DeletePreference(id string) bool {
	res, err := s.db.Exec(`DELETE FROM discovery_preferences WHERE id = ?`, id)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

// Runs

func (s *Store) CreateRun(triggerType string) (Run, error) {
	id := newID()
	ts := now()
	_, err := s.db.Exec(
		`INSERT INTO discovery_runs (id, status, trigger_type, articles_found, started_at) VALUES (?, 'running', ?, 0, ?)`,
		id, triggerType, ts,
	)
	if err != nil {
		return Run{}, fmt.Errorf("create run: %w", err)
	}
	return Run{ID: id, Status: "running", TriggerType: triggerType, StartedAt: ts}, nil
}

func (s *Store) CompleteRun(id string, articlesFound int) error {
	ts := now()
	_, err := s.db.Exec(
		`UPDATE discovery_runs SET status = 'completed', articles_found = ?, completed_at = ? WHERE id = ?`,
		articlesFound, ts, id,
	)
	return err
}

func (s *Store) FailRun(id string, errMsg string) error {
	ts := now()
	_, err := s.db.Exec(
		`UPDATE discovery_runs SET status = 'failed', error_message = ?, completed_at = ? WHERE id = ?`,
		errMsg, ts, id,
	)
	return err
}

// Articles

func (s *Store) SaveArticle(runID string, scored ScoredArticle) (Article, error) {
	id := newID()
	ts := now()
	_, err := s.db.Exec(
		`INSERT INTO article_recommendations
		 (id, run_id, url, title, difficulty_score, total_words, unknown_words, learning_words, known_words, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'new', ?, ?)
		 ON CONFLICT(url) DO UPDATE SET
		   difficulty_score = excluded.difficulty_score,
		   total_words = excluded.total_words,
		   unknown_words = excluded.unknown_words,
		   learning_words = excluded.learning_words,
		   known_words = excluded.known_words,
		   updated_at = excluded.updated_at`,
		id, runID, scored.URL, scored.Title,
		scored.DifficultyScore, scored.TotalWords, scored.UnknownWords, scored.LearningWords, scored.KnownWords,
		ts, ts,
	)
	if err != nil {
		return Article{}, fmt.Errorf("save article: %w", err)
	}
	return s.getArticleByURL(scored.URL)
}

func (s *Store) getArticleByURL(url string) (Article, error) {
	var a Article
	err := s.db.QueryRow(
		`SELECT id, run_id, url, title, source_name, summary, difficulty_score,
		        total_words, unknown_words, learning_words, known_words,
		        status, translation_id, created_at, updated_at
		 FROM article_recommendations WHERE url = ?`, url,
	).Scan(&a.ID, &a.RunID, &a.URL, &a.Title, &a.SourceName, &a.Summary,
		&a.DifficultyScore, &a.TotalWords, &a.UnknownWords, &a.LearningWords, &a.KnownWords,
		&a.Status, &a.TranslationID, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return Article{}, fmt.Errorf("get article by url: %w", err)
	}
	return a, nil
}

func (s *Store) ListArticles(status string, limit, offset int) ([]Article, int, error) {
	var total int
	var countErr error
	if status != "" {
		countErr = s.db.QueryRow(`SELECT COUNT(*) FROM article_recommendations WHERE status = ?`, status).Scan(&total)
	} else {
		countErr = s.db.QueryRow(`SELECT COUNT(*) FROM article_recommendations`).Scan(&total)
	}
	if countErr != nil {
		return nil, 0, fmt.Errorf("count articles: %w", countErr)
	}

	query := `SELECT id, run_id, url, title, source_name, summary, difficulty_score,
	                 total_words, unknown_words, learning_words, known_words,
	                 status, translation_id, created_at, updated_at
	          FROM article_recommendations`
	var args []any
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list articles: %w", err)
	}
	defer rows.Close()

	var out []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(&a.ID, &a.RunID, &a.URL, &a.Title, &a.SourceName, &a.Summary,
			&a.DifficultyScore, &a.TotalWords, &a.UnknownWords, &a.LearningWords, &a.KnownWords,
			&a.Status, &a.TranslationID, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan article: %w", err)
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

func (s *Store) GetArticle(id string) (Article, bool) {
	var a Article
	err := s.db.QueryRow(
		`SELECT id, run_id, url, title, source_name, summary, difficulty_score,
		        total_words, unknown_words, learning_words, known_words,
		        status, translation_id, created_at, updated_at
		 FROM article_recommendations WHERE id = ?`, id,
	).Scan(&a.ID, &a.RunID, &a.URL, &a.Title, &a.SourceName, &a.Summary,
		&a.DifficultyScore, &a.TotalWords, &a.UnknownWords, &a.LearningWords, &a.KnownWords,
		&a.Status, &a.TranslationID, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return Article{}, false
	}
	return a, true
}

func (s *Store) DismissArticle(id string) bool {
	res, err := s.db.Exec(`UPDATE article_recommendations SET status = 'dismissed', updated_at = ? WHERE id = ? AND status = 'new'`, now(), id)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

func (s *Store) ImportArticle(id string, translationID string) bool {
	res, err := s.db.Exec(
		`UPDATE article_recommendations SET status = 'imported', translation_id = ?, updated_at = ? WHERE id = ?`,
		translationID, now(), id,
	)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

func (s *Store) GetKnownHeadwords() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT headword, status FROM vocab_items`)
	if err != nil {
		return nil, fmt.Errorf("get known headwords: %w", err)
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var hw, status string
		if err := rows.Scan(&hw, &status); err != nil {
			return nil, fmt.Errorf("scan headword: %w", err)
		}
		out[hw] = status
	}
	return out, rows.Err()
}

func (s *Store) ListRecentArticleURLs(limit int) ([]string, error) {
	rows, err := s.db.Query(`SELECT url FROM article_recommendations ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent urls: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, fmt.Errorf("scan url: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
