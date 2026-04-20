import { patchJson, postJson, postJsonForm } from '../../lib/api';
import type { ExtractTextResponse, TranslateSentenceSegmentsResponse } from './types';

export async function updateTranslationSource(
  id: string,
  inputText: string
): Promise<{ status: string; sentences_changed: number }> {
  return patchJson(`/api/translations/${id}`, { input_text: inputText });
}

export async function updateTranslationTitle(id: string, title: string): Promise<void> {
  await patchJson(`/api/translations/${id}`, { title });
}

export async function translateSentenceSegments(
  segments: string[],
  fullText: string | null,
  translationId: string | null,
  sentenceIdx: number | null
): Promise<TranslateSentenceSegmentsResponse> {
  return postJson('/api/translations/sentence-segments/translate', {
    segments,
    full_text: fullText,
    translation_id: translationId,
    sentence_idx: sentenceIdx,
  });
}

export async function extractText(formData: FormData): Promise<ExtractTextResponse> {
	return postJsonForm('/api/ocr/extract-text', formData);
}
