// Vocabulary store - manages saved vocabulary and SRS state

import type {
  DueCountResponse,
  RecordLookupResponse,
  SavedVocabInfo,
  SaveVocabResponse,
  VocabSrsInfoListResponse,
} from '@/features/translation/types';
import { getJson, postJson } from '@/lib/api';

// State
let savedVocabMap = $state<Map<string, SavedVocabInfo>>(new Map());
let dueCount = $state(0);

export async function fetchSrsInfo(headwords: string[]): Promise<void> {
  if (headwords.length === 0) return;

  try {
    const params = new URLSearchParams();
    params.set('headwords', headwords.join(','));
    const data = await getJson<VocabSrsInfoListResponse>(
      `/api/vocab/srs-info?${params.toString()}`
    );

    const nextMap = new Map(savedVocabMap);
    data.items.forEach((info) => {
      const opacity = info.status === 'known' ? 0 : info.opacity;
      nextMap.set(info.headword, {
        vocabItemId: info.vocab_item_id,
        opacity,
        isStruggling: info.is_struggling,
        status: info.status,
      });
    });
    savedVocabMap = nextMap;
  } catch (error) {
    console.error('Failed to fetch SRS info:', error);
  }
}

/**
 * Save a new vocabulary item
 */
export async function saveVocab(
  headword: string,
  pinyin: string,
  english: string,
  textId: string | null,
  snippet: string
): Promise<SavedVocabInfo | null> {
  try {
    const data = await postJson<SaveVocabResponse>('/api/vocab/save', {
      headword,
      pinyin,
      english,
      text_id: textId,
      snippet,
      status: 'learning',
    });

    const info: SavedVocabInfo = {
      vocabItemId: data.vocab_item_id,
      opacity: 1,
      isStruggling: false,
      status: 'learning',
    };

    savedVocabMap = new Map(savedVocabMap.set(headword, info));
    return info;
  } catch (error) {
    console.error('Failed to save vocab:', error);
    return null;
  }
}

/**
 * Mark a vocabulary item as known
 */
export async function markKnown(headword: string, vocabItemId: string): Promise<void> {
  try {
    await postJson('/api/vocab/status', {
      vocab_item_id: vocabItemId,
      status: 'known',
    });

    const info = savedVocabMap.get(headword);
    if (info) {
      savedVocabMap = new Map(
        savedVocabMap.set(headword, { ...info, status: 'known', opacity: 0 })
      );
    }
    // Refresh due count after status change
    await loadDueCount();
  } catch (error) {
    console.error('Failed to mark known:', error);
  }
}

/**
 * Resume learning a vocabulary item
 */
export async function resumeLearning(headword: string, vocabItemId: string): Promise<void> {
  try {
    await postJson('/api/vocab/status', {
      vocab_item_id: vocabItemId,
      status: 'learning',
    });

    const info = savedVocabMap.get(headword);
    if (info) {
      savedVocabMap = new Map(
        savedVocabMap.set(headword, { ...info, status: 'learning', opacity: 1 })
      );
    }
    // Refresh due count after status change
    await loadDueCount();
  } catch (error) {
    console.error('Failed to resume learning:', error);
  }
}

/**
 * Record a lookup of a vocabulary item
 */
export async function recordLookup(headword: string, vocabItemId: string): Promise<void> {
  try {
    const data = await postJson<RecordLookupResponse>('/api/vocab/lookup', {
      vocab_item_id: vocabItemId,
    });

    const info = savedVocabMap.get(headword);
    if (info) {
      savedVocabMap = new Map(
        savedVocabMap.set(headword, {
          ...info,
          opacity: data.opacity,
          isStruggling: data.is_struggling,
        })
      );
    }
  } catch (error) {
    console.error('Failed to record lookup:', error);
  }
}

/**
 * Load the count of due cards for review
 */
export async function loadDueCount(): Promise<void> {
  try {
    const data = await getJson<DueCountResponse>('/api/review/count');
    dueCount = data.due_count;
  } catch {
    dueCount = 0;
  }
}

// Export reactive state
export const vocabStore = {
  get savedVocabMap() {
    return savedVocabMap;
  },
  get dueCount() {
    return dueCount;
  },
  fetchSrsInfo,
  saveVocab,
  markKnown,
  resumeLearning,
  recordLookup,
  loadDueCount,
};
