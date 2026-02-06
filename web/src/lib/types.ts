export type JobStatus = "pending" | "processing" | "completed" | "failed";
export type LoadingState = "idle" | "loading" | "error";
export type VocabStatus = "unknown" | "learning" | "known";

export interface JobSummary {
  id: string;
  created_at: string;
  status: JobStatus;
  source_type: string;
  input_preview: string;
  full_translation_preview: string | null;
  segment_count: number | null;
  total_segments: number | null;
}

export interface ListJobsResponse {
  jobs: JobSummary[];
  total: number;
}

export interface TranslationResult {
  segment: string;
  pinyin: string;
  english: string;
}

export interface ParagraphResult {
  translations: TranslationResult[];
  indent: string;
  separator: string;
}

export interface JobDetailResponse {
  id: string;
  created_at: string;
  status: JobStatus;
  source_type: string;
  input_text: string;
  full_translation: string | null;
  error_message: string | null;
  paragraphs: ParagraphResult[] | null;
}

export interface CreateJobResponse {
  job_id: string;
  status: JobStatus;
}

export interface ProgressState {
  current: number;
  total: number;
}

export interface ParagraphMeta {
  segment_count: number;
  indent: string;
  separator: string;
}

export interface SegmentResult {
  segment: string;
  pinyin: string;
  english: string;
  index: number;
  paragraph_index: number;
  pending: boolean;
}

export interface DisplayParagraph extends ParagraphMeta {
  paragraph_index: number;
  segments: SegmentResult[];
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

export interface StreamSegmentResult {
  segment: string;
  pinyin: string;
  english: string;
  index: number;
  paragraph_index: number;
}

export type StreamStartEvent = {
  type: "start";
  job_id: string;
  total?: number;
  paragraphs?: ParagraphMeta[];
  fullTranslation?: string | null;
};

export type StreamProgressEvent = {
  type: "progress";
  current: number;
  total: number;
  result: StreamSegmentResult;
};

export type StreamCompleteEvent = {
  type: "complete";
  paragraphs?: ParagraphResult[];
  fullTranslation?: string | null;
};

export type StreamErrorEvent = {
  type: "error";
  message?: string;
};

export type StreamEvent = StreamStartEvent | StreamProgressEvent | StreamCompleteEvent | StreamErrorEvent;

export interface SavedVocabInfo {
  vocabItemId: string;
  opacity: number;
  isStruggling: boolean;
  status: VocabStatus;
}

export interface TooltipState {
  headword: string;
  pinyin: string;
  english: string;
  vocabItemId: string | null;
  status: VocabStatus | "";
  x: number;
  y: number;
}
