<script lang="ts">
import { translateBatch } from '@/features/translation/api';
import type { DisplaySentence, SentenceMeta, SegmentResult } from '@/features/translation/types';
import { getPastelColor } from '@/features/translation/utils';
import Button from '@/ui/Button.svelte';

const {
  translationResults,
  sentenceMeta,
  currentTranslationId,
  currentRawText,
  onSave,
  onCancel,
}: {
  translationResults: SegmentResult[];
  sentenceMeta: SentenceMeta[];
  currentTranslationId: string | null;
  currentRawText: string;
  onSave: (results: SegmentResult[], meta: SentenceMeta[]) => void;
  onCancel: () => void;
} = $props();

let workingResults = $state<SegmentResult[]>([]);
let workingMeta = $state<SentenceMeta[]>([]);
let pendingIndices = $state(new Set<number>());
let saving = $state(false);

// Initialize working copy on mount
$effect(() => {
  workingResults = translationResults.map((r) => ({ ...r }));
  workingMeta = sentenceMeta.map((m) => ({ ...m }));
  pendingIndices = new Set();
});

const workingSentences = $derived(buildWorkingSentences());

function buildWorkingSentences(): DisplaySentence[] {
  let globalIndex = 0;
  return workingMeta.map((sent, sentenceIdx) => {
    const segments = Array.from({ length: sent.segment_count }).map(() => {
      const existing = workingResults[globalIndex];
      const entry = existing
        ? { ...existing }
        : {
            segment: '',
            pinyin: '',
            english: '',
            index: globalIndex,
            sentence_index: sentenceIdx,
            pending: true,
          };
      entry.index = globalIndex;
      entry.sentence_index = sentenceIdx;
      globalIndex += 1;
      return entry;
    });
    return { ...sent, sentence_index: sentenceIdx, segments };
  });
}

function handleSplit(segmentIndex: number, splitAfterChar: number) {
  const seg = workingResults[segmentIndex];
  if (!seg) return;

  const text = seg.segment;
  if (splitAfterChar < 0 || splitAfterChar >= text.length - 1) return;

  const leftText = text.substring(0, splitAfterChar + 1);
  const rightText = text.substring(splitAfterChar + 1);

  const leftSeg: SegmentResult = {
    segment: leftText,
    pinyin: '...',
    english: `[${leftText}]`,
    index: segmentIndex,
    sentence_index: seg.sentence_index,
    pending: true,
  };
  const rightSeg: SegmentResult = {
    segment: rightText,
    pinyin: '...',
    english: `[${rightText}]`,
    index: segmentIndex + 1,
    sentence_index: seg.sentence_index,
    pending: true,
  };

  const next = workingResults.slice();
  next.splice(segmentIndex, 1, leftSeg, rightSeg);

  // Update meta: increment segment_count for this sentence
  const metaNext = workingMeta.map((m, i) =>
    i === seg.sentence_index ? { ...m, segment_count: m.segment_count + 1 } : { ...m }
  );

  workingMeta = metaNext;
  workingResults = reindex(next, metaNext);

  // Track pending indices (after reindex)
  const newPending = new Set(pendingIndices);
  // Find the two new segments by scanning for leftText and rightText at the right position
  newPending.add(segmentIndex);
  newPending.add(segmentIndex + 1);
  // Shift any existing pending indices above segmentIndex
  const shifted = new Set<number>();
  for (const idx of newPending) {
    if (idx > segmentIndex + 1 && pendingIndices.has(idx - 1)) {
      shifted.add(idx);
    } else {
      shifted.add(idx);
    }
  }
  pendingIndices = shifted;
}

