<script lang="ts">
import { tick } from 'svelte';
import Button from '@/ui/Button.svelte';
import type {
  DisplayParagraph,
  LoadingState,
  ProgressState,
  SavedVocabInfo,
  SegmentResult,
  TooltipState,
} from '@/features/translation/types';
import { getPastelColor } from '@/features/translation/utils';

const {
  displayParagraphs,
  savedVocabMap,
  progress,
  onSaveVocab,
  onMarkKnown,
  onResumeLearning,
  onRecordLookup,
}: {
  displayParagraphs: DisplayParagraph[];
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
} = $props();

let tooltipVisible = $state(false);
let tooltipPinned = $state(false);
let tooltipHideTimeout = $state<number | null>(null);
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

function isHTMLElement(value: EventTarget | null): value is HTMLElement {
  return value instanceof HTMLElement;
}

function getSegmentStyle(segment: SegmentResult) {
  const info = savedVocabMap.get(segment.segment);
  const baseColor = getPastelColor(segment.index || 0);
  const styles: string[] = [];
  if (info) {
    styles.push(`--segment-color: ${baseColor}`);
    styles.push(`--segment-opacity: ${info.opacity}`);
  } else if (!segment.pending && segment.pinyin) {
    styles.push(`background: ${baseColor}`);
  }
  return styles.join('; ');
}

function getSegmentClasses(segment: SegmentResult) {
  const classes = ['segment'];
  if (segment.pending) classes.push('segment-pending');
  if (segment.pinyin || segment.english) {
    classes.push('segment-interactive');
  }
  const info = savedVocabMap.get(segment.segment);
  if (info) {
    classes.push('saved');
    if (info.isStruggling) classes.push('struggling');
  }
  return classes.join(' ');
}

async function handleSegmentHover(segment: SegmentResult, element: EventTarget | null) {
  if (tooltipPinned) return;
  if (tooltipHideTimeout !== null) {
    window.clearTimeout(tooltipHideTimeout);
    tooltipHideTimeout = null;
  }
  if (isHTMLElement(element)) {
    lastHoveredElement = element;
  }
  await showTooltip(segment, element, false);
}

function handleSegmentLeave() {
  if (!tooltipPinned) {
    tooltipHideTimeout = window.setTimeout(() => {
      tooltipVisible = false;
    }, 150);
  }
}

function handleTooltipEnter() {
  if (tooltipHideTimeout !== null) {
    window.clearTimeout(tooltipHideTimeout);
    tooltipHideTimeout = null;
  }
}

