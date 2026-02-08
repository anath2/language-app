// Translation store - manages translation list and current translation
// Located in features/translation/stores/

import { deleteRequest, getJson, postJson } from '@/lib/api';
import type {
  CreateTranslationResponse,
  ListTranslationsResponse,
  LoadingState,
  TranslationDetailResponse,
  TranslationSummary,
} from '@/lib/types';

// State
let translations = $state<TranslationSummary[]>([]);
let currentTranslation = $state<TranslationDetailResponse | null>(null);
let loadingState = $state<LoadingState>('idle');
let errorMessage = $state('');

/**
 * Load the list of translations
 */
export async function loadTranslations(limit: number = 20): Promise<void> {
  try {
    const data = await getJson<ListTranslationsResponse>(`/api/translations?limit=${limit}`);
    translations = data.translations || [];
  } catch (error) {
    console.error('Failed to load translations:', error);
    translations = [];
  }
}

/**
 * Load a specific translation by ID
 */
export async function loadTranslation(id: string): Promise<void> {
  loadingState = 'loading';
  errorMessage = '';

  try {
    const data = await getJson<TranslationDetailResponse>(`/api/translations/${id}`);
    currentTranslation = data;
    loadingState = 'idle';
  } catch (error) {
    console.error('Failed to load translation:', error);
    errorMessage = 'Failed to load translation';
    loadingState = 'error';
  }
}

/**
 * Submit a new translation
 */
export async function submitTranslation(text: string): Promise<string | null> {
  if (!text.trim()) return null;

  loadingState = 'loading';
  errorMessage = '';

  try {
    const data = await postJson<CreateTranslationResponse>('/api/translations', {
      input_text: text,
      source_type: 'text',
    });

    // Refresh the list
    await loadTranslations();

    loadingState = 'idle';
    return data.translation_id;
  } catch (error) {
    console.error('Failed to submit translation:', error);
    errorMessage = 'Failed to submit translation';
    loadingState = 'error';
    return null;
  }
}

/**
 * Delete a translation
 */
export async function deleteTranslation(id: string): Promise<boolean> {
  try {
    await deleteRequest(`/api/translations/${id}`);

    // If we're deleting the current translation, clear it
    if (currentTranslation?.id === id) {
      currentTranslation = null;
    }

    // Refresh the list
    await loadTranslations();
    return true;
  } catch (error) {
    console.error('Failed to delete translation:', error);
    return false;
  }
}

/**
 * Clear the current translation
 */
export function clearCurrentTranslation(): void {
  currentTranslation = null;
  loadingState = 'idle';
  errorMessage = '';
}

// Export reactive state
export const translationStore = {
  get translations() {
    return translations;
  },
  get currentTranslation() {
    return currentTranslation;
  },
  get loadingState() {
    return loadingState;
  },
  get errorMessage() {
    return errorMessage;
  },
  loadTranslations,
  loadTranslation,
  submitTranslation,
  deleteTranslation,
  clearCurrentTranslation,
};
