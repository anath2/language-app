export type { LoadingState, VocabStatus } from "../../lib/types";

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
  status: import("../../lib/types").VocabStatus;
}

export interface TooltipState {
  headword: string;
  pinyin: string;
  english: string;
  vocabItemId: string | null;
  status: import("../../lib/types").VocabStatus | "";
  x: number;
  y: number;
}

export interface TranslateBatchResponse {
  translations: TranslationResult[];
}
