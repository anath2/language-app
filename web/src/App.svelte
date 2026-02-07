<script lang="ts">
  import { getJson, postJson, deleteRequest } from "./lib/api";
  import { router } from "./lib/router.svelte";
  import type {
    TranslationSummary,
    ListTranslationsResponse,
    TranslationDetailResponse,
    CreateTranslationResponse,
    DueCountResponse,
    VocabSrsInfoListResponse,
    RecordLookupResponse,
    SaveVocabResponse,
    CreateTextResponse,
    LoadingState,
  } from "./lib/types";
  import type {
    ParagraphResult,
    SegmentResult,
    SavedVocabInfo,
  } from "./features/segments/types";
  import ReviewPanel from "./components/ReviewPanel.svelte";
  import TranslateForm from "./components/TranslateForm.svelte";
  import TranslationList from "./components/TranslationList.svelte";
  import Segments from "./features/segments/Segments.svelte";

  let translations = $state<TranslationSummary[]>([]);
  let currentTranslationId = $state<string | null>(null);
  let currentTranslationStatus = $state<string | null>(null);
  let currentParagraphs = $state<ParagraphResult[] | null>(null);
  let currentFullTranslation = $state<string | null>(null);
  let detailLoading = $state(false);
  let formLoading = $state<LoadingState>("idle");
  let formErrorMessage = $state("");

  let currentTextId = $state<string | null>(null);
  let currentRawText = $state("");
  let savedVocabMap = $state<Map<string, SavedVocabInfo>>(new Map());

  let reviewPanelOpen = $state(false);
  let dueCount = $state(0);

  const isDetailView = $derived(router.route.page === "translation");
  const translationCountLabel = $derived(`${translations.length} translation${translations.length !== 1 ? "s" : ""}`);
  const otherTranslationsCount = $derived(translations.filter((t) => t.id !== currentTranslationId).length);

  // Initial data load
  $effect(() => {
    void loadTranslations();
    void updateDueCount();
  });

  // React to route changes (handles popstate and initial deep link)
  $effect(() => {
    const route = router.route;
    if (route.page === "translation") {
      void loadTranslationFromRoute(route.id);
    } else {
      clearDetailState();
      void loadTranslations();
    }
  });

  function errorToMessage(error: unknown): string {
    if (error instanceof Error) return error.message;
    return String(error);
  }

  async function loadTranslations() {
    try {
      const data = await getJson<ListTranslationsResponse>("/api/translations?limit=20");
      translations = data.translations || [];
    } catch (error) {
      console.error("Failed to load translations:", error);
    }
  }

  async function handleSubmit(text: string) {
    if (!text) return;

    formLoading = "loading";
    try {
      const data = await postJson<CreateTranslationResponse>("/api/translations", {
        input_text: text,
        source_type: "text"
      });
      currentRawText = text;
      currentTextId = null;
      await loadTranslations();
      openTranslation(data.translation_id);
    } catch (error) {
      formErrorMessage = `Failed to submit translation: ${errorToMessage(error)}`;
      formLoading = "error";
    } finally {
      if (formLoading !== "error") {
        formLoading = "idle";
      }
    }
  }

  function openTranslation(id: string) {
    router.navigateTo(id);
  }

  async function loadTranslationFromRoute(id: string) {
    detailLoading = true;
    currentTranslationId = null;
    currentTranslationStatus = null;
    currentParagraphs = null;
    currentFullTranslation = null;

    try {
      const detail = await getJson<TranslationDetailResponse>(`/api/translations/${id}`);
      currentRawText = detail.input_text;
      currentTextId = null;

      // Set status/data BEFORE setting translationId so the $effect gets complete data
      if (detail.status === "completed" && detail.paragraphs) {
        currentTranslationStatus = "completed";
        currentParagraphs = detail.paragraphs;
        currentFullTranslation = detail.full_translation || null;
      } else if (detail.status === "processing" || detail.status === "pending") {
        currentTranslationStatus = detail.status;
      } else if (detail.status === "failed") {
        currentTranslationStatus = "failed";
      }

      // Set translationId LAST — this triggers the Segments $effect with all data ready
      currentTranslationId = id;
      await loadTranslations();
    } catch (_error) {
      currentTranslationStatus = "failed";
      currentTranslationId = id;
    } finally {
      detailLoading = false;
    }
  }

  function backToList() {
    router.navigateHome();
  }

  function clearDetailState() {
    detailLoading = false;
    currentTranslationId = null;
    currentTranslationStatus = null;
    currentParagraphs = null;
    currentFullTranslation = null;
    formLoading = "idle";
    formErrorMessage = "";
  }

  async function deleteTranslation(id: string) {
    if (!confirm("Delete this translation?")) return;

    try {
      await deleteRequest("/api/translations/" + id);
      if (currentTranslationId === id) {
        backToList();
      } else {
        await loadTranslations();
      }
    } catch (_error) {
      alert("Failed to delete translation");
    }
  }

  function handleStreamComplete() {
    void loadTranslations();
  }

  function handleSegmentsChanged(results: SegmentResult[]) {
    void fetchAndApplySrsInfo(results);
  }

  async function fetchAndApplySrsInfo(results: SegmentResult[]) {
    const headwords = [...new Set(results.filter((s) => s.pinyin || s.english).map((s) => s.segment))];
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
      <TranslateForm onSubmit={handleSubmit} loading={formLoading === "loading"} />
    </div>

    <div class="space-y-4">
      {#if !isDetailView}
        <div id="job-queue-panel" class="input-card p-5">
          <div class="flex items-center justify-between mb-4">
            <h2 class="font-semibold" style="color: var(--text-primary); font-size: var(--text-base);">Translations</h2>
            <span class="text-xs px-2 py-0.5 rounded-full" style="background: var(--pastel-3); color: var(--text-primary);">{translationCountLabel}</span>
          </div>
          <TranslationList {translations} onSelect={openTranslation} onDelete={deleteTranslation} />
        </div>
      {:else}
        <div id="expanded-job-view">
          <button class="mb-3 flex items-center gap-1 hover:underline" style="color: var(--primary); font-size: var(--text-sm);" onclick={backToList}>
            <span>&larr;</span> Back
          </button>

          <!-- svelte-ignore a11y_label_has_associated_control -->
          <div id="original-text-panel" class="input-card p-4 mb-4">
            <label class="block font-medium mb-1.5" style="color: var(--text-secondary); font-size: var(--text-xs);">Original Text</label>
            <div class="font-chinese p-2 rounded" style="background: var(--pastel-7); color: var(--text-primary); font-size: var(--text-chinese); min-height: 60px; white-space: pre-wrap;">{currentRawText}</div>
          </div>

          {#if detailLoading}
            <div class="input-card p-5 flex items-center justify-center" style="min-height: 200px;">
              <div class="text-center">
                <div class="spinner mx-auto mb-2" style="width: 20px; height: 20px; border-color: rgba(124, 158, 178, 0.3); border-top-color: var(--primary);"></div>
                <p style="color: var(--text-muted); font-size: var(--text-sm);">Loading...</p>
              </div>
            </div>
          {:else}
            <Segments
              translationId={currentTranslationId}
              translationStatus={currentTranslationStatus}
              paragraphs={currentParagraphs}
              fullTranslation={currentFullTranslation}
              rawText={currentRawText}
              {savedVocabMap}
              {onSaveVocab}
              {onMarkKnown}
              {onResumeLearning}
              {onRecordLookup}
              onStreamComplete={handleStreamComplete}
              onSegmentsChanged={handleSegmentsChanged}
            />
          {/if}

          {#if otherTranslationsCount > 0}
            <div class="mt-4">
              <button class="w-full input-card p-3 text-left hover:shadow-md transition-shadow" onclick={backToList}>
                <div class="flex items-center justify-between">
                  <span style="color: var(--text-secondary); font-size: var(--text-sm);">Translations</span>
                  <span class="text-xs px-2 py-0.5 rounded-full" style="background: var(--pastel-4); color: var(--text-primary);">{otherTranslationsCount} more</span>
                </div>
              </button>
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </main>
</div>
