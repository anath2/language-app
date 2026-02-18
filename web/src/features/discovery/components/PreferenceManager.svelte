<script lang="ts">
import { Plus, X } from '@lucide/svelte';
import Button from '@/ui/Button.svelte';
import { discoveryStore } from '../stores/discoveryStore.svelte';

let newTopic = $state('');
let adding = $state(false);

async function handleAdd() {
  const topic = newTopic.trim();
  if (!topic) return;
  adding = true;
  try {
    await discoveryStore.addPreference(topic);
    newTopic = '';
  } finally {
    adding = false;
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') {
    e.preventDefault();
    void handleAdd();
  }
}
</script>

<div class="pref-manager">
  <h3 class="section-title">Topics</h3>
  <p class="section-hint">Articles are discovered based on these topics.</p>

  <div class="topic-chips">
    {#each discoveryStore.preferences as pref (pref.id)}
      <span class="topic-chip">
        <span class="chip-label">{pref.topic}</span>
        <button
          class="chip-remove"
          aria-label="Remove {pref.topic}"
          onclick={() => discoveryStore.removePreference(pref.id)}
        >
          <X size={12} />
        </button>
      </span>
    {/each}
  </div>

  <div class="add-row">
    <input
      class="topic-input"
      type="text"
      placeholder="Add topic, e.g. technology"
      bind:value={newTopic}
      onkeydown={handleKeydown}
      disabled={adding}
    />
    <Button variant="secondary" size="sm" loading={adding} onclick={handleAdd} disabled={!newTopic.trim()}>
      <Plus size={14} />
      <span>Add</span>
    </Button>
  </div>
</div>

<style>
  .pref-manager {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
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

  .topic-chips {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    min-height: 32px;
  }

  .topic-chip {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-1) var(--space-3);
    background: var(--primary-alpha);
    border: 1px solid var(--primary-light);
    border-radius: var(--radius-full);
    font-size: var(--text-sm);
    color: var(--text-primary);
  }

  .chip-label {
    line-height: 1;
  }

  .chip-remove {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    cursor: pointer;
    color: var(--text-muted);
    padding: 0;
    border-radius: var(--radius-full);
    transition: color 0.15s ease;
  }

  .chip-remove:hover {
    color: var(--text-primary);
  }

  .add-row {
    display: flex;
    gap: var(--space-2);
    align-items: center;
  }

  .topic-input {
    flex: 1;
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    font-size: var(--text-sm);
    color: var(--text-primary);
    background: var(--background);
    transition: border-color 0.2s ease;
  }

  .topic-input:focus {
    outline: none;
    border-color: var(--primary);
  }

  .topic-input:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .topic-input::placeholder {
    color: var(--text-muted);
  }
</style>
