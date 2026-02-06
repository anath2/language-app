<script lang="ts">
  import { tick } from "svelte";
  import { getPastelColor } from "../lib/utils";
  import type {
    DisplayParagraph,
    LoadingState,
    ProgressState,
    SavedVocabInfo,
    SegmentResult,
    TooltipState,
  } from "../lib/types";

  let {
    displayParagraphs,
    savedVocabMap,
    progress,
    fullTranslation,
    loadingState,
    errorMessage,
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
    onSaveVocab: (headword: string, pinyin: string, english: string) => Promise<SavedVocabInfo | null>;
    onMarkKnown: (headword: string, vocabItemId: string) => Promise<void>;
    onResumeLearning: (headword: string, vocabItemId: string) => Promise<void>;
    onRecordLookup: (headword: string, vocabItemId: string) => Promise<void>;
  } = $props();

  let tooltipVisible = $state(false);
  let tooltipPinned = $state(false);
  let tooltip = $state<TooltipState>({
    headword: "",
    pinyin: "",
    english: "",
    vocabItemId: null,
    status: "",
    x: 0,
    y: 0,
  });

  let resultsContainer = $state<HTMLDivElement | null>(null);
  let tooltipRef = $state<HTMLDivElement | null>(null);

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
    return styles.join("; ");
  }

  function getSegmentClasses(segment: SegmentResult) {
    const classes = ["segment", "inline-block", "px-2", "py-1", "rounded", "border-2", "border-transparent"];
    if (segment.pending) classes.push("segment-pending");
    if (segment.pinyin || segment.english) {
      classes.push("transition-all", "duration-150", "hover:-translate-y-px", "hover:shadow-sm");
    }
    const info = savedVocabMap.get(segment.segment);
    if (info) {
      classes.push("saved");
      if (info.isStruggling) classes.push("struggling");
    }
    return classes.join(" ");
  }

  async function handleSegmentHover(segment: SegmentResult, element: EventTarget | null) {
    if (tooltipPinned) return;
    await showTooltip(segment, element, false);
  }

  function handleSegmentLeave() {
    if (!tooltipPinned) {
      tooltipVisible = false;
    }
  }

  async function toggleSegmentPin(segment: SegmentResult, element: EventTarget | null) {
    if (tooltipPinned && tooltip.headword === segment.segment) {
      tooltipPinned = false;
      tooltipVisible = false;
      return;
    }
    tooltipPinned = true;
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
      pinyin: segment.pinyin || "",
      english: segment.english || "",
      vocabItemId: info?.vocabItemId || null,
      status: info?.status || "",
      x: 0,
      y: 0,
    };
    tooltipVisible = true;
    tooltipPinned = pinned;
    await tick();

    if (!resultsContainer || !tooltipRef || !isHTMLElement(element)) return;
    const segRect = element.getBoundingClientRect();
    const containerRect = resultsContainer.getBoundingClientRect();
    const tooltipRect = tooltipRef.getBoundingClientRect();
    const left = segRect.left - containerRect.left + segRect.width / 2 - tooltipRect.width / 2;
    const top = segRect.top - containerRect.top - tooltipRect.height - 8;
    tooltip = { ...tooltip, x: Math.max(0, left), y: Math.max(0, top) };
  }

  function handleGlobalClick(event: MouseEvent) {
    if (!tooltipPinned) return;
    const target = event.target as HTMLElement | null;
    if (tooltipRef?.contains(target)) return;
    if (target?.closest?.(".segment")) return;
    tooltipPinned = false;
    tooltipVisible = false;
  }

  async function saveVocab() {
    if (!tooltip.headword) return;
    const info = await onSaveVocab(tooltip.headword, tooltip.pinyin, tooltip.english);
    if (info) {
      tooltip = { ...tooltip, vocabItemId: info.vocabItemId, status: "learning" };
    }
  }

  async function markKnown() {
    if (!tooltip.vocabItemId) return;
    await onMarkKnown(tooltip.headword, tooltip.vocabItemId);
    tooltip = { ...tooltip, status: "known" };
  }

  async function resumeLearning() {
    if (!tooltip.vocabItemId) return;
    await onResumeLearning(tooltip.headword, tooltip.vocabItemId);
    tooltip = { ...tooltip, status: "learning" };
  }