function handleTooltipLeave() {
  if (!tooltipPinned) {
    tooltipVisible = false;
  }
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

async function toggleSegmentPin(segment: SegmentResult, element: EventTarget | null) {
  if (tooltipPinned && tooltip.headword === segment.segment) {
    tooltipPinned = false;
    tooltipVisible = false;
    lastHoveredElement = null;
    return;
  }
  tooltipPinned = true;
  if (isHTMLElement(element)) {
    lastHoveredElement = element;
  }
  await showTooltip(segment, element, true);

  const info = savedVocabMap.get(segment.segment);
  if (info?.vocabItemId) {
    await onRecordLookup(segment.segment, info.vocabItemId);
  }
}

async function showTooltip(segment: SegmentResult, element: EventTarget | null, pinned: boolean) {
  const info = savedVocabMap.get(segment.segment);
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
  tooltipPinned = pinned;
  await tick();

  if (!tooltipRef || !isHTMLElement(element)) return;
  const segRect = element.getBoundingClientRect();
  const tooltipRect = tooltipRef.getBoundingClientRect();

  let left = segRect.left + segRect.width / 2 - tooltipRect.width / 2;
  let top = segRect.top - tooltipRect.height - 8;

  const viewportWidth = window.innerWidth;

  if (left < 8) {
    left = 8;
  }
  if (left + tooltipRect.width > viewportWidth - 8) {
    left = viewportWidth - tooltipRect.width - 8;
  }
  if (top < 8) {
    top = segRect.bottom + 8;
  }

  tooltip = { ...tooltip, x: left, y: top };
}

function handleGlobalClick(event: MouseEvent) {
  if (!tooltipPinned) return;
  const target = event.target as HTMLElement | null;
  if (tooltipRef?.contains(target)) return;
  if (target?.closest?.('.segment')) return;
  tooltipPinned = false;
  tooltipVisible = false;
  lastHoveredElement = null;
}

async function saveVocab() {
  if (!tooltip.headword) return;
  const info = await onSaveVocab(tooltip.headword, tooltip.pinyin, tooltip.english);
  if (info) {
    tooltip = { ...tooltip, vocabItemId: info.vocabItemId, status: 'learning' };
  }
}

async function markKnown() {
  if (!tooltip.vocabItemId) return;
  await onMarkKnown(tooltip.headword, tooltip.vocabItemId);
  tooltip = { ...tooltip, status: 'known' };
}

async function resumeLearning() {
  if (!tooltip.vocabItemId) return;
  await onResumeLearning(tooltip.headword, tooltip.vocabItemId);
  tooltip = { ...tooltip, status: 'learning' };
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
    {#each displayParagraphs as para}
      <div class="paragraph" style={`margin-bottom: ${para.separator ? para.separator.split("\n").length * 0.4 : 0}rem; padding-left: ${para.indent ? para.indent.length * 0.5 : 0}rem;`}>
        {#each para.segments as segment (segment.index)}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <span
            class={getSegmentClasses(segment)}
            style={getSegmentStyle(segment)}
            onmouseenter={(event: MouseEvent) => handleSegmentHover(segment, event.currentTarget)}
            onmouseleave={handleSegmentLeave}
            onclick={(event: MouseEvent) => {
              event.stopPropagation();
              toggleSegmentPin(segment, event.currentTarget);
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
    onmouseenter={handleTooltipEnter}
    onmouseleave={handleTooltipLeave}
  >
    <div class="tooltip-pinyin">{tooltip.pinyin}</div>
    <div class="tooltip-english">{tooltip.english}</div>
    <div class="tooltip-actions">
      {#if tooltip.pinyin || tooltip.english}
        {#if !tooltip.vocabItemId}
          <Button variant="secondary" size="xs" shape="pill" onclick={saveVocab}>
            Save to Learn
          </Button>
        {:else if tooltip.status === "learning"}
          <Button variant="secondary" size="xs" shape="pill" onclick={markKnown}>
            Mark as Known
          </Button>
        {:else if tooltip.status === "known"}
          <Button variant="secondary" size="xs" shape="pill" onclick={resumeLearning}>
            Resume Learning
          </Button>
        {:else}
          <Button variant="secondary" size="xs" shape="pill" onclick={saveVocab}>
            Save to Learn
          </Button>
        {/if}
      {/if}
    </div>
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

  /* Paragraphs */
  .paragraph {
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
    color: var(--text-primary);
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
  }

  .segment.saved::before {
    content: '';
    position: absolute;
    inset: 0;
    border-radius: inherit;
    background: var(--segment-color, transparent);
    opacity: var(--segment-opacity, 1);
    z-index: -1;
  }

  .segment.struggling {
    text-decoration: underline dotted var(--text-muted);
  }

  /* Tooltip */
  .word-tooltip {
    position: fixed;
    z-index: var(--z-tooltip);
    min-width: 140px;
    max-width: 240px;
    padding: var(--space-3) var(--space-4);
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

  .tooltip-pinyin {
    font-family: var(--font-body);
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--primary-dark);
    margin-bottom: var(--space-1);
    letter-spacing: var(--tracking-tight);
  }

  .tooltip-english {
    font-family: var(--font-body);
    font-size: var(--text-xs);
    color: var(--text-secondary);
    line-height: var(--leading-snug);
    margin-bottom: var(--space-2);
  }

  .tooltip-actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }

</style>
