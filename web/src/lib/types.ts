export type TranslationStatus = "pending" | "processing" | "completed" | "failed";
export type LoadingState = "idle" | "loading" | "error";
export type VocabStatus = "unknown" | "learning" | "known";

export interface TranslationSummary {
  id: string;
  created_at: string;
  status: TranslationStatus;
  source_type: string;
  input_preview: string;
  full_translation_preview: string | null;
  segment_count: number | null;
  total_segments: number | null;
}

export interface ListTranslationsResponse {
  jobs: TranslationSummary[];
  total: number;
}

export interface TranslationDetailResponse {
  id: string;
  created_at: string;
  status: TranslationStatus;
  source_type: string;
  input_text: string;
  full_translation: string | null;
  error_message: string | null;
  paragraphs: import("../features/segments/types").ParagraphResult[] | null;
}

export interface CreateTranslationResponse {
  job_id: string;
  status: TranslationStatus;
}

export interface ReviewCard {
  vocab_item_id: string;
  headword: string;
  pinyin: string;
  english: string;
  snippets: string[];
}

export interface ReviewQueueResponse {
  cards: ReviewCard[];
  due_count: number;
}

export interface DueCountResponse {
  due_count: number;
}

export interface VocabSrsInfoItem {
  vocab_item_id: string;
  headword: string;
  pinyin: string;
  english: string;
  opacity: number;
  is_struggling: boolean;
  status: VocabStatus;
}

export interface VocabSrsInfoListResponse {
  items: VocabSrsInfoItem[];
}

export interface RecordLookupResponse {
  vocab_item_id: string;
  opacity: number;
  is_struggling: boolean;
}

export interface SaveVocabResponse {
  vocab_item_id: string;
}

export interface CreateTextResponse {
  id: string;
}

export interface ReviewAnswerResponse {
  vocab_item_id: string;
  next_due_at: string | null;
  interval_days: number;
  remaining_due: number;
}

export interface ExtractTextResponse {
  text: string;
}
