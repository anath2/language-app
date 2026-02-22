<script lang="ts">
import { X } from '@lucide/svelte';
import Button from '@/ui/Button.svelte';
import Loader from '@/ui/Loader.svelte';
import TextArea from '@/ui/TextArea.svelte';
import { getJson, postJson, postJsonStream } from '@/lib/api';
import type { ChatMessage, ChatReviewCard, ChatListResponse, ChatStreamEvent } from '@/features/translation/types';

const {
  translationId,
  open,
  onClose,
  selectedText = '',
  onClearSelectedText,
}: {
  translationId: string | null;
  open: boolean;
  onClose: () => void;
  selectedText?: string;
  onClearSelectedText?: () => void;
} = $props();

let messages = $state<ChatMessage[]>([]);
let listLoading = $state(false);
let streaming = $state(false);
let streamingContent = $state('');
let streamingToolCall = $state(false);
let inputValue = $state('');
let errorMessage = $state('');
let listError = $state('');

// Track per-message dedup state (message_id -> bool)
let dedupMap = $state<Record<string, boolean>>({});

$effect(() => {
  if (!open || !translationId) return;
  void loadMessages();
});

async function loadMessages() {
  if (!translationId) return;
  listLoading = true;
  listError = '';
  try {
    const data = await getJson<ChatListResponse>(`/api/translations/${translationId}/chat/list`);
    messages = data.messages ?? [];
  } catch (e) {
    listError = e instanceof Error ? e.message : 'Failed to load chat';
    messages = [];
  } finally {
    listLoading = false;
  }
}

async function sendMessage() {
  const text = inputValue.trim();
  if (!text || !translationId || streaming) return;

  const userMessage: ChatMessage = {
    id: `temp-${Date.now()}`,
    chat_id: '',
    translation_id: translationId,
    message_idx: 0, // placeholder; server-assigned index is loaded on reload
    role: 'user',
    content: text,
    selected_text: selectedText || undefined,
    created_at: new Date().toISOString(),
  };
  messages = [...messages, userMessage];
  inputValue = '';
  errorMessage = '';
  streaming = true;
  streamingContent = '';
  streamingToolCall = false;

  try {
    await postJsonStream<ChatStreamEvent>(
      `/api/translations/${translationId}/chat/new`,
      { message: text, selected_text: selectedText },
      (event) => {
        if (event.type === 'tool_call_start') {
          streamingToolCall = true;
        } else if (event.type === 'chunk' && event.delta != null) {
          streamingContent += event.delta;
        } else if (event.type === 'complete') {
          const aiMessage: ChatMessage = {
            id: event.message_id ?? `ai-${Date.now()}`,
            chat_id: '',
            translation_id: translationId,
            message_idx: 0, // placeholder; server-assigned index is loaded on reload
            role: 'ai',
            content: event.content ?? streamingContent,
            created_at: new Date().toISOString(),
          };
          const toolMessages: ChatMessage[] = (event.tool_results ?? []).map((tr) => ({
            id: tr.message_id,
            chat_id: '',
            translation_id: translationId,
            message_idx: 0, // placeholder; server-assigned index is loaded on reload
            role: 'tool',
            content: tr.review_card.chinese_text,
            created_at: new Date().toISOString(),
            review_card: tr.review_card,
          }));
          messages = [...messages, aiMessage, ...toolMessages];
          streamingContent = '';
        } else if (event.type === 'error') {
          errorMessage = event.message ?? 'Stream error';
        }
      }
    );
  } catch (e) {
    errorMessage = e instanceof Error ? e.message : 'Failed to send message';
  } finally {
    streaming = false;
    streamingContent = '';
    streamingToolCall = false;
    onClearSelectedText?.();
  }
}

async function clearChat() {
  if (!translationId || streaming) return;
  try {
    await postJson<{ ok: boolean }>(`/api/translations/${translationId}/chat/clear`, {});
    messages = [];
    errorMessage = '';
  } catch (e) {
    errorMessage = e instanceof Error ? e.message : 'Failed to clear chat';
  }
}

