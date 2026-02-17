// Character review store - manages character-level SRS review queue

import { getJson, postJson } from '@/lib/api';
import type {
  CharacterReviewCard,
  CharacterReviewQueueResponse,
  ReviewAnswerResponse,
} from '@/features/translation/types';

class CharacterReviewStore {
  queue = $state<CharacterReviewCard[]>([]);
  currentIndex = $state(0);
  isAnswerRevealed = $state(false);
  isLoading = $state(false);

  get currentCard(): CharacterReviewCard | null {
    return this.queue[this.currentIndex] || null;
  }

  get progress(): { current: number; total: number } {
    return { current: this.currentIndex + 1, total: this.queue.length };
  }

  get isQueueExhausted(): boolean {
    return this.queue.length > 0 && this.currentIndex >= this.queue.length;
  }

  async loadQueue(limit: number = 20): Promise<void> {
    this.isLoading = true;
    try {
      const data = await getJson<CharacterReviewQueueResponse>(
        `/api/review/characters/queue?limit=${limit}`
      );
      this.queue = data.cards || [];
      this.currentIndex = 0;
      this.isAnswerRevealed = false;
    } catch (error) {
      console.error('Failed to load character review queue:', error);
      this.queue = [];
    } finally {
      this.isLoading = false;
    }
  }

  revealAnswer(): void {
    this.isAnswerRevealed = true;
  }

  async gradeCard(grade: number): Promise<void> {
    if (!this.queue[this.currentIndex]) return;

    try {
      await postJson<ReviewAnswerResponse>('/api/review/answer', {
        vocab_item_id: this.queue[this.currentIndex].vocab_item_id,
        grade,
      });
    } catch (error) {
      console.error('Failed to record grade:', error);
    }

    this.currentIndex += 1;
    this.isAnswerRevealed = false;
  }
}

export const characterReviewStore = new CharacterReviewStore();
