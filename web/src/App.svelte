<script lang="ts">
  import { getJson, postJson, deleteRequest } from "./lib/api";
  import type {
    JobSummary,
    ListJobsResponse,
    ParagraphResult,
    JobDetailResponse,
    CreateJobResponse,
    ProgressState,
    ParagraphMeta,
    SegmentResult,
    DisplayParagraph,
    DueCountResponse,
    VocabSrsInfoListResponse,
    RecordLookupResponse,
    SaveVocabResponse,
    CreateTextResponse,
    StreamSegmentResult,
    StreamEvent,
    SavedVocabInfo,
    LoadingState,
  } from "./lib/types";
  import ReviewPanel from "./components/ReviewPanel.svelte";
  import TranslateForm from "./components/TranslateForm.svelte";
  import TranslationTable from "./components/TranslationTable.svelte";
  import JobQueue from "./components/JobQueue.svelte";
  import SegmentDisplay from "./components/SegmentDisplay.svelte";

  let jobQueue = $state<JobSummary[]>([]);
  let currentJobId = $state<string | null>(null);
  let isExpandedView = $state(false);
  let paragraphMeta = $state<ParagraphMeta[]>([]);
  let translationResults = $state<SegmentResult[]>([]);
  let fullTranslation = $state("");
  let progress = $state<ProgressState>({ current: 0, total: 0 });
  let loadingState = $state<LoadingState>("idle");
  let errorMessage = $state("");

  let currentTextId = $state<string | null>(null);
  let currentRawText = $state("");
  let savedVocabMap = $state<Map<string, SavedVocabInfo>>(new Map());

  let reviewPanelOpen = $state(false);
  let dueCount = $state(0);

  const displayParagraphs = $derived(buildDisplayParagraphs(paragraphMeta, translationResults));
  const queueCountLabel = $derived(`${jobQueue.length} job${jobQueue.length !== 1 ? "s" : ""}`);
  const otherJobsCount = $derived(jobQueue.filter((job) => job.id !== currentJobId).length);

  $effect(() => {
    void loadJobQueue();
    void updateDueCount();
  });

  function errorToMessage(error: unknown): string {
    if (error instanceof Error) return error.message;
    return String(error);
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
              pending: true
            };
        entry.index = globalIndex;
        entry.paragraph_index = paraIdx;
        globalIndex += 1;
        return entry;
      });
      return { ...para, paragraph_index: paraIdx, segments };
    });
  }

  async function loadJobQueue() {
    try {
      const data = await getJson<ListJobsResponse>("/api/jobs?limit=20");
      jobQueue = data.jobs || [];
    } catch (error) {
      console.error("Failed to load job queue:", error);
    }
  }

  async function handleSubmit(text: string) {
    if (!text) return;

    loadingState = "loading";
    try {
      const data = await postJson<CreateJobResponse>("/api/jobs", {
        input_text: text,
        source_type: "text"
      });
      currentRawText = text;
      currentTextId = null;
      await loadJobQueue();
      await expandJob(data.job_id);
    } catch (error) {
      errorMessage = `Failed to submit translation job: ${errorToMessage(error)}`;
      loadingState = "error";
    } finally {
      if (loadingState !== "error") {
        loadingState = "idle";
      }
    }
  }

  async function expandJob(jobId: string) {
    currentJobId = jobId;
    isExpandedView = true;
    loadingState = "loading";
    errorMessage = "";

    try {
      const job = await getJson<JobDetailResponse>(`/api/jobs/${jobId}`);
      currentRawText = job.input_text;
      currentTextId = null;

      if (job.status === "completed" && job.paragraphs) {
        applyCompletedJob(job);
      } else if (job.status === "processing" || job.status === "pending") {
        await streamJobProgress(jobId);
      } else if (job.status === "failed") {
        loadingState = "error";
        errorMessage = job.error_message || "Job failed";
      }

      await loadJobQueue();
    } catch (error) {
      loadingState = "error";
      errorMessage = "Failed to load job details";
    }
  }

  function applyCompletedJob(job: JobDetailResponse) {
    fullTranslation = job.full_translation || "";
    paragraphMeta = (job.paragraphs || []).map((para) => ({
      segment_count: para.translations.length,
      indent: para.indent,
      separator: para.separator
    }));
    translationResults = flattenParagraphs(job.paragraphs || []);
    progress = { current: translationResults.length, total: translationResults.length };
    loadingState = "idle";
    void fetchAndApplySrsInfo();
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
          pending: false
        });
      });
    });
    return results;
  }

  async function streamJobProgress(jobId: string) {
    translationResults = [];
    paragraphMeta = [];
    fullTranslation = "";
    progress = { current: 0, total: 0 };

    try {
      const response = await fetch(`/jobs/${jobId}/stream`);
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
                separator: para.separator
              }));
              translationResults = flattenParagraphs(data.paragraphs);
            }
            loadingState = "idle";
            await loadJobQueue();
            await fetchAndApplySrsInfo();
          } else if (data.type === "error") {
            loadingState = "error";
            errorMessage = data.message || "Streaming failed";
            await loadJobQueue();
          }
        }
      }
    } catch (error) {
      loadingState = "error";
      errorMessage = `Streaming failed: ${errorToMessage(error)}`;
      await loadJobQueue();
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
      pending: false
    };
    const next = translationResults.slice();
    next[index] = updated;
    translationResults = next;
  }

  function backToQueue() {
    isExpandedView = false;
    currentJobId = null;
    translationResults = [];
    paragraphMeta = [];
    fullTranslation = "";
    progress = { current: 0, total: 0 };
    loadingState = "idle";
    errorMessage = "";
    void loadJobQueue();
  }

  async function deleteJob(jobId: string) {
    if (!confirm("Delete this translation?")) return;

    try {
      await deleteRequest("/api/jobs/" + jobId);
      if (currentJobId === jobId) {
        backToQueue();
      } else {
        await loadJobQueue();
      }
    } catch (_error) {
      alert("Failed to delete job");
    }
  }

  async function fetchAndApplySrsInfo() {
    const headwords = [...new Set(translationResults.filter((s) => s.pinyin || s.english).map((s) => s.segment))];
    if (headwords.length === 0) return;

    try {
      const params = new URLSearchParams();
      params.set("headwords", headwords.join(","));
      const data = await getJson<VocabSrsInfoListResponse>(`/api/vocab/srs-info?${params.toString()}`);
      const nextMap = new Map<string, SavedVocabInfo>();
      data.items.forEach((info) => {
        const opacity = info.status === "known" ? 0 : info.opacity;
        nextMap.set(info.headword, {
          vocabItemId: info.vocab_item_id,
          opacity,
          isStruggling: info.is_struggling,
          status: info.status
        });
      });
      savedVocabMap = nextMap;
    } catch (error) {
      console.error("Failed to fetch SRS info:", error);
    }
  }

  async function updateDueCount() {
    try {
      const data = await getJson<DueCountResponse>("/api/review/count");
      dueCount = data.due_count || 0;
    } catch (error) {
      console.error("Failed to fetch due count:", error);
    }
  }

  function openReviewPanel() {
    reviewPanelOpen = true;
  }

  function closeReviewPanel() {
    reviewPanelOpen = false;
    void updateDueCount();
  }

  async function ensureSavedText() {
    if (currentTextId || !currentRawText) return currentTextId;

    const data = await postJson<CreateTextResponse>("/api/texts", {
      raw_text: currentRawText,
      source_type: "text",
      metadata: {}
    });
    currentTextId = data.id;
    return currentTextId;
  }

  async function onSaveVocab(headword: string, pinyin: string, english: string): Promise<SavedVocabInfo | null> {
    try {
      await ensureSavedText();
      const data = await postJson<SaveVocabResponse>("/api/vocab/save", {
        headword,
        pinyin,
        english,
        text_id: currentTextId,
        snippet: currentRawText,
        status: "learning"
      });
      const info: SavedVocabInfo = {
        vocabItemId: data.vocab_item_id,
        opacity: 1,
        isStruggling: false,
        status: "learning"
      };
      savedVocabMap = new Map(savedVocabMap.set(headword, info));
      await updateDueCount();
      return info;
    } catch (error) {
      console.error("Failed to save vocab:", error);
      return null;
    }
  }

  async function onMarkKnown(headword: string, vocabItemId: string) {
    try {
      await postJson("/api/vocab/status", {
        vocab_item_id: vocabItemId,
        status: "known"
      });
      const info = savedVocabMap.get(headword);
      if (info) {
        savedVocabMap = new Map(savedVocabMap.set(headword, { ...info, status: "known", opacity: 0 }));
      }
      await updateDueCount();
    } catch (error) {
      console.error("Failed to mark known:", error);
    }
  }

  async function onResumeLearning(headword: string, vocabItemId: string) {
    try {
      await postJson("/api/vocab/status", {
        vocab_item_id: vocabItemId,
        status: "learning"
      });
      const info = savedVocabMap.get(headword);
      if (info) {
        savedVocabMap = new Map(savedVocabMap.set(headword, { ...info, status: "learning", opacity: 1 }));
      }
      await updateDueCount();
    } catch (error) {
      console.error("Failed to resume learning:", error);
    }
  }

  async function onRecordLookup(headword: string, vocabItemId: string) {
    try {
      const data = await postJson<RecordLookupResponse>("/api/vocab/lookup", { vocab_item_id: vocabItemId });
      const info = savedVocabMap.get(headword);
      if (info) {
        savedVocabMap = new Map(savedVocabMap.set(headword, {
          ...info,
          opacity: data.opacity,
          isStruggling: data.is_struggling
        }));
      }
    } catch (error) {
      console.error("Failed to record lookup:", error);
    }
  }