async function acceptReviewCard(msg: ChatMessage) {
  if (!translationId || !msg.review_card) return;
  try {
    const res = await postJson<{ ok: boolean; deduplicated: boolean }>(
      `/api/translations/${translationId}/chat/messages/${msg.id}/accept`,
      {}
    );
    messages = messages.map((m) =>
      m.id === msg.id
        ? { ...m, review_card: { ...m.review_card!, status: 'accepted' } }
        : m
    );
    if (res.deduplicated) {
      dedupMap = { ...dedupMap, [msg.id]: true };
    }
  } catch (e) {
    errorMessage = e instanceof Error ? e.message : 'Failed to accept review card';
  }
}

async function rejectReviewCard(msg: ChatMessage) {
  if (!translationId || !msg.review_card) return;
  try {
    await postJson<{ ok: boolean }>(
      `/api/translations/${translationId}/chat/messages/${msg.id}/reject`,
      {}
    );
    // Tool message with null card is not rendered — just drop it from the local array.
    messages = messages.filter((m) => m.id !== msg.id);
  } catch (e) {
    errorMessage = e instanceof Error ? e.message : 'Failed to reject review card';
  }
}
</script>

<svelte:window onkeydown={(e) => { if (e.key === 'Escape' && open) onClose(); }} />

{#if open}
  <div class="chat-panel">
    <header class="chat-panel-header">
      <h2 class="chat-panel-title">Chat</h2>
      <Button variant="ghost" size="xs" iconOnly ariaLabel="Close panel" onclick={onClose}>
        <X size={18} />
      </Button>
    </header>
    <div class="chat-panel-content">
    <div class="chat-layout">
    <div class="chat-messages" role="log" aria-live="polite">
      {#if listLoading}
        <p class="chat-status">Loading...</p>
      {:else if listError}
        <p class="chat-status chat-error">{listError}</p>
      {:else if messages.length === 0 && !streamingContent}
        <p class="chat-status">No messages yet. Ask something about this translation.</p>
      {:else}
        {#each messages as msg (msg.id)}
          {#if msg.role === 'tool'}
            {#if msg.review_card}
              <div class="review-card review-card-standalone">
                <div class="review-card-chinese">{msg.review_card.chinese_text}</div>
                <div class="review-card-pinyin">{msg.review_card.pinyin}</div>
                <div class="review-card-english">{msg.review_card.english}</div>
                {#if msg.review_card.status === 'pending'}
                  <div class="review-card-actions">
                    <Button variant="primary" size="xs" onclick={() => acceptReviewCard(msg)}>
                      Accept
                    </Button>
                    <Button variant="ghost" size="xs" onclick={() => rejectReviewCard(msg)}>
                      Reject
                    </Button>
                  </div>
                {:else if msg.review_card.status === 'accepted'}
                  <div class="review-card-badge">
                    {dedupMap[msg.id] ? 'Already in SRS' : 'Saved to SRS'}
                  </div>
                {/if}
              </div>
            {/if}
          {:else}
            <div class="chat-bubble chat-bubble-{msg.role}">
              <div class="chat-bubble-content">{msg.content}</div>
            </div>
          {/if}
        {/each}
        {#if streaming && streamingToolCall}
          <div class="chat-bubble chat-bubble-ai chat-bubble-tool-call">
            <Loader variant="chat" />
            <span class="tool-call-label">Generating review card…</span>
          </div>
        {:else if streaming && !streamingContent}
          <div class="chat-bubble chat-bubble-ai">
            <Loader variant="chat" />
          </div>
        {:else if streamingContent}
          <div class="chat-bubble chat-bubble-ai chat-bubble-streaming">
            <div class="chat-bubble-content">{streamingContent}</div>
          </div>
        {/if}
      {/if}
    </div>
    {#if errorMessage}
      <p class="chat-inline-error">{errorMessage}</p>
    {/if}
    {#if selectedText}
      <div class="selection-chip">
        <span class="selection-chip-text">{selectedText.length > 60 ? selectedText.slice(0, 60) + '…' : selectedText}</span>
        <button class="selection-chip-dismiss" onclick={() => onClearSelectedText?.()}>✕</button>
      </div>
    {/if}
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div
      class="chat-input-row"
      role="group"
      aria-label="Chat input"
      onkeydown={(e) => {
        if (e.key === 'Enter' && !e.shiftKey && (e.target as HTMLElement).closest('.chat-input-row')) {
          const target = e.target as HTMLTextAreaElement;
          if (target?.tagName === 'TEXTAREA') {
            e.preventDefault();
            sendMessage();
          }
        }
      }}
    >
      <TextArea
        bind:value={inputValue}
        placeholder="Ask about this translation..."
        rows={2}
        disabled={streaming}
        class="chat-input"
      />
      <div class="chat-actions">
        <Button
          variant="primary"
          size="sm"
          disabled={streaming || !inputValue.trim()}
          onclick={sendMessage}
        >
          {streaming ? 'Sending...' : 'Send'}
        </Button>
      </div>
    </div>
    {#if messages.length > 0 && !streaming}
      <Button variant="ghost" size="xs" onclick={clearChat} class="chat-clear">
        Clear chat
      </Button>
    {/if}
    </div>
    </div>
  </div>
{/if}

<style>
  .chat-panel {
    display: flex;
    flex-direction: column;
    height: 100vh;
    position: sticky;
    top: 0;
    border-left: 1px solid var(--border);
    background: var(--surface);
    box-shadow: -4px 0 12px var(--shadow);
    animation: chat-slide-in 0.25s ease;
    overflow: hidden;
  }

  .chat-panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    padding: var(--space-4) var(--space-5);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .chat-panel-title {
    margin: 0;
    font-size: var(--text-lg);
    font-weight: 600;
    color: var(--text-primary);
  }

  .chat-panel-content {
    flex: 1;
    overflow: auto;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }

  @keyframes chat-slide-in {
    from { transform: translateX(100%); }
    to { transform: translateX(0); }
  }

  /* Full-width on narrow screens */
  @media (max-width: 900px) {
    .chat-panel {
      position: fixed;
      inset: 0;
      width: 100%;
      height: 100%;
    }
  }

  .chat-layout {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 320px;
    padding: var(--space-4);
  }

  .chat-messages {
    flex: 1;
    overflow: auto;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    margin-bottom: var(--space-4);
    padding-bottom: var(--space-2);
  }

  .chat-status {
    color: var(--text-muted);
    font-size: var(--text-sm);
    margin: 0;
    padding: var(--space-4);
  }

  .chat-error {
    color: var(--error);
  }

  .chat-bubble {
    max-width: 92%;
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-lg);
    align-self: flex-start;
  }

  .chat-bubble-user {
    align-self: flex-end;
    background: var(--primary);
    color: var(--surface);
  }

  .chat-bubble-ai {
    background: var(--surface-2);
    color: var(--text-primary);
    border: 1px solid var(--border);
  }

  .chat-bubble-streaming {
    opacity: 0.95;
  }

  .chat-bubble-tool-call {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .tool-call-label {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .chat-bubble-content {
    font-size: var(--text-sm);
    line-height: 1.6;
    white-space: pre-wrap;
    word-break: break-word;
  }

  .review-card {
    margin-top: var(--space-3);
    padding: var(--space-3);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .review-card-chinese {
    font-size: var(--text-lg);
    font-weight: 600;
    color: var(--text-primary);
  }

  .review-card-pinyin {
    font-size: var(--text-sm);
    color: var(--text-muted);
  }

  .review-card-english {
    font-size: var(--text-sm);
    color: var(--text-secondary);
  }

  .review-card-actions {
    display: flex;
    gap: var(--space-2);
    margin-top: var(--space-1);
  }

  .review-card-badge {
    font-size: var(--text-xs);
    color: var(--success, #16a34a);
    font-weight: 500;
    margin-top: var(--space-1);
  }

  .chat-inline-error {
    font-size: var(--text-xs);
    color: var(--error);
    margin: 0 0 var(--space-2);
  }

  .chat-input-row {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    flex-shrink: 0;
  }

  .chat-input {
    min-height: 60px;
  }

  .chat-actions {
    display: flex;
    justify-content: flex-end;
  }

  .chat-clear {
    margin-top: var(--space-2);
    align-self: flex-start;
  }

  .selection-chip {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: var(--space-1) var(--space-3);
    font-size: var(--text-xs);
    color: var(--text-secondary);
    margin-bottom: var(--space-2);
    flex-shrink: 0;
  }

  .selection-chip-text {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .selection-chip-dismiss {
    background: none;
    border: none;
    cursor: pointer;
    color: var(--text-muted);
    padding: 0 2px;
    font-size: var(--text-xs);
    flex-shrink: 0;
    line-height: 1;
  }

  .selection-chip-dismiss:hover {
    color: var(--text-primary);
  }
</style>
