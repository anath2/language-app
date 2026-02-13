<script lang="ts">
import Button from '@/ui/Button.svelte';
import Card from '@/ui/Card.svelte';
import { Pencil } from '@lucide/svelte';
import SegmentResult from './SegmentResult.svelte';
import TranslationTable from './TranslationTable.svelte';
import SegmentEditor from './SegmentEditor.svelte';
import type {
  DisplayParagraph,
  LoadingState,
  ParagraphMeta,
  ParagraphResult,
  ProgressState,
  SavedVocabInfo,
  SegmentResult as SegmentResultType,
  StreamEvent,
  StreamSegmentResult,
} from '@/features/translation/types';

const {
  translationId,
  translationStatus,
  fullTranslation: initialFullTranslation,
  rawText,
  savedVocabMap,
  onSaveVocab,
  onMarkKnown,
  onResumeLearning,
  onRecordLookup,
  onStreamComplete,
  onSegmentsChanged,
}: {
  translationId: string | null;
  translationStatus: string | null;
  fullTranslation: string | null;
  rawText: string;
  savedVocabMap: Map<string, SavedVocabInfo>;
  onSaveVocab: (
    headword: string,
    pinyin: string,
    english: string
  ) => Promise<SavedVocabInfo | null>;
  onMarkKnown: (headword: string, vocabItemId: string) => Promise<void>;
  onResumeLearning: (headword: string, vocabItemId: string) => Promise<void>;
  onRecordLookup: (headword: string, vocabItemId: string) => Promise<void>;
  onStreamComplete: () => void;
  onSegmentsChanged: (results: SegmentResultType[]) => void;
} = $props();

let translationResults = $state<SegmentResultType[]>([]);
let paragraphMeta = $state<ParagraphMeta[]>([]);
let fullTranslation = $state('');
let progress = $state<ProgressState>({ current: 0, total: 0 });
let loadingState = $state<LoadingState>('idle');
let errorMessage = $state('');
let isEditMode = $state(false);

const displayParagraphs = $derived(buildDisplayParagraphs(paragraphMeta, translationResults));

let lastTranslationId = $state<string | null>(null);

$effect(() => {
  const currentId = translationId;
  const currentStatus = translationStatus;

  if (currentId !== lastTranslationId) {
    lastTranslationId = currentId;
    isEditMode = false;

    if (!currentId) {
      resetState();
      return;
    }

    if (currentStatus === 'failed') {
      loadingState = 'error';
      errorMessage = 'Translation failed';
    } else if (currentStatus) {
      void streamTranslation(currentId);
    } else {
      loadingState = 'loading';
    }
  }
});

function resetState() {
  translationResults = [];
  paragraphMeta = [];
  fullTranslation = '';
  progress = { current: 0, total: 0 };
  loadingState = 'idle';
  errorMessage = '';
  isEditMode = false;
}

function buildDisplayParagraphs(
  meta: ParagraphMeta[],
  results: SegmentResultType[]
): DisplayParagraph[] {
  let globalIndex = 0;
  return meta.map((para, paraIdx) => {
    const segments = Array.from({ length: para.segment_count }).map(() => {
      const existing = results[globalIndex];
      const entry = existing
        ? { ...existing }
        : {
            segment: 'Loading...',
            pinyin: '',
            english: '',
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

function flattenParagraphs(paragraphs: ParagraphResult[]): SegmentResultType[] {
  const results: SegmentResultType[] = [];
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

async function streamTranslation(streamId: string) {
  translationResults = [];
  paragraphMeta = [];
  fullTranslation = '';
  progress = { current: 0, total: 0 };
  loadingState = 'loading';

  try {
    const response = await fetch(`/api/translations/${streamId}/stream`, {
      credentials: 'include',
    });
    if (!response.body) {
      throw new Error('Streaming unavailable');
    }
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { value, done } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      for (const line of lines) {
        if (!line.startsWith('data: ')) continue;
        const data = JSON.parse(line.slice(6)) as StreamEvent;
        if (data.type === 'start') {
          paragraphMeta = data.paragraphs || [];
          if (paragraphMeta.length === 0 && data.total) {
            paragraphMeta = [{ segment_count: data.total, indent: '', separator: '' }];
          }
          progress = { current: 0, total: data.total || 0 };
          fullTranslation = data.fullTranslation || '';
          loadingState = 'idle';
        } else if (data.type === 'progress') {
          progress = { current: data.current, total: data.total };
          updateSegmentResult(data.result);
        } else if (data.type === 'complete') {
          fullTranslation = data.fullTranslation || fullTranslation;
          if (data.paragraphs) {
            paragraphMeta = data.paragraphs.map((para) => ({
              segment_count: para.translations.length,
              indent: para.indent,
              separator: para.separator,
            }));
            translationResults = flattenParagraphs(data.paragraphs);
          }
          loadingState = 'idle';
          onStreamComplete();
          onSegmentsChanged(translationResults);
        } else if (data.type === 'error') {
          loadingState = 'error';
          errorMessage = data.message || 'Streaming failed';
          onStreamComplete();
        }
      }
    }
  } catch (error) {
    loadingState = 'error';
    errorMessage = `Streaming failed: ${errorToMessage(error)}`;
    onStreamComplete();
  }
}

function updateSegmentResult(result: StreamSegmentResult) {
  const index = result.index;
  const updated: SegmentResultType = {
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

function handleEditSave(results: SegmentResultType[], meta: ParagraphMeta[]) {
  translationResults = results;
  paragraphMeta = meta;
  isEditMode = false;
  onSegmentsChanged(translationResults);
}

function handleEditCancel() {
  isEditMode = false;
}
</script>

<Card id="results" padding="5" shadow>
  {#if loadingState === "loading"}
    <div class="loading-state">
      <div class="spinner spinner-dark"></div>
      <p class="loading-label">Starting translation...</p>
    </div>
  {:else if loadingState === "error"}
    <div class="error-banner">
      <p>{errorMessage}</p>
    </div>
  {:else if displayParagraphs.length === 0}
    <div class="empty-state-inline">
      <p>Translation results will appear here</p>
    </div>
  {:else}
    <div class="section-header">
      <span class="section-title">Segmented Text</span>
      {#if !isEditMode && progress.current >= progress.total && translationResults.length > 0}
        <Button size="xs" variant="ghost" shape="pill" onclick={enterEditMode}>
         <Pencil size={16} /> Edit Segments
        </Button>
      {/if}
    </div>

    {#if isEditMode}
      <SegmentEditor
        {translationResults}
        {paragraphMeta}
        currentTranslationId={translationId}
        currentRawText={rawText}
        onSave={handleEditSave}
        onCancel={handleEditCancel}
      />
    {:else}
      <SegmentResult
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
</Card>

<div id="translation-table">
  <TranslationTable results={translationResults} />
</div>

<style>
  .loading-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: var(--space-8) 0;
  }

  .loading-label {
    color: var(--text-muted);
    font-size: var(--text-sm);
    margin-top: var(--space-2);
  }

  .error-banner {
    background: var(--error-bg);
    color: var(--error);
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-lg);
    border-left: 3px solid var(--error);
    font-size: var(--text-sm);
  }

  .empty-state-inline {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-8) 0;
    color: var(--text-muted);
    font-size: var(--text-sm);
    font-style: italic;
  }

  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--space-3);
  }

  .section-title {
    font-size: var(--text-xs);
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
</style>
