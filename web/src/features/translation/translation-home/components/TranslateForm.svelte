<script lang="ts">
import type { ExtractTextResponse } from '@/features/translation/types';
import Button from '@/ui/Button.svelte';
import { Languages, Camera, X, LoaderCircle } from '@lucide/svelte';

interface TranslateFormProps {
  onSubmit: (text: string) => void;
  loading: boolean;
}

const { onSubmit, loading }: TranslateFormProps = $props();

let textInput = $state('');
let ocrFile = $state<File | null>(null);
let ocrPreviewUrl = $state('');
let ocrFileName = $state('');
let ocrLoading = $state(false);

let fileInputEl = $state<HTMLInputElement | null>(null);

function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement | null;
  const [file] = input?.files || [];
  if (!file) return;
  ocrFile = file;
  ocrFileName = file.name;
  ocrPreviewUrl = URL.createObjectURL(file);
  extractTextFromImage();
}

function clearPreview() {
  ocrFile = null;
  ocrFileName = '';
  if (ocrPreviewUrl) {
    URL.revokeObjectURL(ocrPreviewUrl);
  }
  ocrPreviewUrl = '';
  if (fileInputEl) {
    fileInputEl.value = '';
  }
}

async function extractTextFromImage() {
  if (!ocrFile) return;
  ocrLoading = true;
  try {
    const formData = new FormData();
    formData.append('file', ocrFile);
    const res = await fetch('/extract-text', {
      method: 'POST',
      body: formData,
    });
    if (!res.ok) {
      const data = (await res.json()) as { detail?: string };
      throw new Error(data?.detail || 'OCR failed');
    }
    const data = (await res.json()) as ExtractTextResponse;
    textInput = data.text || '';
    clearPreview();
  } catch (error) {
    console.error('OCR failed:', error);
  } finally {
    ocrLoading = false;
  }
}
</script>

<div class="translation-form-header">
  <h2>Hello! <span class="font-chinese">你好! こんにちは!</span></h2>
  <h4>Type anything you want to learn</h4>
</div>

<form class="unified-input"
  onsubmit={(e: SubmitEvent) => {
    e.preventDefault();
    const trimmed = textInput.trim();
    if (trimmed) {
      onSubmit(trimmed);
      textInput = '';
    }
  }}
>
  <textarea
    id="text"
    name="text"
    aria-label="Type something, e.g., 你好世界"
    required
    class="textarea-main"
    rows={6}
    placeholder="Type something, e.g., 你好世界"
    bind:value={textInput}
  ></textarea>

  {#if ocrPreviewUrl}
    <div class="ocr-preview-row">
      <img src={ocrPreviewUrl} alt="Preview" class="ocr-thumbnail" />
      <span class="ocr-info">
        <span class="ocr-filename">{ocrFileName}</span>
        {#if ocrLoading}
          <span class="ocr-status">
            <LoaderCircle size={14} class="spinner-icon" />
            Extracting…
          </span>
        {/if}
      </span>
      <button type="button" class="ocr-dismiss" onclick={clearPreview} aria-label="Remove image">
        <X size={16} />
      </button>
    </div>
  {/if}

  <div class="form-action-bar">
    <input
      type="file"
      id="image-input"
      name="file"
      accept=".png,.jpg,.jpeg,.webp,.gif"
      class="hidden"
      bind:this={fileInputEl}
      onchange={handleFileChange}
    />
    <Button size="sm" variant="ghost" onclick={() => fileInputEl?.click()}>
      <Camera size={20} />
    </Button>
    <Button type="submit" size="sm" variant="primary" disabled={loading}>
      <Languages size={20} />
    </Button>
  </div>
</form>

<style>
  .translation-form-header {
    margin-bottom: var(--space-4);
  }

  .translation-form-header h2 {
    font-size: var(--text-2xl);
    font-weight: 600;
    color: var(--text-primary);
    margin-bottom: var(--space-2);
  }

  .translation-form-header h4 {
    font-size: var(--text-base);
    color: var(--text-primary);
    font-weight: 400;
  }

  .font-chinese {
    font-family: var(--font-chinese);
  }

  .unified-input {
    display: flex;
    flex-direction: column;
    border: None;
    border-radius: var(--radius-lg);
    background: var(--surface);
    overflow: hidden;
    transition: border-color 0.2s ease, box-shadow 0.2s ease;
    border: 1px solid var(--border);
  }


  .textarea-main {
    width: 100%;
    padding: var(--space-3) var(--space-4);
    border: none;
    font-size: var(--text-base);
    line-height: var(--leading-normal);
    resize: none;
    min-height: 100px;
    background: transparent;
    color: var(--text-primary);
    box-shadow: None;
  }

  .textarea-main::placeholder {
    font-size: var(--text-secondary);
    color: var(--text-secondary);
  }

  .textarea-main:focus {
    outline: none;
    box-shadow: none;
    border: none;
  }

  /* OCR preview row */
  .ocr-preview-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-4);
    border-top: 1px solid var(--border);
    background: var(--surface-2);
  }

  .ocr-thumbnail {
    height: 48px;
    width: auto;
    border-radius: var(--radius-sm);
    object-fit: cover;
  }

  .ocr-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
    flex: 1;
    min-width: 0;
  }

  .ocr-filename {
    font-size: var(--text-sm);
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .ocr-status {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }

  .ocr-status :global(.spinner-icon) {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  .ocr-dismiss {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-1);
    border: none;
    background: transparent;
    color: var(--text-muted);
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: color 0.15s ease, background 0.15s ease;
  }

  .ocr-dismiss:hover {
    color: var(--text-primary);
    background: var(--surface);
  }

  /* Action bar */
  .form-action-bar {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-2);
  }

  .hidden {
    display: none;
  }
</style>
