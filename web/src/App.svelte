<script lang="ts">
import NavBar from '@/components/NavBar.svelte';
import Admin from '@/features/admin/components/Admin.svelte';
import Login from '@/features/auth/components/Login.svelte';
import { auth } from '@/features/auth/stores/authStore.svelte';
import Segments from '@/features/translation/components/Segments.svelte';
import TranslateForm from '@/features/translation/components/TranslateForm.svelte';
import TranslationList from '@/features/translation/components/TranslationList.svelte';
import { translationStore } from '@/features/translation/stores/translationStore.svelte';
import type { ParagraphResult, SavedVocabInfo, SegmentResult } from '@/features/translation/types';
import ReviewPanel from '@/features/vocab/components/ReviewPanel.svelte';
import { reviewStore } from '@/features/vocab/stores/reviewStore.svelte';
import { vocabStore } from '@/features/vocab/stores/vocabStore.svelte';
import { deleteRequest, getJson, postJson } from '@/lib/api';
import { router } from '@/lib/router.svelte';
import type {
  CreateTextResponse,
  CreateTranslationResponse,
  DueCountResponse,
  ListTranslationsResponse,
  LoadingState,
  RecordLookupResponse,
  SaveVocabResponse,
  TranslationDetailResponse,
  TranslationSummary,
  VocabSrsInfoListResponse,
} from '@/lib/types';

let translations = $state<TranslationSummary[]>([]);
let currentTranslationId = $state<string | null>(null);
let currentTranslationStatus = $state<string | null>(null);
let currentParagraphs = $state<ParagraphResult[] | null>(null);
let currentFullTranslation = $state<string | null>(null);
let detailLoading = $state(false);
let formLoading = $state<LoadingState>('idle');
let formErrorMessage = $state('');

let currentTextId = $state<string | null>(null);
let currentRawText = $state('');
let savedVocabMap = $state<Map<string, SavedVocabInfo>>(new Map());

let reviewPanelOpen = $state(false);

const currentPage = $derived(router.route.page);

// Check authentication on mount
$effect(() => {
  void auth.checkAuthStatus();
});

// Initial data load after auth is checked
$effect(() => {
  if (!auth.isLoading && auth.isAuthenticated) {
    void loadTranslations();
  }
});

// React to route changes (handles popstate and initial deep link)
$effect(() => {
  if (!auth.isAuthenticated && router.route.page !== 'login') {
    // Only process routes if authenticated or on login page
    return;
  }

  const route = router.route;
  if (route.page === 'translation') {
    void loadTranslationFromRoute(route.id);
  } else if (route.page === 'home') {
    clearDetailState();
    void loadTranslations();
  } else {
    clearDetailState();
  }
});

function errorToMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

async function loadTranslations() {
  try {
    const data = await getJson<ListTranslationsResponse>('/api/translations?limit=20');
    translations = data.translations || [];
  } catch (error) {
    console.error('Failed to load translations:', error);
  }
}

