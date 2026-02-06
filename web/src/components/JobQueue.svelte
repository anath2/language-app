<script lang="ts">
  import type { JobStatus, JobSummary } from "../lib/types";
  import { formatTimeAgo } from "../lib/utils";

  let { jobs, onExpand, onDelete }: {
    jobs: JobSummary[];
    onExpand: (jobId: string) => void;
    onDelete: (jobId: string) => void;
  } = $props();

  const statusLabels: Record<JobStatus, string> = {
    pending: "Pending",
    processing: "Processing",
    completed: "Completed",
    failed: "Failed",
  };
</script>

<div class="space-y-3">
  {#if jobs.length === 0}
    <div class="text-center py-8">
      <p class="italic" style="color: var(--text-muted); font-size: var(--text-sm);">No translation jobs yet</p>
      <p class="mt-1" style="color: var(--text-muted); font-size: var(--text-xs);">Submit text on the left to start</p>
    </div>
  {:else}
    {#each jobs as job}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class={`job-card ${job.status}`} onclick={() => onExpand(job.id)}>
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
            <div class="job-progress-fill" style={`width: ${(job.segment_count! / job.total_segments!) * 100}%`}></div>
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
            onclick={(e: MouseEvent) => { e.stopPropagation(); onDelete(job.id); }}
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
