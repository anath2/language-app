<script lang="ts">
import type { SegmentResult } from '@/features/translation/types';
import Card from '@/ui/Card.svelte';
import { ChevronDown } from '@lucide/svelte';

const {
  results,
}: {
  results: SegmentResult[];
} = $props();

let showDetails = $state(false);
</script>

{#if results.length > 0}
  <Card padding="4" collapsible class="table-card">
    <button class="table-toggle" onclick={() => (showDetails = !showDetails)}>
      <h3 class="table-title">Translation Details</h3>
      <span class="toggle-icon" class:open={showDetails}>
        <ChevronDown size={18} />
      </span>
    </button>
    {#if showDetails}
      <div class="table-wrap">
        <table class="details-table">
          <thead>
            <tr>
              <th>Chinese</th>
              <th>Pinyin</th>
              <th>English</th>
            </tr>
          </thead>
          <tbody>
            {#each results as item}
              <tr>
                <td class="cell-chinese">{item.segment}</td>
                <td class="cell-pinyin">{item.pinyin}</td>
                <td class="cell-english">{item.english}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </Card>
{/if}

<style>
  :global(.table-card) {
    margin-top: var(--space-4);
  }

  .table-toggle {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    background: none;
    border: none;
    cursor: pointer;
    text-align: left;
    padding: 0;
    font-family: var(--font-body);
  }

  .table-title {
    font-size: var(--text-base);
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }

  .toggle-icon {
    color: var(--text-muted);
    transition: transform 0.2s ease;
    display: flex;
    align-items: center;
  }

  .toggle-icon.open {
    transform: rotate(180deg);
  }

  .table-wrap {
    margin-top: var(--space-3);
    overflow-x: auto;
  }

  .details-table {
    width: 100%;
    text-align: left;
    border-collapse: collapse;
  }

  .details-table th {
    padding: var(--space-2);
    font-size: var(--text-xs);
    font-weight: 600;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    border-bottom: 1px solid var(--border);
  }

  .details-table td {
    padding: var(--space-2);
    font-size: var(--text-sm);
    border-bottom: 1px solid var(--background-alt);
  }

  .details-table tbody tr {
    transition: background 0.1s ease;
  }

  .details-table tbody tr:hover {
    background: var(--background-alt);
  }

  .cell-chinese {
    font-family: var(--font-chinese);
    font-size: var(--text-chinese);
    color: var(--text-primary);
  }

  .cell-pinyin {
    color: var(--text-secondary);
  }

  .cell-english {
    color: var(--secondary-dark);
  }
</style>