function handleJoin(leftIndex: number, rightIndex: number) {
  const leftSeg = workingResults[leftIndex];
  const rightSeg = workingResults[rightIndex];
  if (!leftSeg || !rightSeg) return;

  const mergedText = leftSeg.segment + rightSeg.segment;
  const mergedSeg: SegmentResult = {
    segment: mergedText,
    pinyin: '...',
    english: `[${mergedText}]`,
    index: leftIndex,
    sentence_index: leftSeg.sentence_index,
    pending: true,
  };

  const next = workingResults.slice();
  next.splice(leftIndex, 2, mergedSeg);

  // Update meta: decrement segment_count for this sentence
  const metaNext = workingMeta.map((m, i) =>
    i === leftSeg.sentence_index ? { ...m, segment_count: m.segment_count - 1 } : { ...m }
  );

  workingMeta = metaNext;
  workingResults = reindex(next, metaNext);

  // Update pending: remove old indices, add merged
  const newPending = new Set<number>();
  for (const idx of pendingIndices) {
    if (idx === leftIndex || idx === rightIndex) continue;
    if (idx > rightIndex) {
      newPending.add(idx - 1);
    } else {
      newPending.add(idx);
    }
  }
  newPending.add(leftIndex);
  pendingIndices = newPending;
}

function reindex(results: SegmentResult[], meta: SentenceMeta[]): SegmentResult[] {
  let globalIndex = 0;
  const reindexed: SegmentResult[] = [];
  meta.forEach((sent, sentenceIdx) => {
    for (let i = 0; i < sent.segment_count; i++) {
      const existing = results[globalIndex];
      if (existing) {
        reindexed.push({
          ...existing,
          index: globalIndex,
          sentence_index: sentenceIdx,
        });
      }
      globalIndex++;
    }
  });
  return reindexed;
}

async function save() {
  if (pendingIndices.size === 0) {
    onSave(workingResults, workingMeta);
    return;
  }

  saving = true;
  try {
    // Identify the affected sentence
    const sentenceIdx = workingResults[[...pendingIndices][0]]?.sentence_index ?? null;

    if (sentenceIdx === null) {
      onSave(workingResults, workingMeta);
      return;
    }

    // Get ALL segments for this sentence (not just pending ones)
    // This is required because the backend replaces all segments for the sentence
    const sentenceSegmentIndices: number[] = [];
    const allSegmentTexts: string[] = [];

    workingResults.forEach((seg, idx) => {
      if (seg.sentence_index === sentenceIdx) {
        sentenceSegmentIndices.push(idx);
        allSegmentTexts.push(seg.segment);
      }
    });

    const data = await translateBatch(
      allSegmentTexts,
      currentRawText || null,
      currentTranslationId,
      sentenceIdx
    );

    // Apply translations to ALL segments in the sentence
    const next = workingResults.map((r) => ({ ...r }));
    sentenceSegmentIndices.forEach((idx, i) => {
      if (data.translations[i] && next[idx]) {
        next[idx] = {
          ...next[idx],
          segment: data.translations[i].segment,
          pinyin: data.translations[i].pinyin,
          english: data.translations[i].english,
          pending: false,
        };
      }
    });

    onSave(next, workingMeta);
  } catch (error) {
    console.error('Failed to translate segments:', error);
    saving = false;
  }
}

