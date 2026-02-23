<script lang="ts">
import { tick } from 'svelte';
import Button from '@/ui/Button.svelte';
import type {
  DisplaySentence,
  LoadingState,
  ProgressState,
  SavedVocabInfo,
  SegmentResult,
  TooltipState,
} from '@/features/translation/types';
import { getPastelColor } from '@/features/translation/utils';

const {
  displaySentences,
  savedVocabMap,
  progress,
  onSaveVocab,
  onMarkKnown,
  onResumeLearning,
  onRecordLookup,
  onGradeReview,
}: {
  displaySentences: DisplaySentence[];
  savedVocabMap: Map<string, SavedVocabInfo>;
  progress: ProgressState;
  fullTranslation: string;
  loadingState: LoadingState;
  errorMessage: string;
  onSaveVocab: (
    headword: string,
    pinyin: string,
    english: string
  ) => Promise<SavedVocabInfo | null>;
  onMarkKnown: (headword: string, vocabItemId: string) => Promise<void>;
  onResumeLearning: (headword: string, vocabItemId: string) => Promise<void>;
  onRecordLookup: (headword: string, vocabItemId: string) => Promise<void>;
  onGradeReview: (vocabItemId: string, grade: number) => Promise<void>;
} = $props();

let tooltipVisible = $state(false);
let tooltip = $state<TooltipState>({
  headword: '',
  pinyin: '',
  english: '',
  vocabItemId: null,
  status: '',
  x: 0,
  y: 0,
});

let resultsContainer = $state<HTMLDivElement | null>(null);
let tooltipRef = $state<HTMLDivElement | null>(null);
let lastHoveredElement = $state<HTMLElement | null>(null);
let reviewSubmitting = $state(false);
let reviewError = $state('');

$effect(() => {
  if (!tooltipVisible || !tooltip.headword) return;
  const info = savedVocabMap.get(tooltip.headword);
  if (!info) {
    if (tooltip.vocabItemId !== null || tooltip.status !== '') {
      tooltip = { ...tooltip, vocabItemId: null, status: '' };
    }
    return;
  }

  if (tooltip.vocabItemId !== info.vocabItemId || tooltip.status !== info.status) {
    tooltip = { ...tooltip, vocabItemId: info.vocabItemId, status: info.status };
  }
});

const currentVocabInfo = $derived(
  tooltip.headword ? (savedVocabMap.get(tooltip.headword) ?? null) : null
);

function isHTMLElement(value: EventTarget | null): value is HTMLElement {
  return value instanceof HTMLElement;
}

function isSkippedSegment(segment: SegmentResult): boolean {
  return !segment.pending && !segment.pinyin && !segment.english;
}

function getSegmentStyle(segment: SegmentResult) {
  const info = savedVocabMap.get(segment.segment);
  const baseColor = getPastelColor(segment.index || 0);
  const isSkipped = isSkippedSegment(segment);
  const styles: string[] = [];
  if (info?.status === 'learning') {
    styles.push('--segment-color: var(--primary)');
    styles.push('--segment-text-color: var(--surface)');
    styles.push(`--segment-opacity: ${info.opacity}`);
  } else if (info?.status === 'known' || isSkipped) {
    styles.push('background: transparent');
  } else if (!segment.pending && segment.pinyin) {
    styles.push(`background: ${baseColor}`);
  }
  return styles.join('; ');
}

function getSegmentClasses(segment: SegmentResult) {
  const classes = ['segment'];
  const isSkipped = isSkippedSegment(segment);
  if (segment.pending) classes.push('segment-pending');
  if (segment.pinyin || segment.english) {
    classes.push('segment-interactive');
  }
  const info = savedVocabMap.get(segment.segment);
  if (info?.status === 'learning') {
    classes.push('saved');
    classes.push('status-learning');
    if (info.isStruggling) classes.push('struggling');
  } else if (info?.status === 'known') {
    classes.push('status-known');
  } else if (isSkipped) {
    classes.push('status-skipped');
  }
  return classes.join(' ');
}

