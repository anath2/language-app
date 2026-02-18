export interface DiscoveryPreference {
  id: string;
  topic: string;
  weight: number;
  created_at: string;
  updated_at: string;
}

export interface DiscoveryArticle {
  id: string;
  run_id: string;
  url: string;
  title: string;
  source_name: string;
  summary: string;
  difficulty_score: number;
  total_words: number;
  unknown_words: number;
  learning_words: number;
  known_words: number;
  status: 'new' | 'dismissed' | 'imported';
  translation_id: string | null;
  created_at: string;
  updated_at: string;
}

export interface ListArticlesResponse {
  articles: DiscoveryArticle[];
  total: number;
}

export interface ImportArticleResponse {
  translation_id: string;
  article_id: string;
}
