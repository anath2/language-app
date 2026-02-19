export type TranslationStatus = 'pending' | 'processing' | 'completed' | 'failed';
export type LoadingState = 'idle' | 'loading' | 'error';
export type VocabStatus = 'unknown' | 'learning' | 'known';

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
  translations: TranslationSummary[];
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
  paragraphs: ParagraphMeta[] | null;
}

export interface CreateTranslationResponse {
  translation_id: string;
  status: TranslationStatus;
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

export interface StreamSegmentResult {
  segment: string;
  pinyin: string;
  english: string;
  index: number;
  paragraph_index: number;
}

export type StreamStartEvent = {
  type: 'start';
  translation_id: string;
  total?: number;
  paragraphs?: ParagraphMeta[];
  fullTranslation?: string | null;
};

export type StreamProgressEvent = {
  type: 'progress';
  current: number;
  total: number;
  result: StreamSegmentResult;
};

export type StreamCompleteEvent = {
  type: 'complete';
  paragraphs?: ParagraphResult[];
  fullTranslation?: string | null;
};

export type StreamErrorEvent = {
  type: 'error';
  message?: string;
};

export type StreamEvent =
  | StreamStartEvent
  | StreamProgressEvent
  | StreamCompleteEvent
  | StreamErrorEvent;

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

export interface ReviewAnswerResponse {
  vocab_item_id: string;
  next_due_at: string | null;
  interval_days: number;
  remaining_due: number;
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

export interface DueCountResponse {
  due_count: number;
}

export interface CreateTextResponse {
  id: string;
}

export interface ExtractTextResponse {
  text: string;
}

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
  status: VocabStatus | '';
  x: number;
  y: number;
}

export interface TranslateBatchResponse {
  translations: TranslationResult[];
}

export interface CharacterExampleWord {
  vocab_item_id: string;
  headword: string;
  pinyin: string;
  english: string;
}

export interface CharacterReviewCard {
  vocab_item_id: string;
  character: string;
  pinyin: string;
  english?: string;
  example_words: CharacterExampleWord[];
}

export interface CharacterReviewQueueResponse {
  cards: CharacterReviewCard[];
  due_count: number;
}

export interface VocabStatsResponse {
  vocabStats: {
    known: number;
    learning: number;
    total: number;
  };
}

// Translation chat API types
export interface ChatCreateRequest {
  message: string;
  selected_segment_ids?: string[];
}

export interface ChatReviewCard {
  chinese_text: string;
  pinyin: string;
  english: string;
  status: 'pending' | 'accepted';
}

export interface ChatMessage {
  id: string;
  chat_id: string;
  translation_id: string;
  message_idx: number;
  role: 'user' | 'ai' | 'tool';
  content: string;
  selected_segment_ids: string[];
  created_at: string;
  review_card?: ChatReviewCard;
}

export interface ChatListResponse {
  chat_id: string;
  messages: ChatMessage[];
}

export type ChatStreamStartEvent = {
  type: 'start';
  translation_id?: string;
  chat_id?: string;
  user_message_id?: string;
};

export type ChatStreamChunkEvent = {
  type: 'chunk';
  delta?: string;
};

export type ChatToolResult = {
  message_id: string;
  review_card: ChatReviewCard;
};

export type ChatStreamCompleteEvent = {
  type: 'complete';
  message_id?: string;
  content?: string;
  tool_results?: ChatToolResult[];
};

export type ChatStreamToolCallStartEvent = {
  type: 'tool_call_start';
  tool_name?: string;
};

export type ChatStreamErrorEvent = {
  type: 'error';
  message?: string;
};

export type ChatStreamEvent =
  | ChatStreamStartEvent
  | ChatStreamChunkEvent
  | ChatStreamCompleteEvent
  | ChatStreamToolCallStartEvent
  | ChatStreamErrorEvent;
