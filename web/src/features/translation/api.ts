import { patchJson, postJson } from '../../lib/api';
import type { TranslateBatchResponse } from './types';

export async function updateTranslationSource(
  id: string,
  inputText: string
): Promise<{ status: string; sentences_changed: number }> {
  return patchJson(`/api/translations/${id}`, { input_text: inputText });
}

export async function translateBatch(
  segments: string[],
  context: string | null,
  translationId: string | null,
  sentenceIdx: number | null
): Promise<TranslateBatchResponse> {
  return postJson('/api/segments/translate-batch', {
    segments,
    context,
    translation_id: translationId,
    sentence_idx: sentenceIdx,
  });
}
