<script>
  import { onMount, tick } from "svelte";
  import { getJson, postJson, deleteRequest } from "./lib/api";
  import { formatTimeAgo, getPastelColor } from "./lib/utils";

  let textInput = "";
  let jobQueue = [];
  let currentJobId = null;
  let isExpandedView = false;
  let paragraphMeta = [];
  let translationResults = [];
  let fullTranslation = "";
  let progress = { current: 0, total: 0 };
  let loadingState = "idle";
  let errorMessage = "";
  let showDetails = false;

  let currentTextId = null;
  let currentRawText = "";
  let savedVocabMap = new Map();

  let reviewPanelOpen = false;
  let reviewQueue = [];
  let reviewIndex = 0;
  let reviewAnswered = false;
  let dueCount = 0;

  let tooltipVisible = false;
  let tooltipPinned = false;
  let tooltip = {
    headword: "",
    pinyin: "",
    english: "",
    vocabItemId: null,
    status: "",
    x: 0,
    y: 0
  };

  let resultsContainer;
  let tooltipRef;

  let ocrFile = null;
  let ocrPreviewUrl = "";
  let ocrFileName = "";
  let ocrLoading = false;

  const statusLabels = {
    pending: "Pending",
    processing: "Processing",
    completed: "Completed",
    failed: "Failed"
  };

  $: displayParagraphs = buildDisplayParagraphs(paragraphMeta, translationResults);
  $: queueCountLabel = `${jobQueue.length} job${jobQueue.length !== 1 ? "s" : ""}`;
  $: otherJobsCount = jobQueue.filter((job) => job.id !== currentJobId).length;

  onMount(() => {
    loadJobQueue();
    updateDueCount();
  });

  function buildDisplayParagraphs(meta, results) {
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
      const data = await getJson("/api/jobs?limit=20");
      jobQueue = data.jobs || [];
    } catch (error) {
      console.error("Failed to load job queue:", error);
    }
  }

  async function submitJob() {
    const text = textInput.trim();
    if (!text) return;

    loadingState = "loading";
    try {
      const data = await postJson("/api/jobs", {
        input_text: text,
        source_type: "text"
      });
      currentRawText = text;
      currentTextId = null;
      textInput = "";
      await loadJobQueue();
      await expandJob(data.job_id);
    } catch (error) {
      errorMessage = `Failed to submit translation job: ${error.message}`;
      loadingState = "error";
    } finally {
      if (loadingState !== "error") {
        loadingState = "idle";
      }
    }
  }

  async function expandJob(jobId) {
    currentJobId = jobId;
    isExpandedView = true;
    loadingState = "loading";
    errorMessage = "";

    try {
      const job = await getJson(`/api/jobs/${jobId}`);
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

  function applyCompletedJob(job) {
    fullTranslation = job.full_translation || "";
    paragraphMeta = (job.paragraphs || []).map((para) => ({
      segment_count: para.translations.length,
      indent: para.indent,
      separator: para.separator
    }));
    translationResults = flattenParagraphs(job.paragraphs || []);
    progress = { current: translationResults.length, total: translationResults.length };
    loadingState = "idle";
    fetchAndApplySrsInfo();
  }

  function flattenParagraphs(paragraphs) {
    const results = [];
    paragraphs.forEach((para, paraIdx) => {
      para.translations.forEach((t, idx) => {
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

  async function streamJobProgress(jobId) {
    translationResults = [];
    paragraphMeta = [];
    fullTranslation = "";
    progress = { current: 0, total: 0 };

    try {
      const response = await fetch(`/jobs/${jobId}/stream`);
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
          const data = JSON.parse(line.slice(6));
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
      errorMessage = `Streaming failed: ${error.message}`;
      await loadJobQueue();
    }
  }

  function updateSegmentResult(result) {
    const index = result.index;
    const updated = {
      segment: result.segment,
      pinyin: result.pinyin,
      english: result.english,
      index,
      paragraph_index: result.paragraph_index,
      pending: false
    };
    translationResults = replaceIndex(translationResults, index, updated);
  }

  function replaceIndex(list, index, item) {
    const next = list.slice();
    next[index] = item;
    return next;
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
    loadJobQueue();
  }

  async function deleteJob(jobId) {
    if (!confirm("Delete this translation?")) return;

    try {
      await deleteRequest(`/api/jobs/${jobId}`);
      if (currentJobId === jobId) {
        backToQueue();
      } else {
        await loadJobQueue();
      }
    } catch (error) {
      alert("Failed to delete job");
    }
  }

  function getSegmentStyle(segment) {
    const info = savedVocabMap.get(segment.segment);
    const baseColor = getPastelColor(segment.index || 0);
    const styles = [];
    if (info) {
      styles.push(`--segment-color: ${baseColor}`);
      styles.push(`--segment-opacity: ${info.opacity}`);
    } else if (!segment.pending && segment.pinyin) {
      styles.push(`background: ${baseColor}`);
    }
    return styles.join("; ");
  }

  function getSegmentClasses(segment) {
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

  async function fetchAndApplySrsInfo() {
    const headwords = [...new Set(translationResults.filter((s) => s.pinyin || s.english).map((s) => s.segment))];
    if (headwords.length === 0) return;

    try {
      const params = new URLSearchParams();
      params.set("headwords", headwords.join(","));
      const data = await getJson(`/api/vocab/srs-info?${params.toString()}`);
      const nextMap = new Map();
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
      const data = await getJson("/api/review/count");
      dueCount = data.due_count || 0;
    } catch (error) {
      console.error("Failed to fetch due count:", error);
    }
  }

  async function openReviewPanel() {
    reviewPanelOpen = true;
    await loadReviewQueue();
  }

  function closeReviewPanel() {
    reviewPanelOpen = false;
    updateDueCount();
  }

  async function loadReviewQueue() {
    try {
      const data = await getJson("/api/review/queue?limit=20");
      reviewQueue = data.cards || [];
      reviewIndex = 0;
      reviewAnswered = false;
      dueCount = data.due_count || 0;
    } catch (error) {
      console.error("Failed to load review queue:", error);
      reviewQueue = [];
    }
  }

  function revealAnswer() {
    reviewAnswered = true;
  }

  async function gradeCard(grade) {
    if (!reviewQueue[reviewIndex]) return;

    try {
      await postJson("/api/review/answer", {
        vocab_item_id: reviewQueue[reviewIndex].vocab_item_id,
        grade
      });
    } catch (error) {
      console.error("Failed to record grade:", error);
    }

    reviewIndex += 1;
    reviewAnswered = false;

    if (reviewIndex >= reviewQueue.length) {
      await loadReviewQueue();
    }
  }

  async function ensureSavedText() {
    if (currentTextId || !currentRawText) return currentTextId;

    const data = await postJson("/api/texts", {
      raw_text: currentRawText,
      source_type: "text",
      metadata: {}
    });
    currentTextId = data.id;
    return currentTextId;
  }

  async function handleSegmentHover(segment, element) {
    if (tooltipPinned) return;
    await showTooltip(segment, element, false);
  }

  function handleSegmentLeave() {
    if (!tooltipPinned) {
      tooltipVisible = false;
    }
  }

  async function toggleSegmentPin(segment, element) {
    if (tooltipPinned && tooltip.headword === segment.segment) {
      tooltipPinned = false;
      tooltipVisible = false;
      return;
    }
    tooltipPinned = true;
    await showTooltip(segment, element, true);

    const info = savedVocabMap.get(segment.segment);
    if (info?.vocabItemId) {
      try {
        const data = await postJson("/api/vocab/lookup", { vocab_item_id: info.vocabItemId });
        savedVocabMap.set(segment.segment, {
          ...info,
          opacity: data.opacity,
          isStruggling: data.is_struggling
        });
      } catch (error) {
        console.error("Failed to record lookup:", error);
      }
    }
  }

  async function showTooltip(segment, element, pinned) {
    const info = savedVocabMap.get(segment.segment);
    tooltip = {
      headword: segment.segment,
      pinyin: segment.pinyin || "",
      english: segment.english || "",
      vocabItemId: info?.vocabItemId || null,
      status: info?.status || "",
      x: 0,
      y: 0
    };
    tooltipVisible = true;
    tooltipPinned = pinned;
    await tick();

    if (!resultsContainer || !tooltipRef || !element) return;
    const segRect = element.getBoundingClientRect();
    const containerRect = resultsContainer.getBoundingClientRect();
    const tooltipRect = tooltipRef.getBoundingClientRect();
    const left = segRect.left - containerRect.left + segRect.width / 2 - tooltipRect.width / 2;
    const top = segRect.top - containerRect.top - tooltipRect.height - 8;
    tooltip = { ...tooltip, x: Math.max(0, left), y: Math.max(0, top) };
  }

  function handleGlobalClick(event) {
    if (!tooltipPinned) return;
    if (tooltipRef?.contains(event.target)) return;
    if (event.target.closest?.(".segment")) return;
    tooltipPinned = false;
    tooltipVisible = false;
  }

  async function saveVocab() {
    if (!tooltip.headword) return;
    try {
      await ensureSavedText();
      const data = await postJson("/api/vocab/save", {
        headword: tooltip.headword,
        pinyin: tooltip.pinyin,
        english: tooltip.english,
        text_id: currentTextId,
        snippet: currentRawText,
        status: "learning"
      });
      const info = {
        vocabItemId: data.vocab_item_id,
        opacity: 1,
        isStruggling: false,
        status: "learning"
      };
      savedVocabMap = new Map(savedVocabMap.set(tooltip.headword, info));
      tooltip = { ...tooltip, vocabItemId: data.vocab_item_id, status: "learning" };
      await updateDueCount();
    } catch (error) {
      console.error("Failed to save vocab:", error);
    }
  }

  async function markKnown() {
    if (!tooltip.vocabItemId) return;
    try {
      await postJson("/api/vocab/status", {
        vocab_item_id: tooltip.vocabItemId,
        status: "known"
      });
      const info = savedVocabMap.get(tooltip.headword);
      if (info) {
        savedVocabMap = new Map(savedVocabMap.set(tooltip.headword, { ...info, status: "known", opacity: 0 }));
      }
      tooltip = { ...tooltip, status: "known" };
      await updateDueCount();
    } catch (error) {
      console.error("Failed to mark known:", error);
    }
  }

  async function resumeLearning() {
    if (!tooltip.vocabItemId) return;
    try {
      await postJson("/api/vocab/status", {
        vocab_item_id: tooltip.vocabItemId,
        status: "learning"
      });
      const info = savedVocabMap.get(tooltip.headword);
      if (info) {
        savedVocabMap = new Map(savedVocabMap.set(tooltip.headword, { ...info, status: "learning", opacity: 1 }));
      }
      tooltip = { ...tooltip, status: "learning" };
      await updateDueCount();
    } catch (error) {
      console.error("Failed to resume learning:", error);
    }
  }

  function handleFileChange(event) {
    const [file] = event.target.files || [];
    if (!file) return;
    ocrFile = file;
    ocrFileName = file.name;
    ocrPreviewUrl = URL.createObjectURL(file);
  }

  function clearPreview() {
    ocrFile = null;
    ocrFileName = "";
    if (ocrPreviewUrl) {
      URL.revokeObjectURL(ocrPreviewUrl);
    }
    ocrPreviewUrl = "";
  }

  async function extractTextFromImage() {
    if (!ocrFile) return;
    ocrLoading = true;
    try {
      const formData = new FormData();
      formData.append("file", ocrFile);
      const res = await fetch("/extract-text", {
        method: "POST",
        body: formData
      });
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data?.detail || "OCR failed");
      }
      const data = await res.json();
      textInput = data.text || "";
      clearPreview();
    } catch (error) {
      console.error("OCR failed:", error);
    } finally {
      ocrLoading = false;
    }
  }
</script>

<svelte:head>
  <script src="https://cdn.tailwindcss.com"></script>
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
  <link
    href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&family=JetBrains+Mono:wght@400;500&family=Noto+Sans+SC:wght@400;500;600&display=swap"
    rel="stylesheet"
  />
  <link rel="stylesheet" href="/static/css/variables.css" />
  <link rel="stylesheet" href="/static/css/base.css" />
  <link rel="stylesheet" href="/static/css/segments.css" />
</svelte:head>

<svelte:window on:click={handleGlobalClick} />

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
      <button id="review-btn" class="btn-secondary px-3 py-1.5 flex items-center" on:click={openReviewPanel}>
        Review
        {#if dueCount > 0}
          <span class="review-badge">{dueCount}</span>
        {/if}
      </button>
    </div>
  </header>

  {#if reviewPanelOpen}
    <div class="panel-overlay visible" on:click={closeReviewPanel}></div>
  {:else}
    <div class="panel-overlay" on:click={closeReviewPanel}></div>
  {/if}

  <div class={`review-panel ${reviewPanelOpen ? "open" : ""}`}>
    <div class="review-panel-header">
      <h2 class="font-semibold" style="color: var(--text-primary); font-size: var(--text-lg);">Review</h2>
      <button on:click={closeReviewPanel} style="color: var(--text-muted); font-size: var(--text-xl);">&times;</button>
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
              <button class="reveal-btn" on:click={revealAnswer}>Show Answer</button>
            {/if}
            <div class={`answer-section ${reviewAnswered ? "" : "hidden"}`}>
              <div class="pinyin">{reviewQueue[reviewIndex].pinyin}</div>
              <div class="english">{reviewQueue[reviewIndex].english}</div>
              {#if reviewQueue[reviewIndex].snippets?.length}
                <div class="snippet">"{reviewQueue[reviewIndex].snippets[0]}"</div>
              {/if}
              <div class="grade-buttons">
                <button class="grade-btn again" on:click={() => gradeCard(0)}>Again</button>
                <button class="grade-btn hard" on:click={() => gradeCard(1)}>Hard</button>
                <button class="grade-btn good" on:click={() => gradeCard(2)}>Good</button>
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

  <main class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
    <div class="space-y-4">
      <div class="input-card p-5 h-[100%]">
        <form on:submit|preventDefault={submitJob}>
          <div class="mb-4">
            <label for="text" class="block font-medium mb-1.5" style="color: var(--text-primary); font-size: var(--text-sm);">
              Chinese Text
            </label>
            <textarea
              id="text"
              name="text"
              rows="5"
              placeholder="Enter Chinese text here, e.g., 你好世界"
              required
              class="textarea-main w-full px-3 py-2.5 resize-y"
              bind:value={textInput}
            ></textarea>
          </div>
          <button type="submit" class="btn-primary inline-flex items-center justify-center gap-1.5 px-4 py-2 min-w-[110px]" disabled={loadingState === "loading"}>
            {#if loadingState === "loading"}
              <span class="spinner"></span>
              Translating...
            {:else}
              Translate
            {/if}
          </button>
        </form>

        <div class="section-divider my-5">
          <span>or extract from image</span>
        </div>

        <div id="drop-zone" class="image-upload-zone p-4 text-center cursor-pointer">
          <input
            type="file"
            id="image-input"
            name="file"
            accept=".png,.jpg,.jpeg,.webp,.gif"
            class="hidden"
            on:change={handleFileChange}
          />
          {#if !ocrPreviewUrl}
            <div>
              <svg class="mx-auto h-8 w-8 mb-2" style="color: var(--text-muted);" stroke="currentColor" fill="none" viewBox="0 0 48 48">
                <path d="M28 8H12a4 4 0 00-4 4v20m32-12v8m0 0v8a4 4 0 01-4 4H12a4 4 0 01-4-4v-4m32-4l-3.172-3.172a4 4 0 00-5.656 0L28 28M8 32l9.172-9.172a4 4 0 015.656 0L28 28m0 0l4 4m4-24h8m-4-4v8m-12 4h.02" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
              <p class="font-medium" style="color: var(--text-secondary); font-size: var(--text-sm);">
                Drop an image here or click to upload
              </p>
              <p class="mt-0.5" style="color: var(--text-muted); font-size: var(--text-xs);">
                PNG, JPG, JPEG, WebP, GIF (max 5MB)
              </p>
              <button class="btn-secondary mt-3 px-3 py-1.5 inline-flex items-center gap-1.5" type="button" on:click={() => document.getElementById("image-input").click()}>
                Choose File
              </button>
            </div>
          {:else}
            <div>
              <img src={ocrPreviewUrl} alt="Preview" class="max-h-32 mx-auto rounded-md shadow-sm" />
              <p class="mt-2 font-medium" style="color: var(--text-primary); font-size: var(--text-sm);">{ocrFileName}</p>
              <div class="mt-2 flex items-center justify-center gap-2">
                <button type="button" class="btn-secondary px-3 py-1.5 inline-flex items-center gap-1.5" on:click={extractTextFromImage} disabled={ocrLoading}>
                  {#if ocrLoading}
                    <span class="spinner" style="border-color: rgba(99, 110, 114, 0.3); border-top-color: var(--text-secondary);"></span>
                    Extracting...
                  {:else}
                    Extract Text
                  {/if}
                </button>
                <button type="button" class="hover:underline" style="color: var(--primary); font-size: var(--text-xs);" on:click={clearPreview}>
                  Remove
                </button>
              </div>
            </div>
          {/if}
        </div>
      </div>

      <div id="translation-table">
        {#if translationResults.length > 0}
          <div class="p-4 rounded-xl" style="background: var(--surface); box-shadow: 0 1px 3px var(--shadow); border: 1px solid var(--border);">
            <button class="flex items-center justify-between w-full text-left" on:click={() => (showDetails = !showDetails)}>
              <h3 class="font-semibold" style="color: var(--text-primary); font-size: var(--text-base);">Translation Details</h3>
              <span style="color: var(--text-muted); font-size: var(--text-lg);">{showDetails ? "−" : "+"}</span>
            </button>
            {#if showDetails}
              <div class="mt-3 overflow-x-auto">
                <table class="w-full text-left">
                  <thead>
                    <tr style="border-bottom: 1px solid var(--border);">
                      <th class="py-1.5 px-2 font-semibold uppercase tracking-wider" style="color: var(--text-muted); font-size: var(--text-xs);">Chinese</th>
                      <th class="py-1.5 px-2 font-semibold uppercase tracking-wider" style="color: var(--text-muted); font-size: var(--text-xs);">Pinyin</th>
                      <th class="py-1.5 px-2 font-semibold uppercase tracking-wider" style="color: var(--text-muted); font-size: var(--text-xs);">English</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each translationResults as item}
                      <tr class="cursor-pointer translation-row" style="border-bottom: 1px solid var(--background-alt);">
                        <td class="py-2 px-2" style="font-family: var(--font-chinese); font-size: var(--text-chinese); color: var(--text-primary);">{item.segment}</td>
                        <td class="py-2 px-2" style="color: var(--text-secondary); font-size: var(--text-sm);">{item.pinyin}</td>
                        <td class="py-2 px-2" style="color: var(--secondary-dark); font-size: var(--text-sm);">{item.english}</td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {/if}
          </div>
        {/if}
      </div>
    </div>

    <div class="space-y-4">
      {#if !isExpandedView}
        <div id="job-queue-panel" class="input-card p-5">
          <div class="flex items-center justify-between mb-4">
            <h2 class="font-semibold" style="color: var(--text-primary); font-size: var(--text-base);">Translation Queue</h2>
            <span class="text-xs px-2 py-0.5 rounded-full" style="background: var(--pastel-3); color: var(--text-primary);">{queueCountLabel}</span>
          </div>
          <div class="space-y-3">
            {#if jobQueue.length === 0}
              <div class="text-center py-8">
                <p class="italic" style="color: var(--text-muted); font-size: var(--text-sm);">No translation jobs yet</p>
                <p class="mt-1" style="color: var(--text-muted); font-size: var(--text-xs);">Submit text on the left to start</p>
              </div>
            {:else}
              {#each jobQueue as job}
                <div class={`job-card ${job.status}`} on:click={() => expandJob(job.id)}>
                  <div class="job-header">
                    <div class="job-status">
                      <span class="job-status-icon"></span>
                      <span style="color: var(--text-secondary);">{statusLabels[job.status]}</span>
                    </div>
                    <span class="job-time">{formatTimeAgo(job.created_at)}</span>
                  </div>
                  <div class="job-preview">{job.input_preview}</div>
                  {#if job.full_translation_preview}
                    <div class="job-translation-preview">"{job.full_translation_preview}"</div>
                  {/if}
                  {#if job.status === "processing" && job.total_segments}
                    <div class="job-progress">
                      <div class="job-progress-fill" style={`width: ${(job.segment_count / job.total_segments) * 100}%`}></div>
                    </div>
                  {/if}
                  <div class="job-footer">
                    <span class="job-segments-count">
                      {#if job.segment_count !== null && job.total_segments !== null}
                        {job.segment_count} / {job.total_segments} segments
                      {:else if job.status === "completed" && job.segment_count}
                        {job.segment_count} segments
                      {/if}
                    </span>
                    <button
                      class="job-delete-btn"
                      on:click|stopPropagation={() => deleteJob(job.id)}
                      title="Delete"
                    >
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M3 6h18M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2" />
                      </svg>
                    </button>
                  </div>
                </div>
              {/each}
            {/if}
          </div>
        </div>
      {:else}
        <div id="expanded-job-view">
          <button class="mb-3 flex items-center gap-1 hover:underline" style="color: var(--primary); font-size: var(--text-sm);" on:click={backToQueue}>
            <span>&larr;</span> Back to Queue
          </button>

          <div id="original-text-panel" class="input-card p-4 mb-4">
            <label class="block font-medium mb-1.5" style="color: var(--text-secondary); font-size: var(--text-xs);">Original Text</label>
            <div class="font-chinese p-2 rounded" style="background: var(--pastel-7); color: var(--text-primary); font-size: var(--text-chinese); min-height: 60px; white-space: pre-wrap;">{currentRawText}</div>
          </div>

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
                        <span
                          class={getSegmentClasses(segment)}
                          style={`font-family: var(--font-chinese); font-size: var(--text-chinese); color: var(--text-primary); cursor: ${segment.pinyin || segment.english ? "pointer" : "default"}; ${getSegmentStyle(segment)}`}
                          on:mouseenter={(event) => handleSegmentHover(segment, event.currentTarget)}
                          on:mouseleave={handleSegmentLeave}
                          on:click={(event) => {
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
                        <button class="tooltip-btn" type="button" on:click={saveVocab}>Save to Learn</button>
                      {:else if tooltip.status === "learning"}
                        <button class="tooltip-btn" type="button" on:click={markKnown}>Mark as Known</button>
                      {:else if tooltip.status === "known"}
                        <button class="tooltip-btn" type="button" on:click={resumeLearning}>Resume Learning</button>
                      {:else}
                        <button class="tooltip-btn" type="button" on:click={saveVocab}>Save to Learn</button>
                      {/if}
                    {/if}
                  </div>
                  <div class="tooltip-arrow"></div>
                </div>
              </div>
            {/if}
          </div>

          {#if otherJobsCount > 0}
            <div class="mt-4">
              <button class="w-full input-card p-3 text-left hover:shadow-md transition-shadow" on:click={backToQueue}>
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
