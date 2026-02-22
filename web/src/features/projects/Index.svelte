<script lang="ts">
import { translationStore } from '@/features/translation/stores/translationStore.svelte';
import { router } from '@/lib/router.svelte';

const PROJECTS_LIST_LIMIT = 500;

$effect(() => {
  void translationStore.loadTranslations(PROJECTS_LIST_LIMIT);
});

function openTranslation(id: string) {
  router.navigateTo(id);
}

function formatDate(dateIso: string): string {
  const d = new Date(dateIso);
  if (Number.isNaN(d.getTime())) return dateIso;
  return d.toLocaleString();
}
</script>

<section class="projects">
  <h1 class="projects-title">Projects</h1>
  <p class="projects-subtitle">All translations</p>

  <div class="table-wrap">
    <table class="projects-table">
      <thead>
        <tr>
          <th>Title</th>
          <th>Status</th>
          <th>Created</th>
          <th>Preview</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {#if translationStore.translations.length === 0}
          <tr>
            <td colspan="5" class="empty">No translations found.</td>
          </tr>
        {:else}
          {#each translationStore.translations as translation (translation.id)}
            <tr>
              <td>{translation.title || 'Untitled'}</td>
              <td>
                <span class="status status-{translation.status}">
                  {translation.status}
                </span>
              </td>
              <td>{formatDate(translation.created_at)}</td>
              <td class="preview">{translation.input_preview}</td>
              <td class="actions">
                <button class="open-btn" onclick={() => openTranslation(translation.id)}>
                  Open
                </button>
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>
</section>

<style>
  .projects-title {
    margin: 0;
    font-size: var(--text-2xl);
    color: var(--text-primary);
  }

  .projects-subtitle {
    margin: var(--space-1) 0 var(--space-4);
    color: var(--text-secondary);
    font-size: var(--text-sm);
  }

  .table-wrap {
    overflow-x: auto;
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--surface);
  }

  .projects-table {
    width: 100%;
    border-collapse: collapse;
    min-width: 760px;
  }

  .projects-table th,
  .projects-table td {
    padding: var(--space-3);
    border-bottom: 1px solid var(--border);
    text-align: left;
    vertical-align: top;
    font-size: var(--text-sm);
    color: var(--text-primary);
  }

  .projects-table th {
    color: var(--text-secondary);
    font-weight: 600;
    background: var(--surface-2);
  }

  .projects-table tbody tr:last-child td {
    border-bottom: none;
  }

  .preview {
    max-width: 460px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--text-secondary);
  }

  .actions {
    width: 1%;
    white-space: nowrap;
  }

  .open-btn {
    border: 1px solid var(--border);
    background: var(--surface);
    color: var(--text-primary);
    border-radius: var(--radius-sm);
    padding: 0.3rem 0.6rem;
    cursor: pointer;
  }

  .open-btn:hover {
    background: var(--surface-hover);
  }

  .empty {
    color: var(--text-muted);
    text-align: center;
    padding: var(--space-6);
  }

  .status {
    display: inline-block;
    padding: 0.1rem 0.45rem;
    border-radius: 999px;
    font-size: var(--text-xs);
    text-transform: capitalize;
    border: 1px solid var(--border);
    background: var(--surface);
    color: var(--text-secondary);
  }

  .status-completed {
    color: var(--success, #16a34a);
  }

  .status-failed {
    color: var(--error);
  }

  .status-processing,
  .status-pending {
    color: var(--warning, #d97706);
  }
</style>
