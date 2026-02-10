<script lang="ts">
import { Check } from '@lucide/svelte';
import { reviewStore } from '@/features/translation/stores/reviewStore.svelte';
import { translationStore } from '@/features/translation/stores/translationStore.svelte';
import { vocabStore } from '@/features/translation/stores/vocabStore.svelte';
import type { VocabStatsResponse } from '@/features/translation/types';
import { getJson } from '@/lib/api';
import { router } from '@/lib/router.svelte';
import Button from '@/ui/Button.svelte';

let vocabStats = $state({ known: 0, learning: 0, total: 0 });
let loading = $state(true);
let isReviewMode = $state(false);

const currentCard = $derived(reviewStore.currentCard);
const progress = $derived(reviewStore.progress);
const progressCurrent = $derived(
  progress.total === 0 ? 0 : Math.min(progress.current, progress.total)
);
const currentSnippet = $derived(currentCard?.snippets?.[0] ?? '');
const snippetPreview = $derived(truncateSnippet(currentSnippet));
const snippetTranslationId = $derived(findTranslationIdForSnippet(currentSnippet));

$effect(() => {
  void Promise.all([loadStats(), vocabStore.loadDueCount()]);
});

async function loadStats() {
  try {
    const response = await getJson<VocabStatsResponse>('/admin/api/profile');
    vocabStats = response.vocabStats;
  } catch (error) {
    console.error('Failed to load vocab stats:', error);
  } finally {
    loading = false;
  }
}

async function enterReview() {
  isReviewMode = true;
  await reviewStore.loadQueue();
}

function exitReview() {
  isReviewMode = false;
  void Promise.all([loadStats(), vocabStore.loadDueCount()]);
}

function truncateSnippet(snippet: string, maxWords: number = 18, maxChars: number = 90): string {
  const normalized = snippet.trim().replace(/\s+/g, ' ');
  if (!normalized) return '';

  const words = normalized.split(' ');
  if (words.length > 1) {
    return words.length > maxWords ? `${words.slice(0, maxWords).join(' ')}...` : normalized;
  }

  return normalized.length > maxChars ? `${normalized.slice(0, maxChars).trim()}...` : normalized;
}

function findTranslationIdForSnippet(snippet: string): string | null {
  const normalizedSnippet = snippet.trim().toLowerCase();
  if (!normalizedSnippet) return null;

  const match = translationStore.translations.find((translation) => {
    const inputPreview = translation.input_preview.toLowerCase();
    const fullPreview = (translation.full_translation_preview ?? '').toLowerCase();
    return (
      inputPreview.includes(normalizedSnippet) ||
      fullPreview.includes(normalizedSnippet) ||
      normalizedSnippet.includes(inputPreview)
    );
  });

  return match?.id ?? null;
}

function openSnippetTranslation() {
  if (!snippetTranslationId) return;
  router.navigateTo(snippetTranslationId);
}
</script>

<h2 class="vocab-header">Vocabulary</h2>

