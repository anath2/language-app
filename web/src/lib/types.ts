export interface UserProfile {
  name: string;
  email: string;
  language: string;
  created_at: string;
  updated_at: string;
}

export interface AdminProfileResponse {
  profile: UserProfile | null;
  vocabStats: {
    known: number;
    learning: number;
    total: number;
  };
}

export interface ImportProgressResponse {
  success: boolean;
  counts?: {
    vocab_items: number;
    srs_state: number;
    vocab_lookups: number;
  };
}
