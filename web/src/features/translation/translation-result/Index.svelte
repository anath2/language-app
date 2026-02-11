<script lang="ts">
import { ChevronLeft } from '@lucide/svelte';
import { getJson, postJson } from '@/lib/api';
import Button from '@/ui/Button.svelte';
import Card from '@/ui/Card.svelte';
import { translationStore } from '@/features/translation/stores/translationStore.svelte';
import TranslationResult from './components/TranslationResult.svelte';
import type {
  CreateTextResponse,
  ParagraphResult,
  RecordLookupResponse,
  SavedVocabInfo,
  SaveVocabResponse,
  SegmentResult,
  TranslationDetailResponse,
  VocabSrsInfoListResponse,
} from '@/features/translation/types';

const { translationId, onBack }: { translationId: string | null; onBack: () => void } = $props();

let currentTranslationStatus = $state<string | null>(null);
let currentParagraphs = $state<ParagraphResult[] | null>(null);
let currentFullTranslation = $state<string | null>(null);
let detailLoading = $state(false);

let currentTextId = $state<string | null>(null);
let currentRawText = $state('');
let savedVocabMap = $state<Map<string, SavedVocabInfo>>(new Map());

$effect(() => {
  const id = translationId;
  if (id) {
    void loadTranslationFromRoute(id);
  } else {
    clearDetailState();
  }
});

async function loadTranslationFromRoute(id: string) {
  detailLoading = true;
  currentTranslationStatus = null;
  currentParagraphs = null;
  currentFullTranslation = null;

  try {
    const detail = await getJson<TranslationDetailResponse>(`/api/translations/${id}`);
    currentRawText = detail.input_text;
    currentTextId = null;

    if (detail.status === 'completed' && detail.paragraphs) {
      currentTranslationStatus = 'completed';
      currentParagraphs = detail.paragraphs;
      currentFullTranslation = detail.full_translation || null;
    } else if (detail.status === 'processing' || detail.status === 'pending') {
      currentTranslationStatus = detail.status;
    } else if (detail.status === 'failed') {
      currentTranslationStatus = 'failed';
    }
  } catch (_error) {
    currentTranslationStatus = 'failed';
  } finally {
    detailLoading = false;
  }
}

function clearDetailState() {
  detailLoading = false;
  currentTranslationStatus = null;
  currentParagraphs = null;
  currentFullTranslation = null;
}

function handleStreamComplete() {
  void translationStore.loadTranslations();
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

<div class="page-container">
  <Button variant="ghost" size="sm" onclick={onBack}>
    <ChevronLeft size={16} />
    <span>Back to translations</span>
  </Button>

  <div class="translation-layout">
    <div class="translation-left">
      <Card padding="5" important class="sticky-top">
        <span class="panel-label">Original Text</span>
        <div class="chinese-text">{currentRawText}</div>
        <div class="panel-divider"></div>
        <span class="panel-label">Translated Text</span>
        <div class="translated-text">{currentFullTranslation}</div>
      </Card>
    </div>

    <div class="translation-right">
      {#if detailLoading}
        <Card padding="5" class="loading-card">
          <div class="loading-inner">
            <div class="spinner spinner-dark"></div>
            <p class="loading-text">Loading...</p>
          </div>
        </Card>
      {:else}
        <TranslationResult
          translationId={translationId}
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

<style>
  .page-container {
    max-width: 1200px;
    margin: 0 auto;
    padding: 1.5rem;
  }

  .translation-layout {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1.5rem;
    align-items: start;
    margin-top: var(--space-2);
  }

  .translation-left,
  .translation-right {
    min-width: 0;
  }

  .panel-label {
    display: block;
    color: var(--text-secondary);
    font-size: var(--text-xs);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    margin-bottom: var(--space-2);
  }

  .chinese-text {
    font-family: var(--font-chinese);
    font-size: var(--text-chinese);
    color: var(--text-primary);
    line-height: 1.8;
    white-space: pre-wrap;
  }

  .panel-divider {
    height: 1px;
    background: var(--border);
    margin: var(--space-4) 0;
  }

  .translated-text {
    font-size: var(--text-base);
    color: var(--text-primary);
    line-height: var(--leading-relaxed);
  }

  :global(.loading-card) {
    min-height: 200px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .loading-inner {
    text-align: center;
  }

  .loading-text {
    color: var(--text-muted);
    font-size: var(--text-sm);
    margin-top: var(--space-2);
  }

  :global(.sticky-top) {
    position: sticky;
    top: 80px;
  }

  @media (max-width: 960px) {
    .translation-layout {
      grid-template-columns: 1fr;
    }

    :global(.sticky-top) {
      position: static;
    }
  }

  @media (max-width: 640px) {
    .page-container {
      padding: 1rem;
    }
  }
</style>