{#if loading}
  <p class="loading-text">Loading...</p>
{:else}
  <div class="vocab-review-layout">
    <div class="vocab-stats-grid">
      <div class="stat-item">
        <span class="stat-label">Known</span>
        <span class="stat-value">{vocabStats.known}</span>
      </div>
      <div class="stat-item">
        <span class="stat-label">Learning</span>
        <span class="stat-value">{vocabStats.learning}</span>
      </div>
      <div class="stat-item">
        <span class="stat-label">Total</span>
        <span class="stat-value">{vocabStats.total}</span>
      </div>
    </div>

    <div class="vocab-review-card">
      <div class="review-header">
        <h3 class="card-title">Review</h3>
        {#if isReviewMode && progress.total > 0}
          <span class="progress-counter">{progressCurrent} / {progress.total}</span>
        {/if}
      </div>

      {#if !isReviewMode}
        <p class="review-summary">
          {vocabStore.dueCount} card{vocabStore.dueCount === 1 ? '' : 's'} due
        </p>
        <div class="review-action">
          <Button
            variant="primary"
            size="md"
            onclick={enterReview}
            disabled={vocabStore.dueCount === 0}
          >
            Start Review
          </Button>
        </div>
      {:else if reviewStore.isLoading}
        <p class="loading-text">Loading cards...</p>
      {:else if !currentCard}
        <div class="empty-state">
          <Check height={48} width={48} style="color: var(--review-good); margin: 0 auto var(--space-3);" />
          <p class="empty-title">All caught up!</p>
          <p class="empty-subtitle">No cards due for review right now.</p>
          <div class="review-action">
            <Button variant="secondary" size="sm" onclick={exitReview}>Done</Button>
          </div>
        </div>
      {:else}
        <div class="flashcard">
          <div class="headword">{currentCard.headword}</div>

          {#if !reviewStore.isAnswerRevealed}
            <Button variant="primary" size="lg" onclick={() => reviewStore.revealAnswer()}>
              Show Answer
            </Button>
          {:else}
            <div class="answer-section">
              <div class="pinyin">{currentCard.pinyin}</div>
              <div class="english">{currentCard.english}</div>

              {#if currentSnippet}
                <div class="snippet">"{snippetPreview}"</div>
              {/if}

              <div class="grade-buttons">
                <button class="grade-btn again" onclick={() => reviewStore.gradeCard(0)}>Again</button>
                <button class="grade-btn hard" onclick={() => reviewStore.gradeCard(1)}>Hard</button>
                <button class="grade-btn good" onclick={() => reviewStore.gradeCard(2)}>Good</button>
              </div>

              {#if currentSnippet}
                <div class="review-action">
                  <Button
                    variant="secondary"
                    size="sm"
                    onclick={openSnippetTranslation}
                    disabled={!snippetTranslationId}
                  >
                    {snippetTranslationId ? 'Open Translation' : 'Translation unavailable'}
                  </Button>
                </div>
              {/if}

              <div class="review-action">
                <Button variant="ghost" size="sm" onclick={exitReview}>End Review</Button>
              </div>
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .vocab-header {
    margin-bottom: var(--space-4);
    font-weight: normal;
    font-size: var(--text-2xl);
  }

  .vocab-review-layout {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .vocab-stats-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: var(--space-3);
  }

  .vocab-review-card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    padding: var(--space-6);
  }

  .stat-item {
    color: var(--text-secondary);
    font-size: var(--text-lg);
    font-weight: 600;
    background: var(--surface-2);
    border-radius: var(--radius-lg);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-2) var(--space-3);
  }

  .stat-label {
    color: var(--text-secondary);
    font-size: var(--text-xs);
  }

  .stat-value {
    color: var(--text-primary);
    font-size: var(--text-lg);
    font-weight: 600;
  }

  .review-header {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .card-title {
    color: var(--text-primary);
    font-size: var(--text-base);
    font-weight: 600;
    margin: 0;
  }

  .progress-counter {
    color: var(--text-muted);
    font-size: var(--text-sm);
    margin-left: auto;
  }

  .loading-text {
    color: var(--text-secondary);
    font-size: var(--text-sm);
    margin: var(--space-3) 0 0;
  }

  .review-summary {
    color: var(--text-secondary);
    font-size: var(--text-sm);
    margin: var(--space-3) 0;
  }

  .review-action {
    display: flex;
    justify-content: center;
    margin-top: var(--space-3);
  }

  .flashcard {
    text-align: center;
    padding: var(--space-4) 0;
  }

  .headword {
    color: var(--text-primary);
    font-family: var(--font-chinese);
    font-size: var(--text-4xl);
    margin-bottom: var(--space-6);
  }

  .answer-section {
    margin-top: var(--space-4);
  }

  .pinyin {
    color: var(--text-secondary);
    font-size: var(--text-lg);
    margin-bottom: var(--space-2);
  }

  .english {
    color: var(--primary-dark);
    font-size: var(--text-base);
    margin-bottom: var(--space-4);
  }

  .snippet {
    background: var(--background-alt);
    border-radius: var(--radius-lg);
    color: var(--text-muted);
    font-family: var(--font-chinese);
    font-size: var(--text-sm);
    margin-bottom: var(--space-6);
    padding: var(--space-3);
  }

  .grade-buttons {
    display: flex;
    gap: var(--space-3);
    justify-content: center;
    margin-top: var(--space-4);
  }

  .grade-btn {
    border: none;
    border-radius: var(--radius-md);
    cursor: pointer;
    font-size: var(--text-sm);
    font-weight: 500;
    padding: var(--space-2) var(--space-5);
    transition: all 0.15s ease;
  }

  .grade-btn:hover {
    box-shadow: 0 2px 6px var(--shadow);
    transform: translateY(-1px);
  }

  .grade-btn.again {
    background: var(--review-again);
    color: white;
  }

  .grade-btn.hard {
    background: var(--review-hard);
    color: var(--text-primary);
  }

  .grade-btn.good {
    background: var(--review-good);
    color: white;
  }

  .empty-state {
    color: var(--text-muted);
    padding: var(--space-8) var(--space-4);
    text-align: center;
  }

  .empty-title {
    color: var(--text-primary);
    font-size: var(--text-base);
    font-weight: 500;
    margin-bottom: var(--space-1);
  }

  .empty-subtitle {
    font-size: var(--text-sm);
  }
</style>
