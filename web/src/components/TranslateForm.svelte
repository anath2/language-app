<script lang="ts">
  import type { ExtractTextResponse } from "../lib/types";

  let { onSubmit, loading }: {
    onSubmit: (text: string) => void;
    loading: boolean;
  } = $props();

  let textInput = $state("");
  let ocrFile = $state<File | null>(null);
  let ocrPreviewUrl = $state("");
  let ocrFileName = $state("");
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
    ocrFileName = "";
    if (ocrPreviewUrl) {
      URL.revokeObjectURL(ocrPreviewUrl);
    }
    ocrPreviewUrl = "";
  }

  async function extractTextFromImage() {
    if (!ocrFile) return;
    ocrLoading = true;
    try {
      const formData = new FormData();
      formData.append("file", ocrFile);
      const res = await fetch("/extract-text", {
        method: "POST",
        body: formData,
      });
      if (!res.ok) {
        const data = (await res.json()) as { detail?: string };
        throw new Error(data?.detail || "OCR failed");
      }
      const data = (await res.json()) as ExtractTextResponse;
      textInput = data.text || "";
      clearPreview();
    } catch (error) {
      console.error("OCR failed:", error);
    } finally {
      ocrLoading = false;
    }
  }
</script>

<div class="input-card p-5 h-[100%]">
  <form onsubmit={(e: SubmitEvent) => { e.preventDefault(); onSubmit(textInput.trim()); if (textInput.trim()) textInput = ""; }}>
    <div class="mb-4">
      <label for="text" class="block font-medium mb-1.5" style="color: var(--text-primary); font-size: var(--text-sm);">
        Chinese Text
      </label>
      <textarea
        id="text"
        name="text"
        rows="5"
        placeholder="Enter Chinese text here, e.g., 你好世界"
        required
        class="textarea-main w-full px-3 py-2.5 resize-y"
        bind:value={textInput}
      ></textarea>
    </div>
    <button type="submit" class="btn-primary inline-flex items-center justify-center gap-1.5 px-4 py-2 min-w-[110px]" disabled={loading}>
      {#if loading}
        <span class="spinner"></span>
        Translating...
      {:else}
        Translate
      {/if}
    </button>
  </form>

  <div class="section-divider my-5">
    <span>or extract from image</span>
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
      <div>
        <svg class="mx-auto h-8 w-8 mb-2" style="color: var(--text-muted);" stroke="currentColor" fill="none" viewBox="0 0 48 48">
          <path d="M28 8H12a4 4 0 00-4 4v20m32-12v8m0 0v8a4 4 0 01-4 4H12a4 4 0 01-4-4v-4m32-4l-3.172-3.172a4 4 0 00-5.656 0L28 28M8 32l9.172-9.172a4 4 0 015.656 0L28 28m0 0l4 4m4-24h8m-4-4v8m-12 4h.02" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
        <p class="font-medium" style="color: var(--text-secondary); font-size: var(--text-sm);">
          Drop an image here or click to upload
        </p>
        <p class="mt-0.5" style="color: var(--text-muted); font-size: var(--text-xs);">
          PNG, JPG, JPEG, WebP, GIF (max 5MB)
        </p>
        <button class="btn-secondary mt-3 px-3 py-1.5 inline-flex items-center gap-1.5" type="button" onclick={() => document.getElementById("image-input")?.click()}>
          Choose File
        </button>
      </div>
    {:else}
      <div>
        <img src={ocrPreviewUrl} alt="Preview" class="max-h-32 mx-auto rounded-md shadow-sm" />
        <p class="mt-2 font-medium" style="color: var(--text-primary); font-size: var(--text-sm);">{ocrFileName}</p>
        <div class="mt-2 flex items-center justify-center gap-2">
          <button type="button" class="btn-secondary px-3 py-1.5 inline-flex items-center gap-1.5" onclick={extractTextFromImage} disabled={ocrLoading}>
            {#if ocrLoading}
              <span class="spinner" style="border-color: rgba(99, 110, 114, 0.3); border-top-color: var(--text-secondary);"></span>
              Extracting...
            {:else}
              Extract Text
            {/if}
          </button>
          <button type="button" class="hover:underline" style="color: var(--primary); font-size: var(--text-xs);" onclick={clearPreview}>
            Remove
          </button>
        </div>
      </div>
    {/if}
  </div>
</div>