</script>

<svelte:head>
  <script src="https://cdn.tailwindcss.com"></script>
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous" />
  <link
    href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&family=JetBrains+Mono:wght@400;500&family=Noto+Sans+SC:wght@400;500;600&display=swap"
    rel="stylesheet"
  />
  <link rel="stylesheet" href="/css/variables.css" />
  <link rel="stylesheet" href="/css/base.css" />
  <link rel="stylesheet" href="/css/segments.css" />
</svelte:head>

<div class="container mx-auto px-4 py-8 max-w-6xl">
  <header class="mb-8 flex items-start justify-between">
    <div>
      <h1 class="font-chinese font-semibold mb-1" style="color: var(--text-primary); font-size: var(--text-2xl);">
        Language App
      </h1>
      <p style="color: var(--text-secondary); font-size: var(--text-base);">
        Enter Chinese text to get word segmentation, pinyin, and English translation
      </p>
    </div>
    <div class="flex items-center gap-3">
      <a href="/translations" class="btn-secondary px-3 py-1.5 text-sm">Translations</a>
      <a href="/admin" class="btn-secondary px-3 py-1.5 text-sm">Admin</a>
      <button id="review-btn" class="btn-secondary px-3 py-1.5 flex items-center" onclick={openReviewPanel}>
        Review
        {#if dueCount > 0}
          <span class="review-badge">{dueCount}</span>
        {/if}
      </button>
    </div>
  </header>

  <ReviewPanel open={reviewPanelOpen} onClose={closeReviewPanel} onDueCountChange={(c) => dueCount = c} />

  <main class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
    <div class="space-y-4">
      <TranslateForm onSubmit={handleSubmit} loading={loadingState === "loading"} />

      <div id="translation-table">
        <TranslationTable results={translationResults} />
      </div>
    </div>

    <div class="space-y-4">
      {#if !isExpandedView}
        <div id="job-queue-panel" class="input-card p-5">
          <div class="flex items-center justify-between mb-4">
            <h2 class="font-semibold" style="color: var(--text-primary); font-size: var(--text-base);">Translation Queue</h2>
            <span class="text-xs px-2 py-0.5 rounded-full" style="background: var(--pastel-3); color: var(--text-primary);">{queueCountLabel}</span>
          </div>
          <JobQueue jobs={jobQueue} onExpand={expandJob} onDelete={deleteJob} />
        </div>
      {:else}
        <div id="expanded-job-view">
          <button class="mb-3 flex items-center gap-1 hover:underline" style="color: var(--primary); font-size: var(--text-sm);" onclick={backToQueue}>
            <span>&larr;</span> Back to Queue
          </button>

          <!-- svelte-ignore a11y_label_has_associated_control -->
          <div id="original-text-panel" class="input-card p-4 mb-4">
            <label class="block font-medium mb-1.5" style="color: var(--text-secondary); font-size: var(--text-xs);">Original Text</label>
            <div class="font-chinese p-2 rounded" style="background: var(--pastel-7); color: var(--text-primary); font-size: var(--text-chinese); min-height: 60px; white-space: pre-wrap;">{currentRawText}</div>
          </div>

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

          {#if otherJobsCount > 0}
            <div class="mt-4">
              <button class="w-full input-card p-3 text-left hover:shadow-md transition-shadow" onclick={backToQueue}>
                <div class="flex items-center justify-between">
                  <span style="color: var(--text-secondary); font-size: var(--text-sm);">Queue</span>
                  <span class="text-xs px-2 py-0.5 rounded-full" style="background: var(--pastel-4); color: var(--text-primary);">{otherJobsCount} more</span>
                </div>
              </button>
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </main>
</div>