</script>

<svelte:window onclick={handleGlobalClick} />

<div id="results" class="input-card p-5 overflow-y-auto" style="max-height: 60vh;" bind:this={resultsContainer}>
  {#if loadingState === "loading"}
    <div class="h-full flex items-center justify-center">
      <div class="text-center">
        <div class="spinner mx-auto mb-2" style="width: 20px; height: 20px; border-color: rgba(124, 158, 178, 0.3); border-top-color: var(--primary);"></div>
        <p style="color: var(--text-muted); font-size: var(--text-sm);">Starting translation...</p>
      </div>
    </div>
  {:else if loadingState === "error"}
    <div class="p-3 rounded-md" style="background: var(--error); border-left: 3px solid var(--secondary-dark);">
      <p style="color: var(--text-primary); font-size: var(--text-sm);">{errorMessage}</p>
    </div>
  {:else if displayParagraphs.length === 0}
    <div class="h-full flex items-center justify-center">
      <p class="text-center italic" style="color: var(--text-muted); font-size: var(--text-sm);">Translation results will appear here</p>
    </div>
  {:else}
    <div class="relative space-y-3">
      <div class="space-y-1">
        <h2 class="font-semibold" style="color: var(--text-primary); font-size: var(--text-lg);">Translation</h2>
        <p class="full-translation">{fullTranslation || "Translating..."}</p>
      </div>

      <div class="section-divider my-3">
        <span>Segmented Text</span>
      </div>

      {#if progress.total > 0 && progress.current < progress.total}
        <div class="progress-container">
          <div class="flex justify-between mb-1.5" style="font-size: var(--text-xs);">
            <span style="color: var(--text-secondary);">Translating...</span>
            <span style="color: var(--text-secondary);">{progress.current} / {progress.total}</span>
          </div>
          <div class="progress-bar-bg">
            <div class="progress-bar-fill" style={`width: ${(progress.current / progress.total) * 100}%`}></div>
          </div>
        </div>
      {/if}

      <div id="segments-container">
        {#each displayParagraphs as para}
          <div class="paragraph flex flex-wrap gap-1" style={`margin-bottom: ${para.separator ? para.separator.split("\n").length * 0.4 : 0}rem; padding-left: ${para.indent ? para.indent.length * 0.5 : 0}rem;`}>
            {#each para.segments as segment (segment.index)}
              <!-- svelte-ignore a11y_click_events_have_key_events -->
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <span
                class={getSegmentClasses(segment)}
                style={`font-family: var(--font-chinese); font-size: var(--text-chinese); color: var(--text-primary); cursor: ${segment.pinyin || segment.english ? "pointer" : "default"}; ${getSegmentStyle(segment)}`}
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

      <div
        class={`word-tooltip ${tooltipVisible ? "" : "hidden"}`}
        bind:this={tooltipRef}
        style={`left: ${tooltip.x}px; top: ${tooltip.y}px;`}
      >
        <div class="tooltip-pinyin">{tooltip.pinyin}</div>
        <div class="tooltip-english">{tooltip.english}</div>
        <div class="tooltip-actions">
          {#if tooltip.pinyin || tooltip.english}
            {#if !tooltip.vocabItemId}
              <button class="tooltip-btn" type="button" onclick={saveVocab}>Save to Learn</button>
            {:else if tooltip.status === "learning"}
              <button class="tooltip-btn" type="button" onclick={markKnown}>Mark as Known</button>
            {:else if tooltip.status === "known"}
              <button class="tooltip-btn" type="button" onclick={resumeLearning}>Resume Learning</button>
            {:else}
              <button class="tooltip-btn" type="button" onclick={saveVocab}>Save to Learn</button>
            {/if}
          {/if}
        </div>
        <div class="tooltip-arrow"></div>
      </div>
    </div>
  {/if}
</div>
