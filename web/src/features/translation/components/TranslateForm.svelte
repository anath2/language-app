<script lang="ts">
import type { ExtractTextResponse } from '@/lib/types';
import Button from '@/ui/Button.svelte';
import { Languages, Camera } from '@lucide/svelte';
/** Props interface for TranslateForm */
interface TranslateFormProps {
  /** Function to call when translation form is submitted */
  onSubmit: (text: string) => void;
  /** Whether translation is currently loading */
  loading: boolean;
}

const { onSubmit, loading }: TranslateFormProps = $props();

let textInput = $state('');
let ocrFile = $state<File | null>(null);
let ocrPreviewUrl = $state('');
let ocrFileName = $state('');
let ocrLoading = $state(false);

function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement | null;
  const [file] = input?.files || [];
  if (!file) return;
  ocrFile = file;
  ocrFileName = file.name;
  ocrPreviewUrl = URL.createObjectURL(file);
}

function clearPreview() {
  ocrFile = null;
  ocrFileName = '';
  if (ocrPreviewUrl) {
    URL.revokeObjectURL(ocrPreviewUrl);
  }
  ocrPreviewUrl = '';
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

<div class="translation-form-header mb-4">
  <h2>Hello! 你好! こんにちは!</h2>
  <h4>Type anything you want to learn</h4>
</div>
<div class="translation-input-card">
  <form class="translation-text-input-card-form" 
    onsubmit={
    (e: SubmitEvent) => { e.preventDefault(); onSubmit(textInput.trim()); if (textInput.trim()) textInput = ""; }}>
    <div class="translation-text-input-card-form-content">
      <textarea
        id="text"
        name="text"
        aria-label="Type something, e.g., 你好世界"
        required
        class="textarea-main"
        rows={10}
        placeholder="Type something, e.g., 你好世界"
        bind:value={textInput}
      >
      </textarea>
    </div>
    <div class="translation-submit-button">
    <Button type="submit" size="sm" variant="primary" disabled={loading}>
      <Languages />
    </Button>
    </div>
  </form>

  <div class="section-divider my-5">
    <span>Or extract from image</span>
  </div>

  <div id="drop-zone" class="image-upload-zone p-4 text-center cursor-pointer">
    <input
      type="file"
      id="image-input"
      name="file"
      accept=".png,.jpg,.jpeg,.webp,.gif"
      class="hidden"
      onchange={handleFileChange}
    />
    {#if !ocrPreviewUrl}
      <Button size="sm" variant="secondary" onclick={() => document.getElementById("image-input")?.click()}>
        <Camera /> 
      </Button>
    {:else}
      <img src={ocrPreviewUrl} alt="Preview"  style="max-width: 100%;" />
      <p class="mt-2 font-medium" style="color: var(--text-primary); font-size: var(--text-sm);">{ocrFileName}</p>
      <Button size="sm" variant="primary" onclick={extractTextFromImage} disabled={ocrLoading}>
        Extract Text
      </Button>
      <Button size="sm" variant="danger" onclick={clearPreview}>
        Remove
      </Button>
    {/if}
  </div>
</div>

<style>

  .translation-form-header h2 {
    font-size: var(--text-2xl);
    font-weight: 600;
    color: var(--text-primary);
    margin-bottom: var(--space-2);
  }

  .translation-form-header h4 {
    font-size: var(--text-base);
    color: var(--text-muted);
  }

  .translation-text-input-card-form {
    width: 100%;
    gap: var(--space-6);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .textarea-main {
    width: 100%;
    padding: var(--space-3) var(--space-4);
    border: 2px solid var(--border);
    border-radius: var(--radius-lg);
    font-size: var(--text-base);
    line-height: var(--leading-normal);
    resize: none;
    min-height: 100px;
    transition: all 0.2s ease;
    background: var(--surface);
    color: var(--text-primary);
  }

  .textarea-main::placeholder {
    font-size: var(--text-secondary);
    color: var(--text-secondary);
  }

  .textarea-main:focus {
    outline: none;
    border-color: var(--background-alt);
    border-width: 1px;
    box-shadow: 0 0 0 calc(var(--space-unit) * 3) var(--border-focus-alpha);
  }
  .translation-submit-button {
    align-self: flex-end;
    transform: translate(-20%, -180%);
  }

  .section-divider {
    display: flex;
    align-items: center;
    gap: 1rem;
  }
  .section-divider::before,
  .section-divider::after {
    content: "";
    flex: 1;
    height: 1px;
    background: var(--border);
  }

  .image-upload-zone {
    padding: var(--space-6);
  }
</style>