function updateTooltipPosition() {
  if (!tooltipVisible || !tooltipRef) return;

  if (lastHoveredElement && document.contains(lastHoveredElement)) {
    const segRect = lastHoveredElement.getBoundingClientRect();
    const tooltipRect = tooltipRef.getBoundingClientRect();

    let left = segRect.left + segRect.width / 2 - tooltipRect.width / 2;
    let top = segRect.top - tooltipRect.height - 8;

    const viewportWidth = window.innerWidth;

    if (left < 8) left = 8;
    if (left + tooltipRect.width > viewportWidth - 8) {
      left = viewportWidth - tooltipRect.width - 8;
    }
    if (top < 8) {
      top = segRect.bottom + 8;
    }

    tooltip = { ...tooltip, x: left, y: top };
  }
}

async function handleSegmentClick(segment: SegmentResult, element: EventTarget | null) {
  if (isSkippedSegment(segment)) return;
  if (tooltipVisible && tooltip.headword === segment.segment) {
    tooltipVisible = false;
    lastHoveredElement = null;
    return;
  }
  if (isHTMLElement(element)) {
    lastHoveredElement = element;
  }
  const info = savedVocabMap.get(segment.segment);
  reviewSubmitting = false;
  reviewError = '';
  tooltip = {
    headword: segment.segment,
    pinyin: segment.pinyin || '',
    english: segment.english || '',
    vocabItemId: info?.vocabItemId || null,
    status: info?.status || '',
    x: 0,
    y: 0,
  };
  tooltipVisible = true;
  await tick();

  if (tooltipRef && isHTMLElement(element)) {
    const segRect = element.getBoundingClientRect();
    const tooltipRect = tooltipRef.getBoundingClientRect();
    let left = segRect.left + segRect.width / 2 - tooltipRect.width / 2;
    let top = segRect.top - tooltipRect.height - 8;
    const viewportWidth = window.innerWidth;
    if (left < 8) left = 8;
    if (left + tooltipRect.width > viewportWidth - 8) left = viewportWidth - tooltipRect.width - 8;
    if (top < 8) top = segRect.bottom + 8;
    tooltip = { ...tooltip, x: left, y: top };
  }

  if (info?.status === 'learning' && info.vocabItemId) {
    await onRecordLookup(segment.segment, info.vocabItemId);
  }
}

function handleGlobalClick(_event: MouseEvent) {
  if (!tooltipVisible) return;
  tooltipVisible = false;
  lastHoveredElement = null;
}

async function saveVocab() {
  if (!tooltip.headword) return;
  await onSaveVocab(tooltip.headword, tooltip.pinyin, tooltip.english);
  tooltipVisible = false;
}

async function markKnown() {
  let vocabItemId = tooltip.vocabItemId;
  if (!vocabItemId) {
    // Unknown word — save it first so we have an ID to mark known
    const info = await onSaveVocab(tooltip.headword, tooltip.pinyin, tooltip.english);
    if (!info) return;
    vocabItemId = info.vocabItemId;
  }
  await onMarkKnown(tooltip.headword, vocabItemId);
  tooltipVisible = false;
}

async function resumeLearning() {
  if (!tooltip.vocabItemId) return;
  await onResumeLearning(tooltip.headword, tooltip.vocabItemId);
  tooltipVisible = false;
}

async function gradeLearningSegment(grade: number) {
  if (!tooltip.vocabItemId || reviewSubmitting) return;
  reviewSubmitting = true;
  reviewError = '';
  try {
    await onGradeReview(tooltip.vocabItemId, grade);
    tooltipVisible = false;
  } catch (error) {
    reviewError = error instanceof Error ? error.message : 'Failed to save review grade.';
  } finally {
    reviewSubmitting = false;
  }
}

function formatDueLabel(nextDueAt: string | null | undefined): string {
  if (!nextDueAt) return 'New';
  const diff = new Date(nextDueAt).getTime() - Date.now();
  const days = Math.ceil(diff / 86_400_000);
  if (days <= 0) return 'Due now';
  if (days === 1) return 'Due tomorrow';
  if (days < 7) return `Due in ${days}d`;
  if (days < 30) return `Due in ${Math.round(days / 7)}w`;
  return `Due in ${Math.round(days / 30)}mo`;
}
</script>

<svelte:window onclick={handleGlobalClick} onscroll={updateTooltipPosition} />

