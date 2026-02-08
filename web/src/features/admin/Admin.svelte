<script lang="ts">
import { onMount } from 'svelte';
import { getJson, postJsonForm } from '../../lib/api';
import type { AdminProfileResponse, ImportProgressResponse, UserProfile } from '../../lib/types';

interface Props {
  onImportSuccess?: () => void;
}

const { onImportSuccess = () => {} }: Props = $props();

// State
let profile = $state<UserProfile | null>(null);
let vocabStats = $state({ known: 0, learning: 0, total: 0 });
let loading = $state(true);
let saving = $state(false);
let importModalOpen = $state(false);
let importing = $state(false);
let message = $state<{ type: 'error' | 'success'; text: string } | null>(null);
let fileInput = $state<HTMLInputElement | null>(null);

// Form fields
let name = $state('');
let email = $state('');
let language = $state('');

async function loadProfile() {
  try {
    const response = await getJson<AdminProfileResponse>('/admin/api/profile');
    profile = response.profile;
    vocabStats = response.vocabStats;

    if (profile) {
      name = profile.name;
      email = profile.email;
      language = profile.language;
    }
  } catch (error) {
    message = { type: 'error', text: 'Failed to load profile' };
  } finally {
    loading = false;
  }
}

async function saveProfile(e: Event) {
  e.preventDefault();
  saving = true;
  message = null;

  try {
    const data = new FormData();
    data.append('name', name);
    data.append('email', email);
    data.append('language', language);

    const response = await postJsonForm<{ profile: UserProfile }>('/admin/api/profile', data);
    profile = response.profile;
    message = { type: 'success', text: 'Profile saved successfully' };
  } catch (error) {
    message = { type: 'error', text: 'Failed to save profile' };
  } finally {
    saving = false;
  }
}

async function handleFileSelect(e: Event) {
  const target = e.target as HTMLInputElement;
  if (!target.files?.length) return;

  const file = target.files[0];
  if (!file.name.endsWith('.json')) {
    message = { type: 'error', text: 'Please select a JSON file' };
    return;
  }

  importing = true;
  message = null;

  try {
    const formData = new FormData();
    formData.append('file', file, file.name);

    const response = await fetch('/admin/progress/import', {
      method: 'POST',
      body: formData,
    });

    const result = (await response.json()) as ImportProgressResponse;

    if (result.success && result.counts) {
      message = {
        type: 'success',
        text: `Import successful! Added ${result.counts.vocab_items} vocabulary items, ${result.counts.srs_state} review states, and ${result.counts.vocab_lookups} lookups.`,
      };
      importModalOpen = false;
      onImportSuccess();
      // Reload profile to update stats
      loadProfile();
    } else {
      throw new Error('Invalid response format');
    }
  } catch (error) {
    message = {
      type: 'error',
      text: error instanceof Error ? error.message : 'Import failed',
    };
  } finally {
    importing = false;
    if (fileInput) fileInput.value = '';
  }
}

function openImportModal() {
  importModalOpen = true;
}

function closeImportModal() {
  importModalOpen = false;
  if (fileInput) fileInput.value = '';
}

async function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && importModalOpen) {
    closeImportModal();
  }
}

onMount(() => {
  loadProfile();
  document.addEventListener('keydown', handleKeydown);
  return () => {
    document.removeEventListener('keydown', handleKeydown);
  };
});
</script>

