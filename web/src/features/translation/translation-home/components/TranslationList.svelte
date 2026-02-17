<script lang="ts">
import type { TranslationStatus, TranslationSummary } from '@/features/translation/types';
import { formatTimeAgo } from '@/lib/utils';
import Button from '@/ui/Button.svelte';
import { Trash2 } from '@lucide/svelte';

const {
  translations,
  onSelect,
  onDelete,
}: {
  translations: TranslationSummary[];
  onSelect: (id: string) => void;
  onDelete: (id: string) => void;
} = $props();

const statusLabels: Record<TranslationStatus, string> = {
  pending: 'Pending',
  processing: 'Processing',
  completed: 'Completed',
  failed: 'Failed',
};
</script>

<div class="space-y-3">
  {#if translations.length === 0}
    <div class="text-center py-8">
      <p class="italic" style="color: var(--text-muted); font-size: var(--text-sm);">No translations yet</p>
      <p class="mt-1" style="color: var(--text-muted); font-size: var(--text-xs);">Submit text above to start</p>
    </div>
  {:else}
    {#each translations as translation}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class={`job-card ${translation.status}`} onclick={() => onSelect(translation.id)}>
        <div class="job-header">
          <div class="job-status">
            <span class="job-status-icon"></span>
            <span style="color: var(--text-secondary);">{statusLabels[translation.status]}</span>
          </div>
          <span class="job-time">{formatTimeAgo(translation.created_at)}</span>
        </div>
        <div class="job-preview">{translation.input_preview}</div>
        {#if translation.full_translation_preview}
          <div class="job-translation-preview">"{translation.full_translation_preview}"</div>
        {/if}
        {#if translation.status === "processing" && translation.total_segments}
          <div class="job-progress">
            <div class="job-progress-fill" style={`width: ${(translation.segment_count! / translation.total_segments!) * 100}%`}></div>
          </div>
        {/if}
        <div class="job-footer">
          <span class="job-segments-count">
            {#if translation.segment_count !== null && translation.total_segments !== null}
              {translation.segment_count} / {translation.total_segments} segments
            {:else if translation.status === "completed" && translation.segment_count}
              {translation.segment_count} segments
            {/if}
          </span>
          <Button
            variant="ghost"
            size="xs"
            iconOnly
            class="job-delete-btn"
            title="Delete"
            ariaLabel="Delete translation"
            onclick={(e: MouseEvent) => {
              e.stopPropagation();
              onDelete(translation.id);
            }}
          >
            <Trash2 size={14} />
          </Button>
        </div>
      </div>
    {/each}
  {/if}
</div>

<style>
  .job-card {
    position: relative;
    background: var(--surface);
    border-left: 4px solid var(--pastel-4);
    padding: calc(var(--space-unit) * 3.5) var(--space-4);
    border-radius: 0 var(--radius-lg) var(--radius-lg) 0;
    cursor: pointer;
    transition: transform 0.15s ease, box-shadow 0.15s ease;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  }

  .job-card:hover {
    transform: translateX(4px);
    box-shadow: -4px 4px 0 var(--pastel-2), 0 2px 6px rgba(0, 0, 0, 0.08);
  }

  .job-card.pending { border-left-color: var(--pastel-4); }
  .job-card.processing { border-left-color: var(--pastel-3); }
  .job-card.completed { border-left-color: var(--pastel-1); }
  .job-card.failed { border-left-color: var(--error); }

  .job-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--space-2);
  }

  .job-status {
    display: flex;
    align-items: center;
    gap: calc(var(--space-unit) * 1.5);
    font-size: var(--text-xs);
    font-weight: 500;
  }

  .job-status-icon {
    width: calc(var(--space-unit) * 2);
    height: calc(var(--space-unit) * 2);
    border-radius: 50%;
    background: var(--pastel-4);
  }

  .job-card.pending .job-status-icon {
    background: transparent;
    border: 2px solid var(--pastel-4);
  }

  .job-card.processing .job-status-icon {
    background: var(--pastel-3);
    animation: pulse 1.5s ease-in-out infinite;
  }

  .job-card.completed .job-status-icon {
    background: var(--success);
  }

  .job-card.failed .job-status-icon {
    background: var(--error);
  }

  .job-time {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .job-preview {
    font-family: var(--font-chinese);
    font-size: var(--text-sm);
    color: var(--text-primary);
    line-height: var(--leading-normal);
    margin-bottom: var(--space-2);
    overflow: hidden;
    text-overflow: ellipsis;
    display: -webkit-box;
    line-clamp: 2;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
  }

  .job-translation-preview {
    font-size: var(--text-xs);
    color: var(--text-secondary);
    font-style: italic;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    margin-bottom: var(--space-2);
  }

  .job-progress {
    height: calc(var(--space-unit) * 1);
    background: var(--pastel-7);
    border-radius: calc(var(--space-unit) * 0.5);
    overflow: hidden;
    margin-top: var(--space-2);
  }

  .job-progress-fill {
    height: 100%;
    background: linear-gradient(90deg, var(--pastel-3), var(--pastel-1));
    transition: width 0.3s ease-out;
    border-radius: calc(var(--space-unit) * 0.5);
  }

  .job-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: var(--space-2);
  }

  .job-segments-count {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  :global(.job-delete-btn) {
    padding: var(--space-1);
    opacity: 0;
    transition: opacity 0.15s ease, color 0.15s ease;
  }

  .job-card:hover :global(.job-delete-btn) {
    opacity: 1;
  }

  :global(.job-delete-btn:hover) {
    color: var(--error);
  }
</style>