<div class="segment-results" bind:this={resultsContainer}>
  {#if progress.total > 0 && progress.current < progress.total}
    <div class="progress-container">
      <div class="progress-header">
        <span>Translating...</span>
        <span>{progress.current} / {progress.total}</span>
      </div>
      <div class="progress-bar-bg">
        <div class="progress-bar-fill" style={`width: ${(progress.current / progress.total) * 100}%`}></div>
      </div>
    </div>
  {/if}

  <div id="segments-container">
    {#each displaySentences as sent}
      <div class="sentence" style={`margin-bottom: ${sent.separator ? sent.separator.split("\n").length * 0.4 : 0}rem; padding-left: ${sent.indent ? sent.indent.length * 0.5 : 0}rem;`}>
        {#each sent.segments as segment (segment.index)}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <span
            class={getSegmentClasses(segment)}
            style={getSegmentStyle(segment)}
            onclick={(event: MouseEvent) => {
              event.stopPropagation();
              handleSegmentClick(segment, event.currentTarget);
            }}
          >
            {segment.segment}
          </span>
        {/each}
      </div>
    {/each}
  </div>

  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="word-tooltip"
    class:hidden={!tooltipVisible}
    bind:this={tooltipRef}
    style={`left: ${tooltip.x}px; top: ${tooltip.y}px;`}
    onclick={(e) => e.stopPropagation()}
  >
    <div class="tooltip-header">
      <div class="tooltip-pinyin">{tooltip.pinyin}</div>
      <div class="tooltip-english">{tooltip.english}</div>
    </div>

    {#if tooltip.pinyin || tooltip.english}
      {#if tooltip.status === "learning" && tooltip.vocabItemId}
        <div class="tooltip-divider"></div>
        {#if currentVocabInfo}
          <div class="tooltip-stats">
            <span>{formatDueLabel(currentVocabInfo.nextDueAt)}</span>
            {#if currentVocabInfo.isStruggling}
              <span class="tooltip-stats-struggling">⚠ Struggling</span>
            {/if}
          </div>
        {/if}
        <div class="tooltip-actions">
          <div class="tooltip-grade-buttons">
            <button
              class="grade-btn again"
              onclick={() => gradeLearningSegment(0)}
              disabled={reviewSubmitting}
            >
              Again
            </button>
            <button
              class="grade-btn hard"
              onclick={() => gradeLearningSegment(1)}
              disabled={reviewSubmitting}
            >
              Hard
            </button>
            <button
              class="grade-btn good"
              onclick={() => gradeLearningSegment(2)}
              disabled={reviewSubmitting}
            >
              Good
            </button>
          </div>
          {#if reviewError}
            <div class="tooltip-review-error">{reviewError}</div>
          {/if}
          <button class="tooltip-mark-known-btn" onclick={markKnown} disabled={reviewSubmitting}>
            Mark as Known
          </button>
        </div>
      {:else if !tooltip.vocabItemId}
        <div class="tooltip-actions tooltip-actions-row">
          <Button variant="secondary" size="xs" shape="pill" onclick={saveVocab}>
            Save to Learn
          </Button>
          <Button variant="secondary" size="xs" shape="pill" onclick={markKnown}>Mark as Known</Button>
        </div>
      {:else if tooltip.status === "known"}
        <div class="tooltip-actions">
          <Button variant="secondary" size="xs" shape="pill" onclick={resumeLearning}>
            Resume Learning
          </Button>
        </div>
      {:else}
        <div class="tooltip-actions">
          <Button variant="secondary" size="xs" shape="pill" onclick={saveVocab}>
            Save to Learn
          </Button>
        </div>
      {/if}
    {/if}
  </div>
</div>

<style>
  .segment-results {
    position: relative;
  }

  /* Progress Bar */
  .progress-container {
    margin-bottom: var(--space-3);
  }

  .progress-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: var(--space-2);
    font-size: var(--text-xs);
    color: var(--text-secondary);
  }

  .progress-bar-bg {
    width: 100%;
    height: 4px;
    background: var(--border);
    border-radius: 2px;
    overflow: hidden;
  }

  .progress-bar-fill {
    height: 100%;
    background: linear-gradient(90deg, var(--primary-light), var(--primary));
    border-radius: 2px;
    transition: width 0.3s ease;
  }

  /* Sentences */
  .sentence {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
  }

  /* Segments */
  .segment {
    display: inline-block;
    padding: var(--space-1) var(--space-2);
    border-radius: var(--radius-md);
    border: 2px solid transparent;
    font-family: var(--font-chinese);
    font-size: var(--text-chinese);
    color: var(--surface);
    transition: all 0.15s ease;
  }

  .segment-interactive {
    cursor: pointer;
  }

  .segment-interactive:hover {
    transform: translateY(-1px);
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
  }

  /* Pending segment */
  .segment-pending {
    background: var(--background-alt) !important;
    border: 2px dashed var(--border) !important;
    opacity: 0.8;
  }

  .segment-pending:hover {
    transform: none;
    box-shadow: none;
  }

  /* SRS saved word opacity */
  .segment.saved {
    position: relative;
    background: transparent !important;
    isolation: isolate;
  }

  .segment.saved::before {
    content: '';
    position: absolute;
    inset: 0;
    border-radius: inherit;
    background: var(--segment-color, var(--primary));
    opacity: var(--segment-opacity, 1);
    z-index: -1;
  }

  .segment.struggling {
    text-decoration: underline dotted var(--text-muted);
  }

  .segment.status-learning {
    color: var(--segment-text-color, var(--surface));
  }

  .segment.status-known,
  .segment.status-skipped {
    background: transparent !important;
    color: var(--text-primary) !important;
  }

  /* Tooltip */
  .word-tooltip {
    position: fixed;
    z-index: var(--z-tooltip);
    min-width: 140px;
    max-width: 260px;
    padding: var(--space-3);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12), 0 2px 6px rgba(0, 0, 0, 0.04);
    pointer-events: auto;
    opacity: 0;
    transform: translateY(4px);
    animation: tooltipFadeIn 0.15s ease forwards;
  }

  .word-tooltip.hidden {
    display: none;
  }

  @keyframes tooltipFadeIn {
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .tooltip-header {
    margin-bottom: var(--space-2);
  }

  .tooltip-pinyin {
    font-family: var(--font-body);
    font-size: var(--text-base);
    font-weight: 600;
    color: var(--primary-dark);
    margin-bottom: 2px;
    letter-spacing: var(--tracking-tight);
  }

  .tooltip-english {
    font-family: var(--font-body);
    font-size: var(--text-sm);
    color: var(--text-secondary);
    line-height: var(--leading-snug);
  }

  .tooltip-divider {
    height: 1px;
    background: var(--border);
    margin: var(--space-2) 0;
  }

  .tooltip-stats {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-bottom: var(--space-2);
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .tooltip-stats-struggling {
    color: var(--review-again);
    font-weight: 600;
  }

  .tooltip-actions {
    display: flex;
    flex-direction: column;
    align-items: stretch;
    gap: var(--space-2);
  }

  /* Single-row layout for unknown word buttons */
  .tooltip-actions-row {
    flex-direction: row;
    flex-wrap: nowrap;
    align-items: center;
  }

  .tooltip-grade-buttons {
    display: flex;
    gap: var(--space-2);
    width: 100%;
    min-width: 190px;
  }

  .grade-btn {
    flex: 1;
    border: none;
    border-radius: var(--radius-md);
    cursor: pointer;
    font-size: var(--text-xs);
    font-weight: 600;
    padding: var(--space-1) 0;
    text-align: center;
    transition: all 0.15s ease;
  }

  .grade-btn:disabled {
    cursor: not-allowed;
    opacity: 0.6;
  }

  .grade-btn.again {
    background: var(--review-again);
    color: var(--text-primary);
  }

  .grade-btn.hard {
    background: var(--review-hard);
    color: var(--text-primary);
  }

  .grade-btn.good {
    background: var(--review-good);
    color: var(--surface);
  }

  .tooltip-mark-known-btn {
    background: none;
    border: none;
    width: 100%;
    font-size: var(--text-xs);
    font-family: var(--font-body);
    color: var(--text-muted);
    cursor: pointer;
    padding: var(--space-1) 0;
    text-align: center;
    transition: color 0.15s ease;
  }

  .tooltip-mark-known-btn:hover {
    color: var(--text-secondary);
  }

  .tooltip-mark-known-btn:disabled {
    cursor: not-allowed;
    opacity: 0.5;
  }

  .tooltip-review-error {
    color: var(--error);
    font-size: var(--text-xs);
    width: 100%;
  }
</style>
