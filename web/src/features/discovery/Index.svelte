<script lang="ts">
import { RefreshCw } from '@lucide/svelte';
import Button from '@/ui/Button.svelte';
import Card from '@/ui/Card.svelte';
import { router } from '@/lib/router.svelte';
import ArticleCard from './components/ArticleCard.svelte';
import PreferenceManager from './components/PreferenceManager.svelte';
import { discoveryStore } from './stores/discoveryStore.svelte';

const STATUS_TABS = [
  { value: 'new', label: 'New' },
  { value: 'imported', label: 'Imported' },
  { value: 'dismissed', label: 'Dismissed' },
];

$effect(() => {
  void discoveryStore.loadPreferences();
  void discoveryStore.loadArticles('new');
});

function handleTabChange(status: string) {
  discoveryStore.setStatusFilter(status);
}

async function handleImport(id: string) {
  const translationId = await discoveryStore.import(id);
  if (translationId) {
    router.navigateTo(translationId);
  }
}

function handleNavigateToTranslation(id: string) {
  router.navigateTo(id);
}
</script>

<div class="discover-layout">
  <aside class="discover-sidebar">
    <Card padding="5">
      <PreferenceManager />
    </Card>

    <Card padding="5">
      <div class="run-section">
        <div class="run-info">
          <h3 class="section-title">Discovery</h3>
          <p class="section-hint">Manually trigger a discovery run to find new articles.</p>
        </div>
        <Button
          variant="secondary"
          size="sm"
          loading={discoveryStore.runningDiscovery}
          onclick={() => discoveryStore.triggerDiscovery()}
        >
          <RefreshCw size={14} />
          <span>Run now</span>
        </Button>
      </div>
    </Card>
  </aside>

  <main class="discover-main">
    <div class="page-header">
      <h2 class="page-title">Explore</h2>
      <p class="page-subtitle">Chinese articles matched to your learning level</p>
    </div>

    <div class="status-tabs">
      {#each STATUS_TABS as tab}
        <button
          class="tab-btn {discoveryStore.statusFilter === tab.value ? 'tab-active' : ''}"
          onclick={() => handleTabChange(tab.value)}
        >
          {tab.label}
        </button>
      {/each}
    </div>

    {#if discoveryStore.loadingArticles}
      <div class="loading-state">
        <div class="spinner spinner-dark"></div>
        <p>Loading articles…</p>
      </div>
    {:else if discoveryStore.errorMessage}
      <div class="empty-state">
        <p class="error-text">{discoveryStore.errorMessage}</p>
      </div>
    {:else if discoveryStore.articles.length === 0}
      <div class="empty-state">
        <p class="empty-text">
          {#if discoveryStore.statusFilter === 'new'}
            No articles yet. Add topics and run discovery to find articles.
          {:else if discoveryStore.statusFilter === 'imported'}
            No imported articles yet.
          {:else}
            No dismissed articles.
          {/if}
        </p>
      </div>
    {:else}
      <div class="articles-grid">
        {#each discoveryStore.articles as article (article.id)}
          <ArticleCard
            {article}
            onDismiss={(id) => discoveryStore.dismiss(id)}
            onImport={handleImport}
            onNavigateToTranslation={handleNavigateToTranslation}
          />
        {/each}
      </div>
    {/if}
  </main>
</div>

<style>
  .discover-layout {
    display: grid;
    grid-template-columns: 280px 1fr;
    gap: var(--space-6);
    align-items: start;
  }

  .discover-sidebar {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    position: sticky;
    top: 80px;
  }

  .run-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .run-info {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .section-title {
    font-size: var(--text-base);
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }

  .section-hint {
    font-size: var(--text-sm);
    color: var(--text-muted);
    margin: 0;
  }

  .discover-main {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
    min-width: 0;
  }

  .page-header {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .page-title {
    font-size: var(--text-2xl);
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }

  .page-subtitle {
    font-size: var(--text-base);
    color: var(--text-secondary);
    margin: 0;
  }

  .status-tabs {
    display: flex;
    gap: var(--space-1);
    border-bottom: 1px solid var(--border);
    padding-bottom: 0;
  }

  .tab-btn {
    padding: var(--space-2) var(--space-4);
    border: none;
    background: transparent;
    font-size: var(--text-sm);
    color: var(--text-secondary);
    cursor: pointer;
    border-bottom: 2px solid transparent;
    margin-bottom: -1px;
    transition: all 0.15s ease;
  }

  .tab-btn:hover {
    color: var(--text-primary);
  }

  .tab-active {
    color: var(--primary);
    border-bottom-color: var(--primary);
    font-weight: 600;
  }

  .articles-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: var(--space-4);
  }

  .loading-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-12) 0;
    color: var(--text-muted);
    font-size: var(--text-sm);
  }

  .empty-state {
    padding: var(--space-12) 0;
    text-align: center;
  }

  .empty-text {
    color: var(--text-muted);
    font-size: var(--text-sm);
  }

  .error-text {
    color: var(--error);
    font-size: var(--text-sm);
  }

  @media (max-width: 960px) {
    .discover-layout {
      grid-template-columns: 1fr;
    }

    .discover-sidebar {
      position: static;
    }
  }
</style>
