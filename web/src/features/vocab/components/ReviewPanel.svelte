<script lang="ts">
import { getJson, postJson } from '@/lib/api';
import type { ReviewAnswerResponse, ReviewCard, ReviewQueueResponse } from '@/lib/types';

const {
  open,
  onClose,
  onDueCountChange,
}: {
  open: boolean;
  onClose: () => void;
  onDueCountChange: (count: number) => void;
} = $props();

let reviewQueue = $state<ReviewCard[]>([]);
let reviewIndex = $state(0);
let reviewAnswered = $state(false);

$effect(() => {
  if (open) {
    void loadReviewQueue();
  }
});

async function loadReviewQueue() {
  try {
    const data = await getJson<ReviewQueueResponse>('/api/review/queue?limit=20');
    reviewQueue = data.cards || [];
    reviewIndex = 0;
    reviewAnswered = false;
    onDueCountChange(data.due_count || 0);
  } catch (error) {
    console.error('Failed to load review queue:', error);
    reviewQueue = [];
  }
}

function revealAnswer() {
  reviewAnswered = true;
}

async function gradeCard(grade: number) {
  if (!reviewQueue[reviewIndex]) return;

  try {
    await postJson<ReviewAnswerResponse>('/api/review/answer', {
      vocab_item_id: reviewQueue[reviewIndex].vocab_item_id,
      grade,
    });
  } catch (error) {
    console.error('Failed to record grade:', error);
  }

  reviewIndex += 1;
  reviewAnswered = false;

  if (reviewIndex >= reviewQueue.length) {
    await loadReviewQueue();
  }
}
</script>

{#if open}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="panel-overlay visible" onclick={onClose}></div>
{:else}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="panel-overlay" onclick={onClose}></div>
{/if}

<div class={`review-panel ${open ? "open" : ""}`}>
  <div class="review-panel-header">
    <h2 class="font-semibold" style="color: var(--text-primary); font-size: var(--text-lg);">Review</h2>
    <button onclick={onClose} style="color: var(--text-muted); font-size: var(--text-xl);">&times;</button>
  </div>
  <div class="review-panel-content">
    {#if reviewQueue.length === 0}
      <div class="review-empty">
        <svg fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
        </svg>
        <p class="font-medium" style="font-size: var(--text-base);">All caught up!</p>
        <p style="font-size: var(--text-sm);">No cards due for review right now.</p>
      </div>
    {:else}
      {#if reviewQueue[reviewIndex]}
        <div class="review-card">
          <div class="headword">{reviewQueue[reviewIndex].headword}</div>
          {#if !reviewAnswered}
            <button class="reveal-btn" onclick={revealAnswer}>Show Answer</button>
          {/if}
          <div class={`answer-section ${reviewAnswered ? "" : "hidden"}`}>
            <div class="pinyin">{reviewQueue[reviewIndex].pinyin}</div>
            <div class="english">{reviewQueue[reviewIndex].english}</div>
            {#if reviewQueue[reviewIndex].snippets?.length}
              <div class="snippet">"{reviewQueue[reviewIndex].snippets[0]}"</div>
            {/if}
            <div class="grade-buttons">
              <button class="grade-btn again" onclick={() => gradeCard(0)}>Again</button>
              <button class="grade-btn hard" onclick={() => gradeCard(1)}>Hard</button>
              <button class="grade-btn good" onclick={() => gradeCard(2)}>Good</button>
            </div>
          </div>
        </div>
      {/if}
    {/if}
  </div>
  {#if reviewQueue.length > 0}
    <div class="review-progress">
      <span>{reviewIndex + 1}</span> / <span>{reviewQueue.length}</span>
    </div>
  {/if}
</div>
