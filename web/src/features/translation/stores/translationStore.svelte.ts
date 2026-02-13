// Translation store - manages translation list and current translation

import { deleteRequest, getJson, postJson } from '@/lib/api';
import type {
  CreateTranslationResponse,
  ListTranslationsResponse,
  LoadingState,
  TranslationDetailResponse,
  TranslationSummary,
} from '@/features/translation/types';

class TranslationStore {
  translations = $state<TranslationSummary[]>([]);
  currentTranslation = $state<TranslationDetailResponse | null>(null);
  loadingState = $state<LoadingState>('idle');
  errorMessage = $state('');

  private getCreatedTranslationId(data: CreateTranslationResponse): string | null {
    return data.translation_id ?? null;
  }

  private createOptimisticSummary(id: string, text: string, status?: string): TranslationSummary {
    const trimmed = text.trim();
    const previewLimit = 120;
    const inputPreview =
      trimmed.length > previewLimit ? `${trimmed.slice(0, previewLimit - 3)}...` : trimmed;

    return {
      id,
      created_at: new Date().toISOString(),
      status: (status as TranslationSummary['status']) ?? 'pending',
      source_type: 'text',
      input_preview: inputPreview,
      full_translation_preview: null,
      segment_count: null,
      total_segments: null,
    };
  }

  /**
   * Load the list of translations
   */
  async loadTranslations(limit: number = 20): Promise<void> {
    try {
      const data = await getJson<ListTranslationsResponse>(`/api/translations?limit=${limit}`);
      this.translations = data.translations || [];
    } catch (error) {
      console.error('Failed to load translations:', error);
    }
  }

  /**
   * Load a specific translation by ID
   */
  async loadTranslation(id: string): Promise<void> {
    this.loadingState = 'loading';
    this.errorMessage = '';

    try {
      const data = await getJson<TranslationDetailResponse>(`/api/translations/${id}`);
      this.currentTranslation = data;
      this.loadingState = 'idle';
    } catch (error) {
      console.error('Failed to load translation:', error);
      this.errorMessage = 'Failed to load translation';
      this.loadingState = 'error';
    }
  }

  /**
   * Submit a new translation
   */
  async submitTranslation(text: string): Promise<string | null> {
    if (!text.trim()) return null;

    this.loadingState = 'loading';
    this.errorMessage = '';

    try {
      const data = await postJson<CreateTranslationResponse>('/api/translations', {
        input_text: text,
        source_type: 'text',
      });
      const translationId = this.getCreatedTranslationId(data);
      if (!translationId) {
        throw new Error('Translation created but no ID was returned');
      }

      // Insert immediately so the home list updates without waiting for refetch.
      if (!this.translations.some((translation) => translation.id === translationId)) {
        this.translations = [
          this.createOptimisticSummary(translationId, text, data.status),
          ...this.translations,
        ];
      }

      this.loadingState = 'idle';
      // Keep local list in sync with server fields (counts/status/previews).
      void this.loadTranslations();
      return translationId;
    } catch (error) {
      console.error('Failed to submit translation:', error);
      this.errorMessage = 'Failed to submit translation';
      this.loadingState = 'error';
      return null;
    }
  }

  /**
   * Delete a translation
   */
  async deleteTranslation(id: string): Promise<boolean> {
    try {
      await deleteRequest(`/api/translations/${id}`);

      // If we're deleting the current translation, clear it
      if (this.currentTranslation?.id === id) {
        this.currentTranslation = null;
      }

      // Refresh the list
      await this.loadTranslations();
      return true;
    } catch (error) {
      console.error('Failed to delete translation:', error);
      return false;
    }
  }

  /**
   * Clear the current translation
   */
  clearCurrentTranslation(): void {
    this.currentTranslation = null;
    this.loadingState = 'idle';
    this.errorMessage = '';
  }
}

export const translationStore = new TranslationStore();
