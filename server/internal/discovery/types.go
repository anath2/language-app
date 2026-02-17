package discovery

type Preference struct {
	ID        string  `json:"id"`
	Topic     string  `json:"topic"`
	Weight    float64 `json:"weight"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type Run struct {
	ID            string  `json:"id"`
	Status        string  `json:"status"`
	TriggerType   string  `json:"trigger_type"`
	ArticlesFound int     `json:"articles_found"`
	ErrorMessage  *string `json:"error_message"`
	StartedAt     string  `json:"started_at"`
	CompletedAt   *string `json:"completed_at"`
}

type Article struct {
	ID              string  `json:"id"`
	RunID           string  `json:"run_id"`
	URL             string  `json:"url"`
	Title           string  `json:"title"`
	SourceName      string  `json:"source_name"`
	Summary         string  `json:"summary"`
	DifficultyScore float64 `json:"difficulty_score"`
	TotalWords      int     `json:"total_words"`
	UnknownWords    int     `json:"unknown_words"`
	LearningWords   int     `json:"learning_words"`
	KnownWords      int     `json:"known_words"`
	Status          string  `json:"status"`
	TranslationID   *string `json:"translation_id"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type FetchedPage struct {
	URL   string
	Title string
	Body  string
}

type ScoredArticle struct {
	FetchedPage
	DifficultyScore float64
	TotalWords      int
	UnknownWords    int
	LearningWords   int
	KnownWords      int
}
