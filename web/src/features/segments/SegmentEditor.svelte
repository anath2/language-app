<script lang="ts">
  import { getPastelColor } from "./utils";
  import { translateBatch } from "./api";
  import type {
    DisplayParagraph,
    ParagraphMeta,
    SegmentResult,
  } from "./types";

  let {
    displayParagraphs,
    translationResults,
    paragraphMeta,
    currentTranslationId,
    currentRawText,
    onSave,
    onCancel,
  }: {
    displayParagraphs: DisplayParagraph[];
    translationResults: SegmentResult[];
    paragraphMeta: ParagraphMeta[];
    currentTranslationId: string | null;
    currentRawText: string;
    onSave: (results: SegmentResult[], meta: ParagraphMeta[]) => void;
    onCancel: () => void;
  } = $props();

  let workingResults = $state<SegmentResult[]>([]);
  let workingMeta = $state<ParagraphMeta[]>([]);
  let pendingIndices = $state(new Set<number>());
  let saving = $state(false);

  // Initialize working copy on mount
  $effect(() => {
    workingResults = translationResults.map((r) => ({ ...r }));
    workingMeta = paragraphMeta.map((m) => ({ ...m }));
    pendingIndices = new Set();
  });

  const workingParagraphs = $derived(buildWorkingParagraphs());

  function buildWorkingParagraphs(): DisplayParagraph[] {
    let globalIndex = 0;
    return workingMeta.map((para, paraIdx) => {
      const segments = Array.from({ length: para.segment_count }).map(() => {
        const existing = workingResults[globalIndex];
        const entry = existing
          ? { ...existing }
          : {
              segment: "",
              pinyin: "",
              english: "",
              index: globalIndex,
              paragraph_index: paraIdx,
              pending: true,
            };
        entry.index = globalIndex;
        entry.paragraph_index = paraIdx;
        globalIndex += 1;
        return entry;
      });
      return { ...para, paragraph_index: paraIdx, segments };
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
      pinyin: "...",
      english: `[${leftText}]`,
      index: segmentIndex,
      paragraph_index: seg.paragraph_index,
      pending: true,
    };
    const rightSeg: SegmentResult = {
      segment: rightText,
      pinyin: "...",
      english: `[${rightText}]`,
      index: segmentIndex + 1,
      paragraph_index: seg.paragraph_index,
      pending: true,
    };

    const next = workingResults.slice();
    next.splice(segmentIndex, 1, leftSeg, rightSeg);

    // Update meta: increment segment_count for this paragraph
    const metaNext = workingMeta.map((m, i) =>
      i === seg.paragraph_index ? { ...m, segment_count: m.segment_count + 1 } : { ...m }
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
      pinyin: "...",
      english: `[${mergedText}]`,
      index: leftIndex,
      paragraph_index: leftSeg.paragraph_index,
      pending: true,
    };

    const next = workingResults.slice();
    next.splice(leftIndex, 2, mergedSeg);

    // Update meta: decrement segment_count for this paragraph
    const metaNext = workingMeta.map((m, i) =>
      i === leftSeg.paragraph_index ? { ...m, segment_count: m.segment_count - 1 } : { ...m }
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

  function reindex(results: SegmentResult[], meta: ParagraphMeta[]): SegmentResult[] {
    let globalIndex = 0;
    const reindexed: SegmentResult[] = [];
    meta.forEach((para, paraIdx) => {
      for (let i = 0; i < para.segment_count; i++) {
        const existing = results[globalIndex];
        if (existing) {
          reindexed.push({
            ...existing,
            index: globalIndex,
            paragraph_index: paraIdx,
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
      // Identify the affected paragraph
      const paragraphIdx = workingResults[
        [...pendingIndices][0]
      ]?.paragraph_index ?? null;

      if (paragraphIdx === null) {
        onSave(workingResults, workingMeta);
        return;
      }

      // Get ALL segments for this paragraph (not just pending ones)
      // This is required because the backend replaces all segments for the paragraph
      const paragraphSegmentIndices: number[] = [];
      const allSegmentTexts: string[] = [];

      workingResults.forEach((seg, idx) => {
        if (seg.paragraph_index === paragraphIdx) {
          paragraphSegmentIndices.push(idx);
          allSegmentTexts.push(seg.segment);
        }
      });

      const data = await translateBatch(
        allSegmentTexts,
        currentRawText || null,
        currentTranslationId,
        paragraphIdx,
      );

      // Apply translations to ALL segments in the paragraph
      const next = workingResults.map((r) => ({ ...r }));
      paragraphSegmentIndices.forEach((idx, i) => {
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
      console.error("Failed to translate segments:", error);
      saving = false;
    }
  }

  function cancel() {
    onCancel();
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === "Escape") {
      cancel();
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="edit-mode-active">
  <div id="segments-container">
    {#each workingParagraphs as para}
      <div
        class="paragraph flex flex-wrap gap-1 items-center"
        style={`margin-bottom: ${para.separator ? para.separator.split("\n").length * 0.4 : 0}rem; padding-left: ${para.indent ? para.indent.length * 0.5 : 0}rem;`}
      >
        {#each para.segments as segment, segIdx (segment.index)}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <span
            class={`segment editing inline-block rounded border-2 border-transparent ${pendingIndices.has(segment.index) ? "segment-pending" : ""}`}
            style={`font-family: var(--font-chinese); font-size: var(--text-chinese); color: var(--text-primary);`}
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

          {#if segIdx < para.segments.length - 1}
            <!-- svelte-ignore a11y_click_events_have_key_events -->
            <!-- svelte-ignore a11y_no_static_element_interactions -->
            <span
              class="join-indicator visible"
              title="Join segments"
              onclick={(e: MouseEvent) => {
                e.stopPropagation();
                handleJoin(segment.index, para.segments[segIdx + 1].index);
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
      <button class="btn-cancel" type="button" onclick={cancel}>Cancel</button>
      <button class="btn-save" type="button" onclick={save} disabled={saving}>
        {#if saving}
          Saving...
        {:else}
          Save Changes
        {/if}
      </button>
    </div>
  </div>
</div>