<div class="space-y-6">
  <!-- Profile Section -->
  <div class="input-card p-6">
    <h2 class="font-semibold mb-4" style="color: var(--text-primary);">User Profile</h2>

    {#if loading}
      <p style="color: var(--text-secondary);">Loading...</p>
    {:else}
      <form onsubmit={saveProfile} class="space-y-4">
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div>
            <label for="name" class="block mb-1" style="color: var(--text-secondary);">Name</label>
            <input
              id="name"
              type="text"
              class="input-field"
              bind:value={name}
              required
              disabled={saving}
            />
          </div>
          <div>
            <label for="email" class="block mb-1" style="color: var(--text-secondary);">Email</label>
            <input
              id="email"
              type="email"
              class="input-field"
              bind:value={email}
              required
              disabled={saving}
            />
          </div>
          <div>
            <label for="language" class="block mb-1" style="color: var(--text-secondary);">Language</label>
            <input
              id="language"
              type="text"
              class="input-field"
              bind:value={language}
              required
              disabled={saving}
            />
          </div>
        </div>

        <button type="submit" class="btn-primary px-6 py-2" disabled={saving}>
          {saving ? "Saving..." : "Save Profile"}
        </button>
      </form>
    {/if}
  </div>

  <!-- Vocabulary Statistics -->
  <div class="input-card p-6">
    <h2 class="font-semibold mb-4" style="color: var(--text-primary);">Vocabulary Statistics</h2>

    <div class="grid grid-cols-3 gap-4">
      <div class="stat-card">
        <h3 class="text-sm" style="color: var(--text-secondary);">Known</h3>
        <p class="text-2xl font-semibold">{vocabStats.known}</p>
      </div>
      <div class="stat-card">
        <h3 class="text-sm" style="color: var(--text-secondary);">Learning</h3>
        <p class="text-2xl font-semibold">{vocabStats.learning}</p>
      </div>
      <div class="stat-card">
        <h3 class="text-sm" style="color: var(--text-secondary);">Total</h3>
        <p class="text-2xl font-semibold">{vocabStats.total}</p>
      </div>
    </div>
  </div>

  <!-- Progress Import/Export -->
  <div class="input-card p-6">
    <h2 class="font-semibold mb-4" style="color: var(--text-primary);">Progress Sync</h2>

    <div class="space-y-4">
      <div>
        <h3 class="font-medium mb-2" style="color: var(--text-secondary);">Export Progress</h3>
        <p class="text-sm mb-2" style="color: var(--text-secondary);">Download your learning progress as a JSON file</p>
        <a href="/admin/progress/export" class="btn-primary px-4 py-2 inline-block">Export Progress</a>
      </div>

      <div>
        <h3 class="font-medium mb-2" style="color: var(--text-secondary);">Import Progress</h3>
        <p class="text-sm mb-2" style="color: var(--text-secondary);">Upload a progress JSON file to restore your data</p>
        <button class="btn-primary px-4 py-2" onclick={openImportModal}>Import Progress</button>
      </div>
    </div>
  </div>

  <!-- Messages -->
  {#if message}
    <div class="message" class:error={message.type === "error"} class:success={message.type === "success"}>
      {message.text}
    </div>
  {/if}

  <!-- Import Modal -->
  {#if importModalOpen}
    <div class="modal-overlay" onclick={closeImportModal}>
      <div class="modal" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-label="Import Progress">
        <h3 class="font-semibold mb-4" style="color: var(--text-primary);">Import Progress</h3>
        <p class="text-sm mb-4" style="color: var(--text-secondary);">Select a JSON file containing your exported progress.</p>

        <input
          type="file"
          accept=".json"
          bind:this={fileInput}
          onchange={handleFileSelect}
          disabled={importing}
          class="mb-4"
        />

        {#if importing}
          <p style="color: var(--text-secondary);" class="mb-4">Importing...</p>
        {/if}

        <div class="flex gap-2">
          <button
            class="btn-secondary px-4 py-2"
            onclick={closeImportModal}
            disabled={importing}
          >
            Cancel
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .input-field {
    width: 100%;
    padding: 8px 12px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--background-alt);
    color: var(--text-primary);
    transition: all 0.2s ease;
  }

  .input-field:focus {
    outline: none;
    border-color: var(--border-focus);
    box-shadow: 0 0 0 3px rgba(124, 158, 178, 0.15);
  }

  .input-field:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .stat-card {
    background: var(--background-alt);
    padding: 16px;
    border-radius: 8px;
    border: 1px solid var(--border);
  }

  .message {
    padding: 12px 16px;
    border-radius: 6px;
    font-size: 14px;
  }

  .message.error {
    background-color: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(239, 68, 68, 0.3);
    color: #ef4444;
  }

  .message.success {
    background-color: rgba(34, 197, 94, 0.1);
    border: 1px solid rgba(34, 197, 94, 0.3);
    color: #22c55e;
  }

  .modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }

  .modal {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 24px;
    max-width: 400px;
    width: 90%;
  }

  :global(.btn-secondary:disabled) {
    opacity: 0.6;
    cursor: not-allowed;
  }
</style>