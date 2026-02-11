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
