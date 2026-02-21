<script lang="ts">
import { ChevronLeft, MessageCircle, Pencil } from '@lucide/svelte';
import { getJson, postJson } from '@/lib/api';
import { updateTranslationSource, updateTranslationTitle } from '@/features/translation/api';
import Button from '@/ui/Button.svelte';
import Card from '@/ui/Card.svelte';
import { translationStore } from '@/features/translation/stores/translationStore.svelte';
import TranslationResult from './components/TranslationResult.svelte';
import TranslationChat from './components/TranslationChat.svelte';
import type {
  CreateTextResponse,
  RecordLookupResponse,
  SavedVocabInfo,
  SaveVocabResponse,
  SegmentResult,
  TranslationDetailResponse,
  VocabSrsInfoListResponse,
} from '@/features/translation/types';

const { translationId, onBack }: { translationId: string | null; onBack: () => void } = $props();

let chatPaneOpen = $state(false);
let selectedText = $state('');
let currentTranslationStatus = $state<string | null>(null);
let currentFullTranslation = $state<string | null>(null);
let currentTitle = $state('');
let detailLoading = $state(false);

let isEditingTitle = $state(false);
let editedTitle = $state('');
let isSavingTitle = $state(false);

let isEditingSource = $state(false);
let editedSourceText = $state('');
let isSavingSource = $state(false);
let editSourceNotice = $state('');

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
  currentFullTranslation = null;
  currentTitle = '';
  isEditingSource = false;
  editedSourceText = '';
  editSourceNotice = '';

  try {
    const detail = await getJson<TranslationDetailResponse>(`/api/translations/${id}`);
    currentRawText = detail.input_text;
    currentTextId = null;
    currentFullTranslation = detail.full_translation || null;
    currentTranslationStatus = detail.status;
    currentTitle = detail.title || '';
  } catch (_error) {
    currentTranslationStatus = 'failed';
  } finally {
    detailLoading = false;
  }
}

function startEditTitle() {
  editedTitle = currentTitle;
  isEditingTitle = true;
}

function cancelEditTitle() {
  isEditingTitle = false;
  editedTitle = '';
}

async function saveTitle() {
  if (!translationId || isSavingTitle) return;
  const trimmed = editedTitle.trim();
  if (!trimmed) {
    isEditingTitle = false;
    return;
  }
  isSavingTitle = true;
  try {
    await updateTranslationTitle(translationId, trimmed);
    currentTitle = trimmed;
    isEditingTitle = false;
  } catch (_error) {
    // silently revert
    isEditingTitle = false;
  } finally {
    isSavingTitle = false;
    editedTitle = '';
  }
}

function handleTitleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') {
    void saveTitle();
  } else if (e.key === 'Escape') {
    cancelEditTitle();
  }
}

function startEditSource() {
  editedSourceText = currentRawText;
  isEditingSource = true;
  editSourceNotice = '';
}

function cancelEditSource() {
  isEditingSource = false;
  editedSourceText = '';
  editSourceNotice = '';
}

async function saveEditedSource() {
  if (!translationId || isSavingSource) return;
  isSavingSource = true;
  editSourceNotice = '';
  try {
    const result = await updateTranslationSource(translationId, editedSourceText);
    if (result.sentences_changed === 0) {
      editSourceNotice = 'No changes detected.';
      isEditingSource = false;
    } else {
      currentRawText = editedSourceText;
      currentTranslationStatus = 'pending';
      isEditingSource = false;
    }
  } catch (error) {
    editSourceNotice = error instanceof Error ? error.message : 'Failed to save changes.';
  } finally {
    isSavingSource = false;
  }
}

function handleTextSelection() {
  const sel = window.getSelection()?.toString().trim() ?? '';
  if (sel) selectedText = sel;
}

