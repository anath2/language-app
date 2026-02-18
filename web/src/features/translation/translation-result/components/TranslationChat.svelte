<script lang="ts">
import Button from '@/ui/Button.svelte';
import Sidepane from '@/ui/Sidepane.svelte';
import Loader from '@/ui/Loader.svelte';
import TextArea from '@/ui/TextArea.svelte';
import { getJson, postJson, postJsonStream } from '@/lib/api';
import type {
  ChatMessage,
  ChatListResponse,
  ChatStreamEvent,
} from '@/features/translation/types';

const {
  translationId,
  open,
  onClose,
  selectedSegmentIds = [],
}: {
  translationId: string | null;
  open: boolean;
  onClose: () => void;
  selectedSegmentIds?: string[];
} = $props();

let messages = $state<ChatMessage[]>([]);
let listLoading = $state(false);
let streaming = $state(false);
let streamingContent = $state('');
let inputValue = $state('');
let errorMessage = $state('');
let listError = $state('');

$effect(() => {
  if (!open || !translationId) return;
  void loadMessages();
});

async function loadMessages() {
  if (!translationId) return;
  listLoading = true;
  listError = '';
  try {
    const data = await getJson<ChatListResponse>(
      `/api/translations/${translationId}/chat/list`
    );
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
    message_idx: messages.length,
    role: 'user',
    content: text,
    selected_segment_ids: selectedSegmentIds,
    created_at: new Date().toISOString(),
  };
  messages = [...messages, userMessage];
  inputValue = '';
  errorMessage = '';
  streaming = true;
  streamingContent = '';

  try {
    await postJsonStream<ChatStreamEvent>(
      `/api/translations/${translationId}/chat/new`,
      { message: text, selected_segment_ids: selectedSegmentIds },
      (event) => {
        if (event.type === 'chunk' && event.delta != null) {
          streamingContent += event.delta;
        } else if (event.type === 'complete') {
          const aiMessage: ChatMessage = {
            id: event.message_id ?? `ai-${Date.now()}`,
            chat_id: '',
            translation_id: translationId,
            message_idx: messages.length + 1,
            role: 'ai',
            content: event.content ?? streamingContent,
            selected_segment_ids: [],
            created_at: new Date().toISOString(),
          };
          messages = [...messages, aiMessage];
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
  }
}

async function clearChat() {
  if (!translationId || streaming) return;
  try {
    await postJson<{ ok: boolean }>(
      `/api/translations/${translationId}/chat/clear`,
      {}
    );
    messages = [];
    errorMessage = '';
  } catch (e) {
    errorMessage = e instanceof Error ? e.message : 'Failed to clear chat';
  }
}
</script>

<Sidepane title="Chat" {open} onClose={onClose} width="400px">
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
          <div class="chat-bubble chat-bubble-{msg.role}">
            <div class="chat-bubble-content">{msg.content}</div>
          </div>
        {/each}
        {#if streaming && !streamingContent}
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
</Sidepane>

<style>
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

  .chat-bubble-content {
    font-size: var(--text-sm);
    line-height: 1.5;
    white-space: pre-wrap;
    word-break: break-word;
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
</style>
