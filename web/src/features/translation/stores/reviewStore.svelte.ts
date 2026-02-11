// Review store - manages review queue state

import { getJson, postJson } from '@/lib/api';
import type {
  ReviewAnswerResponse,
  ReviewCard,
  ReviewQueueResponse,
} from '@/features/translation/types';

// State
let queue = $state<ReviewCard[]>([]);
let currentIndex = $state(0);
let isAnswerRevealed = $state(false);
let isLoading = $state(false);

/**
 * Load the review queue from the server
 */
export async function loadQueue(limit: number = 20): Promise<void> {
  isLoading = true;
  try {
    const data = await getJson<ReviewQueueResponse>(`/api/review/queue?limit=${limit}`);
    queue = data.cards || [];
    currentIndex = 0;
    isAnswerRevealed = false;
  } catch (error) {
    console.error('Failed to load review queue:', error);
    queue = [];
  } finally {
    isLoading = false;
  }
}

/**
 * Reveal the answer for the current card
 */
export function revealAnswer(): void {
  isAnswerRevealed = true;
}

/**
 * Grade the current card and move to the next
 */
export async function gradeCard(grade: number): Promise<void> {
  if (!queue[currentIndex]) return;

  try {
    await postJson<ReviewAnswerResponse>('/api/review/answer', {
      vocab_item_id: queue[currentIndex].vocab_item_id,
      grade,
    });
  } catch (error) {
    console.error('Failed to record grade:', error);
  }

  currentIndex += 1;
  isAnswerRevealed = false;
}

// Export reactive state
export const reviewStore = {
  get queue() {
    return queue;
  },
  get currentIndex() {
    return currentIndex;
  },
  get isAnswerRevealed() {
    return isAnswerRevealed;
  },
  get isLoading() {
    return isLoading;
  },
  get currentCard(): ReviewCard | null {
    return queue[currentIndex] || null;
  },
  get progress(): { current: number; total: number } {
    return { current: currentIndex + 1, total: queue.length };
  },
  get isQueueExhausted(): boolean {
    return queue.length > 0 && currentIndex >= queue.length;
  },
  loadQueue,
  revealAnswer,
  gradeCard,
};