function clearDetailState() {
  detailLoading = false;
  currentTranslationStatus = null;
  currentFullTranslation = null;
  currentTitle = '';
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
  <div class="page-header">
    <Button variant="ghost" size="sm" onclick={onBack}>
      <ChevronLeft size={16} />
      <span>Back to translations</span>
    </Button>
    {#if currentTitle || translationId}
      <div class="title-area">
        {#if isEditingTitle}
          <!-- svelte-ignore a11y_autofocus -->
          <input
            class="title-input"
            autofocus
            bind:value={editedTitle}
            onblur={saveTitle}
            onkeydown={handleTitleKeydown}
            disabled={isSavingTitle}
          />
        {:else}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <span class="title-text" onclick={startEditTitle} title="Click to edit title">
            {currentTitle}
            <Pencil size={12} class="title-edit-icon" />
          </span>
        {/if}
      </div>
    {/if}
    {#if translationId}
      <Button variant="primary" size="sm" onclick={() => (chatPaneOpen = true)} ariaLabel="Open chat">
        <MessageCircle size={16} />
        <span>Chat</span>
      </Button>
    {/if}
  </div>

  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <div class="translation-layout" role="region" onmouseup={handleTextSelection}>
    <div class="translation-left">
      <Card padding="5" important class="sticky-top">
        <div class="panel-header">
          <span class="panel-label">Original Text</span>
          {#if currentTranslationStatus === 'completed' && !isEditingSource}
            <button class="edit-icon-btn" onclick={startEditSource} aria-label="Edit source text">
              <Pencil size={14} />
            </button>
          {/if}
        </div>
        {#if isEditingSource}
          <textarea
            class="source-textarea"
            bind:value={editedSourceText}
            rows={8}
            disabled={isSavingSource}
          ></textarea>
          {#if editSourceNotice}
            <p class="edit-notice">{editSourceNotice}</p>
          {/if}
          <div class="edit-actions">
            <Button variant="primary" size="sm" onclick={saveEditedSource} disabled={isSavingSource}>
              {isSavingSource ? 'Saving…' : 'Save & Retranslate'}
            </Button>
            <Button variant="ghost" size="sm" onclick={cancelEditSource} disabled={isSavingSource}>
              Cancel
            </Button>
          </div>
        {:else}
          <div class="chinese-text">{currentRawText}</div>
        {/if}
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

  <TranslationChat
    translationId={translationId}
    open={chatPaneOpen}
    onClose={() => (chatPaneOpen = false)}
    {selectedText}
    onClearSelectedText={() => { selectedText = ''; }}
  />
</div>

<style>
  .page-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    flex-wrap: wrap;
  }

  .title-area {
    flex: 1;
    min-width: 0;
    display: flex;
    justify-content: center;
  }

  .title-text {
    font-size: var(--text-base);
    font-weight: 600;
    color: var(--text-primary);
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: 2px var(--space-2);
    border-radius: var(--radius-sm);
    max-width: 400px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .title-text:hover {
    background: var(--surface-hover);
  }

  :global(.title-edit-icon) {
    opacity: 0;
    color: var(--text-muted);
    flex-shrink: 0;
    transition: opacity 0.15s ease;
  }

  .title-text:hover :global(.title-edit-icon) {
    opacity: 1;
  }

  .title-input {
    font-size: var(--text-base);
    font-weight: 600;
    color: var(--text-primary);
    background: var(--surface);
    border: 1px solid var(--color-primary);
    border-radius: var(--radius-sm);
    padding: 2px var(--space-2);
    outline: none;
    min-width: 200px;
    max-width: 400px;
  }

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

  .panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--space-2);
  }

  .panel-header .panel-label {
    margin-bottom: 0;
  }

  .edit-icon-btn {
    background: none;
    border: none;
    cursor: pointer;
    padding: 2px 4px;
    color: var(--text-secondary);
    display: flex;
    align-items: center;
    border-radius: var(--radius-sm);
  }

  .edit-icon-btn:hover {
    color: var(--text-primary);
    background: var(--surface-hover);
  }

  .source-textarea {
    width: 100%;
    box-sizing: border-box;
    font-family: var(--font-chinese);
    font-size: var(--text-chinese);
    line-height: 1.8;
    color: var(--text-primary);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: var(--space-2);
    resize: vertical;
    outline: none;
  }

  .source-textarea:focus {
    border-color: var(--color-primary);
  }

  .edit-notice {
    font-size: var(--text-sm);
    color: var(--text-secondary);
    margin: var(--space-2) 0 0;
  }

  .edit-actions {
    display: flex;
    gap: var(--space-2);
    margin-top: var(--space-3);
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
    white-space: pre-wrap;
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
