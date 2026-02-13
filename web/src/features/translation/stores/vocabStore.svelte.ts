// Vocabulary store - manages saved vocabulary and SRS state

import type {
  DueCountResponse,
  RecordLookupResponse,
  SavedVocabInfo,
  SaveVocabResponse,
  VocabSrsInfoListResponse,
} from '@/features/translation/types';
import { getJson, postJson } from '@/lib/api';

class VocabStore {
  savedVocabMap = $state<Map<string, SavedVocabInfo>>(new Map());
  dueCount = $state(0);

  async fetchSrsInfo(headwords: string[]): Promise<void> {
    if (headwords.length === 0) return;

    try {
      const params = new URLSearchParams();
      params.set('headwords', headwords.join(','));
      const data = await getJson<VocabSrsInfoListResponse>(
        `/api/vocab/srs-info?${params.toString()}`
      );

      const nextMap = new Map(this.savedVocabMap);
      data.items.forEach((info) => {
        const opacity = info.status === 'known' ? 0 : info.opacity;
        nextMap.set(info.headword, {
          vocabItemId: info.vocab_item_id,
          opacity,
          isStruggling: info.is_struggling,
          status: info.status,
        });
      });
      this.savedVocabMap = nextMap;
    } catch (error) {
      console.error('Failed to fetch SRS info:', error);
    }
  }

  /**
   * Save a new vocabulary item
   */
  async saveVocab(
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

      this.savedVocabMap = new Map(this.savedVocabMap.set(headword, info));
      return info;
    } catch (error) {
      console.error('Failed to save vocab:', error);
      return null;
    }
  }

  /**
   * Mark a vocabulary item as known
   */
  async markKnown(headword: string, vocabItemId: string): Promise<void> {
    try {
      await postJson('/api/vocab/status', {
        vocab_item_id: vocabItemId,
        status: 'known',
      });

      const info = this.savedVocabMap.get(headword);
      if (info) {
        this.savedVocabMap = new Map(
          this.savedVocabMap.set(headword, { ...info, status: 'known', opacity: 0 })
        );
      }
      // Refresh due count after status change
      await this.loadDueCount();
    } catch (error) {
      console.error('Failed to mark known:', error);
    }
  }

  /**
   * Resume learning a vocabulary item
   */
  async resumeLearning(headword: string, vocabItemId: string): Promise<void> {
    try {
      await postJson('/api/vocab/status', {
        vocab_item_id: vocabItemId,
        status: 'learning',
      });

      const info = this.savedVocabMap.get(headword);
      if (info) {
        this.savedVocabMap = new Map(
          this.savedVocabMap.set(headword, { ...info, status: 'learning', opacity: 1 })
        );
      }
      // Refresh due count after status change
      await this.loadDueCount();
    } catch (error) {
      console.error('Failed to resume learning:', error);
    }
  }

  /**
   * Record a lookup of a vocabulary item
   */
  async recordLookup(headword: string, vocabItemId: string): Promise<void> {
    try {
      const data = await postJson<RecordLookupResponse>('/api/vocab/lookup', {
        vocab_item_id: vocabItemId,
      });

      const info = this.savedVocabMap.get(headword);
      if (info) {
        this.savedVocabMap = new Map(
          this.savedVocabMap.set(headword, {
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
  async loadDueCount(): Promise<void> {
    try {
      const data = await getJson<DueCountResponse>('/api/review/count');
      this.dueCount = data.due_count;
    } catch {
      this.dueCount = 0;
    }
  }
}

export const vocabStore = new VocabStore();
