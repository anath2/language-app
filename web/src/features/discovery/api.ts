import { deleteRequest, getJson, postJson } from '@/lib/api';
import type {
  DiscoveryArticle,
  DiscoveryPreference,
  ImportArticleResponse,
  ListArticlesResponse,
} from './types';

export async function listPreferences(): Promise<DiscoveryPreference[]> {
  return getJson<DiscoveryPreference[]>('/api/discovery/preferences');
}

export async function savePreference(topic: string, weight: number): Promise<DiscoveryPreference> {
  return postJson<DiscoveryPreference>('/api/discovery/preferences', { topic, weight });
}

export async function deletePreference(id: string): Promise<void> {
  await deleteRequest(`/api/discovery/preferences/${id}`);
}

export async function listArticles(
  status?: string,
  limit = 20,
  offset = 0
): Promise<ListArticlesResponse> {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (status) params.set('status', status);
  return getJson<ListArticlesResponse>(`/api/discovery/articles?${params.toString()}`);
}

export async function getArticle(id: string): Promise<DiscoveryArticle> {
  return getJson<DiscoveryArticle>(`/api/discovery/articles/${id}`);
}

export async function dismissArticle(id: string): Promise<void> {
  await postJson(`/api/discovery/articles/${id}/dismiss`, {});
}

export async function importArticle(id: string): Promise<ImportArticleResponse> {
  return postJson<ImportArticleResponse>(`/api/discovery/articles/${id}/import`, {});
}

export async function triggerRun(): Promise<void> {
  await postJson('/api/discovery/run', {});
}