async function handleSubmit(text: string) {
  if (!text) return;

  formLoading = 'loading';
  try {
    const data = await postJson<CreateTranslationResponse>('/api/translations', {
      input_text: text,
      source_type: 'text',
    });
    currentRawText = text;
    currentTextId = null;
    await loadTranslations();
    openTranslation(data.translation_id);
  } catch (error) {
    formErrorMessage = `Failed to submit translation: ${errorToMessage(error)}`;
    formLoading = 'error';
  } finally {
    if (formLoading !== 'error') {
      formLoading = 'idle';
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
    if (detail.status === 'completed' && detail.paragraphs) {
      currentTranslationStatus = 'completed';
      currentParagraphs = detail.paragraphs;
      currentFullTranslation = detail.full_translation || null;
    } else if (detail.status === 'processing' || detail.status === 'pending') {
      currentTranslationStatus = detail.status;
    } else if (detail.status === 'failed') {
      currentTranslationStatus = 'failed';
    }

    // Set translationId LAST — this triggers the Segments $effect with all data ready
    currentTranslationId = id;
    await loadTranslations();
  } catch (_error) {
    currentTranslationStatus = 'failed';
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
  formLoading = 'idle';
  formErrorMessage = '';
}

async function deleteTranslation(id: string) {
  if (!confirm('Delete this translation?')) return;

  try {
    await deleteRequest('/api/translations/' + id);
    if (currentTranslationId === id) {
      backToList();
    } else {
      await loadTranslations();
    }
  } catch (_error) {
    alert('Failed to delete translation');
  }
}

function handleStreamComplete() {
  void loadTranslations();
}

function handleSegmentsChanged(results: SegmentResult[]) {
  void fetchAndApplySrsInfo(results);
}

async function fetchAndApplySrsInfo(results: SegmentResult[]) {
  const headwords = [
    ...new Set(results.filter((s) => s.pinyin || s.english).map((s) => s.segment)),
  ];
  if (headwords.length === 0) return;

  try {
    const params = new URLSearchParams();
    params.set('headwords', headwords.join(','));
    const data = await getJson<VocabSrsInfoListResponse>(
      `/api/vocab/srs-info?${params.toString()}`
    );
    const nextMap = new Map<string, SavedVocabInfo>();
    data.items.forEach((info) => {
      const opacity = info.status === 'known' ? 0 : info.opacity;
      nextMap.set(info.headword, {
        vocabItemId: info.vocab_item_id,
        opacity,
        isStruggling: info.is_struggling,
        status: info.status,
      });
    });
    savedVocabMap = nextMap;
  } catch (error) {
    console.error('Failed to fetch SRS info:', error);
  }
}

function openReviewPanel() {
  reviewPanelOpen = true;
}

function closeReviewPanel() {
  reviewPanelOpen = false;
}

async function ensureSavedText() {
  if (currentTextId || !currentRawText) return currentTextId;

  const data = await postJson<CreateTextResponse>('/api/texts', {
    raw_text: currentRawText,
    source_type: 'text',
    metadata: {},
  });
  currentTextId = data.id;
  return currentTextId;
}

async function onSaveVocab(
  headword: string,
  pinyin: string,
  english: string
): Promise<SavedVocabInfo | null> {
  try {
    await ensureSavedText();
    const data = await postJson<SaveVocabResponse>('/api/vocab/save', {
      headword,
      pinyin,
      english,
      text_id: currentTextId,
      snippet: currentRawText,
      status: 'learning',
    });
    const info: SavedVocabInfo = {
      vocabItemId: data.vocab_item_id,
      opacity: 1,
      isStruggling: false,
      status: 'learning',
    };
    savedVocabMap = new Map(savedVocabMap.set(headword, info));
    return info;
  } catch (error) {
    console.error('Failed to save vocab:', error);
    return null;
  }
}

async function onMarkKnown(headword: string, vocabItemId: string) {
  try {
    await postJson('/api/vocab/status', {
      vocab_item_id: vocabItemId,
      status: 'known',
    });
    const info = savedVocabMap.get(headword);
    if (info) {
      savedVocabMap = new Map(
        savedVocabMap.set(headword, { ...info, status: 'known', opacity: 0 })
      );
    }
  } catch (error) {
    console.error('Failed to mark known:', error);
  }
}

async function onResumeLearning(headword: string, vocabItemId: string) {
  try {
    await postJson('/api/vocab/status', {
      vocab_item_id: vocabItemId,
      status: 'learning',
    });
    const info = savedVocabMap.get(headword);
    if (info) {
      savedVocabMap = new Map(
        savedVocabMap.set(headword, { ...info, status: 'learning', opacity: 1 })
      );
    }
  } catch (error) {
    console.error('Failed to resume learning:', error);
  }
}

async function onRecordLookup(headword: string, vocabItemId: string) {
  try {
    const data = await postJson<RecordLookupResponse>('/api/vocab/lookup', {
      vocab_item_id: vocabItemId,
    });
    const info = savedVocabMap.get(headword);
    if (info) {
      savedVocabMap = new Map(
        savedVocabMap.set(headword, {
          ...info,
          opacity: data.opacity,
          isStruggling: data.is_struggling,
        })
      );
    }
  } catch (error) {
    console.error('Failed to record lookup:', error);
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

</svelte:head>

{#if !auth.isAuthenticated && !auth.isLoading && router.route.page !== "login"}
  <Login returnUrl={window.location.pathname + window.location.search} />
{:else if auth.isAuthenticated || router.route.page === "login"}
  <NavBar />

  {#if currentPage === "login"}
    {#if router.route.page === "login"}
      <Login returnUrl={router.route.returnUrl} />
    {/if}
  {:else}

    <ReviewPanel open={reviewPanelOpen} onClose={closeReviewPanel} onDueCountChange={() => {}} />

    {#if currentPage === "home"}
  <!-- Home Page: 2-Column Layout -->
  <div class="page-container">
    <div class="home-layout">
      <!-- Left Column: Translate Form -->
      <div class="home-left">
        <TranslateForm onSubmit={handleSubmit} loading={formLoading === "loading"} />
      </div>

      <!-- Right Column: Recent Translations -->
      <div class="home-right">
        <div class="input-card p-5">
          <div class="flex items-center justify-between mb-4">
            <h2 class="font-semibold" style="color: var(--text-primary); font-size: var(--text-base);">Recent Translations</h2>
            <span class="text-xs px-2 py-0.5 rounded-full" style="background: var(--background-alt); color: var(--text-secondary);">
              {translations.length} total
            </span>
          </div>
          <TranslationList {translations} onSelect={openTranslation} onDelete={deleteTranslation} />
        </div>
      </div>
    </div>
  </div>

{:else if currentPage === "translation"}
  <!-- Translation Detail Page: 2-Column Layout -->
  <div class="page-container">
    <button class="mb-4 flex items-center gap-1 hover:underline" style="color: var(--primary); font-size: var(--text-sm);" onclick={backToList}>
      <span>&larr;</span> Back to translations
    </button>

    <div class="translation-layout">
      <!-- Left Column: Original Text -->
      <div class="translation-left">
        <!-- svelte-ignore a11y_label_has_associated_control -->
        <div id="original-text-panel" class="input-card p-4 sticky-top">
          <label class="block font-medium mb-2" style="color: var(--text-secondary); font-size: var(--text-xs); text-transform: uppercase; letter-spacing: 0.05em;">Original Text</label>
          <div class="font-chinese p-3 rounded" style="color: var(--text-primary); font-size: var(--text-chinese); line-height: 1.8; white-space: pre-wrap;">{currentRawText}</div>
        </div>
      </div>

      <!-- Right Column: Translation Results -->
      <div class="translation-right">
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
      </div>
    </div>
  </div>

{:else if currentPage === "vocab"}
  <!-- Vocab Page: Stub -->
  <div class="page-container max-w-4xl">
    <div class="space-y-6">
      <div class="input-card p-6 text-center">
        <div class="text-4xl mb-4">📖</div>
        <h2 class="font-semibold mb-2" style="color: var(--text-primary); font-size: var(--text-xl);">Vocabulary</h2>
        <p class="mb-6" style="color: var(--text-secondary);">Review and manage your saved vocabulary words.</p>
        <button class="btn-primary px-6 py-2" onclick={openReviewPanel}>
          Review Due Cards
        </button>
      </div>
    </div>
  </div>

{:else if currentPage === "admin"}
    <!-- Admin Page -->
    <div class="page-container max-w-4xl">
      <Admin />
    </div>
  {/if}

  {/if}

{/if}

<style>
  .page-container {
    max-width: 1200px;
    margin: 0 auto;
    padding: 1.5rem;
  }

  .page-container.max-w-4xl {
    max-width: 56rem;
  }

  /* Home Page: 2-Column Layout */
  .home-layout {
    display: grid;
    grid-template-columns: 1fr 380px;
    gap: 1.5rem;
    align-items: start;
  }

  .home-left {
    min-width: 0;
  }

  .home-right {
    min-width: 0;
  }

  /* Translation Detail: 2-Column Layout */
  .translation-layout {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1.5rem;
    align-items: start;
  }

  .translation-left {
    min-width: 0;
  }

  .translation-right {
    min-width: 0;
  }

  .sticky-top {
    position: sticky;
    top: 80px;
  }

  @media (max-width: 960px) {
    .home-layout {
      grid-template-columns: 1fr;
    }

    .translation-layout {
      grid-template-columns: 1fr;
    }

    .sticky-top {
      position: static;
    }
  }

  @media (max-width: 640px) {
    .page-container {
      padding: 1rem;
    }
  }
</style>
