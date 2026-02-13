// Review store - manages review queue state

import { getJson, postJson } from '@/lib/api';
import type {
  ReviewAnswerResponse,
  ReviewCard,
  ReviewQueueResponse,
} from '@/features/translation/types';

class ReviewStore {
  queue = $state<ReviewCard[]>([]);
  currentIndex = $state(0);
  isAnswerRevealed = $state(false);
  isLoading = $state(false);

  get currentCard(): ReviewCard | null {
    return this.queue[this.currentIndex] || null;
  }

  get progress(): { current: number; total: number } {
    return { current: this.currentIndex + 1, total: this.queue.length };
  }

  get isQueueExhausted(): boolean {
    return this.queue.length > 0 && this.currentIndex >= this.queue.length;
  }

  /**
   * Load the review queue from the server
   */
  async loadQueue(limit: number = 20): Promise<void> {
    this.isLoading = true;
    try {
      const data = await getJson<ReviewQueueResponse>(`/api/review/queue?limit=${limit}`);
      this.queue = data.cards || [];
      this.currentIndex = 0;
      this.isAnswerRevealed = false;
    } catch (error) {
      console.error('Failed to load review queue:', error);
      this.queue = [];
    } finally {
      this.isLoading = false;
    }
  }

  /**
   * Reveal the answer for the current card
   */
  revealAnswer(): void {
    this.isAnswerRevealed = true;
  }

  /**
   * Grade the current card and move to the next
   */
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

export const reviewStore = new ReviewStore();
