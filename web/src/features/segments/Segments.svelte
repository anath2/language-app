<script lang="ts">
  import SegmentDisplay from "./SegmentDisplay.svelte";
  import SegmentEditor from "./SegmentEditor.svelte";
  import TranslationTable from "./TranslationTable.svelte";
  import type {
    ParagraphResult,
    ParagraphMeta,
    SegmentResult,
    DisplayParagraph,
    ProgressState,
    SavedVocabInfo,
    StreamSegmentResult,
    StreamEvent,
  } from "./types";
  import type { LoadingState } from "../../lib/types";

  let {
    jobId,
    jobStatus,
    jobParagraphs,
    jobFullTranslation,
    rawText,
    savedVocabMap,
    onSaveVocab,
    onMarkKnown,
    onResumeLearning,
    onRecordLookup,
    onStreamComplete,
    onSegmentsChanged,
  }: {
    jobId: string | null;
    jobStatus: string | null;
    jobParagraphs: ParagraphResult[] | null;
    jobFullTranslation: string | null;
    rawText: string;
    savedVocabMap: Map<string, SavedVocabInfo>;
    onSaveVocab: (headword: string, pinyin: string, english: string) => Promise<SavedVocabInfo | null>;
    onMarkKnown: (headword: string, vocabItemId: string) => Promise<void>;
    onResumeLearning: (headword: string, vocabItemId: string) => Promise<void>;
    onRecordLookup: (headword: string, vocabItemId: string) => Promise<void>;
    onStreamComplete: () => void;
    onSegmentsChanged: (results: SegmentResult[]) => void;
  } = $props();

  let translationResults = $state<SegmentResult[]>([]);
  let paragraphMeta = $state<ParagraphMeta[]>([]);
  let fullTranslation = $state("");
  let progress = $state<ProgressState>({ current: 0, total: 0 });
  let loadingState = $state<LoadingState>("idle");
  let errorMessage = $state("");
  let isEditMode = $state(false);

  const displayParagraphs = $derived(buildDisplayParagraphs(paragraphMeta, translationResults));

  // Track jobId changes and react
  let lastJobId = $state<string | null>(null);

  $effect(() => {
    const currentJobId = jobId;
    const currentJobStatus = jobStatus;
    const currentJobParagraphs = jobParagraphs;
    const currentJobFullTranslation = jobFullTranslation;

    if (currentJobId !== lastJobId) {
      lastJobId = currentJobId;
      isEditMode = false;

      if (!currentJobId) {
        resetState();
        return;
      }

      if (currentJobStatus === "completed" && currentJobParagraphs) {
        applyCompletedJob(currentJobParagraphs, currentJobFullTranslation);
      } else if (currentJobStatus === "processing" || currentJobStatus === "pending") {
        void streamJobProgress(currentJobId);
      } else if (currentJobStatus === "failed") {
        loadingState = "error";
        errorMessage = "Job failed";
      } else {
        loadingState = "loading";
      }
    }
  });

  function resetState() {
    translationResults = [];
    paragraphMeta = [];
    fullTranslation = "";
    progress = { current: 0, total: 0 };
    loadingState = "idle";
    errorMessage = "";
    isEditMode = false;
  }

  function buildDisplayParagraphs(meta: ParagraphMeta[], results: SegmentResult[]): DisplayParagraph[] {
    let globalIndex = 0;
    return meta.map((para, paraIdx) => {
      const segments = Array.from({ length: para.segment_count }).map(() => {
        const existing = results[globalIndex];
        const entry = existing
          ? { ...existing }
          : {
              segment: "Loading...",
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

  function applyCompletedJob(paragraphs: ParagraphResult[], fullTrans: string | null) {
    fullTranslation = fullTrans || "";
    paragraphMeta = paragraphs.map((para) => ({
      segment_count: para.translations.length,
      indent: para.indent,
      separator: para.separator,
    }));
    translationResults = flattenParagraphs(paragraphs);
    progress = { current: translationResults.length, total: translationResults.length };
    loadingState = "idle";
    onSegmentsChanged(translationResults);
  }

  function flattenParagraphs(paragraphs: ParagraphResult[]): SegmentResult[] {
    const results: SegmentResult[] = [];
    paragraphs.forEach((para, paraIdx) => {
      para.translations.forEach((t) => {
        results.push({
          segment: t.segment,
          pinyin: t.pinyin,
          english: t.english,
          index: results.length,
          paragraph_index: paraIdx,
          pending: false,
        });
      });
    });
    return results;
  }

  function errorToMessage(error: unknown): string {
    if (error instanceof Error) return error.message;
    return String(error);
  }

  async function streamJobProgress(streamJobId: string) {
    translationResults = [];
    paragraphMeta = [];
    fullTranslation = "";
    progress = { current: 0, total: 0 };
    loadingState = "loading";

    try {
      const response = await fetch(`/jobs/${streamJobId}/stream`);
      if (!response.body) {
        throw new Error("Streaming unavailable");
      }
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";

      while (true) {
        const { value, done } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() || "";

        for (const line of lines) {
          if (!line.startsWith("data: ")) continue;
          const data = JSON.parse(line.slice(6)) as StreamEvent;
          if (data.type === "start") {
            paragraphMeta = data.paragraphs || [];
            if (paragraphMeta.length === 0 && data.total) {
              paragraphMeta = [{ segment_count: data.total, indent: "", separator: "" }];
            }
            progress = { current: 0, total: data.total || 0 };
            fullTranslation = data.fullTranslation || "";
            loadingState = "idle";
          } else if (data.type === "progress") {
            progress = { current: data.current, total: data.total };
            updateSegmentResult(data.result);
          } else if (data.type === "complete") {
            fullTranslation = data.fullTranslation || fullTranslation;
            if (data.paragraphs) {
              paragraphMeta = data.paragraphs.map((para) => ({
                segment_count: para.translations.length,
                indent: para.indent,
                separator: para.separator,
              }));
              translationResults = flattenParagraphs(data.paragraphs);
            }
            loadingState = "idle";
            onStreamComplete();
            onSegmentsChanged(translationResults);
          } else if (data.type === "error") {
            loadingState = "error";
            errorMessage = data.message || "Streaming failed";
            onStreamComplete();
          }
        }
      }
    } catch (error) {
      loadingState = "error";
      errorMessage = `Streaming failed: ${errorToMessage(error)}`;
      onStreamComplete();
    }
  }

  function updateSegmentResult(result: StreamSegmentResult) {
    const index = result.index;
    const updated: SegmentResult = {
      segment: result.segment,
      pinyin: result.pinyin,
      english: result.english,
      index,
      paragraph_index: result.paragraph_index,
      pending: false,
    };
    const next = translationResults.slice();
    next[index] = updated;
    translationResults = next;
  }

  function enterEditMode() {
    isEditMode = true;
  }

  function handleEditSave(results: SegmentResult[], meta: ParagraphMeta[]) {
    translationResults = results;
    paragraphMeta = meta;
    isEditMode = false;
    onSegmentsChanged(translationResults);
  }

  function handleEditCancel() {
    isEditMode = false;
  }
</script>

<div id="results" class="input-card p-5 overflow-y-auto" style="max-height: 60vh;">
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
    <div class="space-y-1">
      <h2 class="font-semibold" style="color: var(--text-primary); font-size: var(--text-lg);">Translation</h2>
      <p class="full-translation">{fullTranslation || "Translating..."}</p>
    </div>

    <div class="section-divider my-3">
      <div class="flex items-center justify-between w-full">
        <span>Segmented Text</span>
        {#if !isEditMode && progress.current >= progress.total && translationResults.length > 0}
          <button class="btn-edit" type="button" onclick={enterEditMode}>
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path>
              <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path>
            </svg>
            Edit Segments
          </button>
        {/if}
      </div>
    </div>

    {#if isEditMode}
      <SegmentEditor
        {displayParagraphs}
        {translationResults}
        {paragraphMeta}
        currentJobId={jobId}
        currentRawText={rawText}
        onSave={handleEditSave}
        onCancel={handleEditCancel}
      />
    {:else}
      <SegmentDisplay
        {displayParagraphs}
        {savedVocabMap}
        {progress}
        {fullTranslation}
        {loadingState}
        {errorMessage}
        {onSaveVocab}
        {onMarkKnown}
        {onResumeLearning}
        {onRecordLookup}
      />
    {/if}
  {/if}
</div>

<div id="translation-table">
  <TranslationTable results={translationResults} />
</div>