function cancel() {
  onCancel();
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    cancel();
  }
}
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="edit-mode-active">
  <div id="segments-container">
    {#each workingSentences as sent}
      <div
        class="sentence"
        style={`margin-bottom: ${sent.separator ? sent.separator.split("\n").length * 0.4 : 0}rem; padding-left: ${sent.indent ? sent.indent.length * 0.5 : 0}rem;`}
      >
        {#each sent.segments as segment, segIdx (segment.index)}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <span
            class={`segment editing ${pendingIndices.has(segment.index) ? "segment-pending" : ""}`}
          >
            {#each segment.segment.split("") as char, charIdx}
              <!-- svelte-ignore a11y_click_events_have_key_events -->
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <span
                class="char-wrapper"
                style={`background: ${getPastelColor(segment.index)}; border-radius: 3px;`}
              >
                {char}
                {#if charIdx < segment.segment.length - 1}
                  <!-- svelte-ignore a11y_click_events_have_key_events -->
                  <!-- svelte-ignore a11y_no_static_element_interactions -->
                  <span
                    class="split-point"
                    title="Split here"
                    onclick={(e: MouseEvent) => {
                      e.stopPropagation();
                      handleSplit(segment.index, charIdx);
                    }}
                  ></span>
                {/if}
              </span>
            {/each}
          </span>

          {#if segIdx < sent.segments.length - 1}
            <!-- svelte-ignore a11y_click_events_have_key_events -->
            <!-- svelte-ignore a11y_no_static_element_interactions -->
            <span
              class="join-indicator visible"
              title="Join segments"
              onclick={(e: MouseEvent) => {
                e.stopPropagation();
                handleJoin(segment.index, sent.segments[segIdx + 1].index);
              }}
            >
              &#8853;
            </span>
          {/if}
        {/each}
      </div>
    {/each}
  </div>

  <div class="segment-edit-bar">
    <span class="edit-bar-status">
      <span class="pending-count">{pendingIndices.size}</span> changes
    </span>
    <div class="edit-bar-actions">
      <Button variant="ghost" size="sm" onclick={cancel}>Cancel</Button>
      <Button variant="primary" size="sm" onclick={save} disabled={saving} loading={saving}>
        Save Changes
      </Button>
    </div>
  </div>
</div>

<style>
  .sentence {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
    align-items: center;
  }

  .segment {
    display: inline-block;
    border-radius: var(--radius-md);
    border: 2px solid transparent;
    font-family: var(--font-chinese);
    font-size: var(--text-chinese);
    color: var(--text-primary);
  }

  .segment.editing {
    cursor: default;
    padding: 0;
    background: transparent !important;
  }

  .char-wrapper {
    display: inline-block;
    padding: var(--space-1) calc(var(--space-unit) * 0.6);
    position: relative;
    cursor: default;
  }

  .segment.editing .char-wrapper:not(:last-child)::after {
    content: '';
    position: absolute;
    right: -1px;
    top: 15%;
    height: 70%;
    width: 2px;
    background: var(--border);
    border-radius: 1px;
    transition: all 0.15s ease;
    opacity: 0.5;
  }

  .segment.editing .char-wrapper:not(:last-child):hover::after {
    background: var(--primary);
    box-shadow: 0 0 4px var(--primary);
    opacity: 1;
  }

  .split-point {
    position: absolute;
    right: calc(var(--space-unit) * -2);
    top: 0;
    width: var(--space-8);
    height: 100%;
    cursor: pointer;
    z-index: 5;
  }

  .split-point:hover {
    background: var(--primary-alpha);
  }

  .segment-pending {
    background: var(--background-alt) !important;
    border: 2px dashed var(--border) !important;
    opacity: 0.8;
  }

  .join-indicator {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: var(--space-8);
    height: var(--space-8);
    margin: 0 var(--space-1);
    opacity: 0;
    transition: all 0.15s ease;
    cursor: pointer;
    color: var(--text-muted);
    font-size: calc(var(--space-unit) * 5);
    font-weight: 300;
    position: relative;
    z-index: 10;
    vertical-align: baseline;
    border-radius: var(--radius-md);
    line-height: 1;
  }

  .join-indicator:hover {
    opacity: 1;
    color: var(--primary);
    background: rgba(108, 190, 237, 0.15);
  }

  .join-indicator.visible {
    opacity: 0.7;
  }

  .join-indicator.visible:hover {
    opacity: 1;
  }

  .segment-edit-bar {
    position: sticky;
    bottom: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-3) var(--space-4);
    background: var(--surface);
    border-top: 1px solid var(--border);
    box-shadow: 0 -2px 8px rgba(0, 0, 0, 0.05);
    margin: var(--space-4) calc(var(--space-unit) * -5) calc(var(--space-unit) * -5);
    border-radius: 0 0 var(--radius-xl) var(--radius-xl);
    z-index: 20;
  }

  .edit-bar-status {
    color: var(--text-muted);
    font-size: var(--text-sm);
  }

  .pending-count {
    font-weight: 600;
    color: var(--primary);
  }

  .edit-bar-actions {
    display: flex;
    gap: var(--space-2);
  }
</style>
