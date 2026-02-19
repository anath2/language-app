import {
  deletePreference,
  dismissArticle,
  listArticles,
  listPreferences,
  savePreference,
  triggerRun,
} from '../api';
import type { DiscoveryArticle, DiscoveryPreference } from '../types';

class DiscoveryStore {
  articles = $state<DiscoveryArticle[]>([]);
  total = $state(0);
  preferences = $state<DiscoveryPreference[]>([]);
  loadingArticles = $state(false);
  loadingPreferences = $state(false);
  runningDiscovery = $state(false);
  errorMessage = $state('');
  statusFilter = $state('new');

  async loadArticles(status?: string, limit = 20, offset = 0): Promise<void> {
    this.loadingArticles = true;
    this.errorMessage = '';
    try {
      const data = await listArticles(status ?? this.statusFilter, limit, offset);
      this.articles = data.articles ?? [];
      this.total = data.total;
    } catch (err) {
      console.error('Failed to load articles:', err);
      this.errorMessage = 'Failed to load articles';
    } finally {
      this.loadingArticles = false;
    }
  }

  async loadPreferences(): Promise<void> {
    this.loadingPreferences = true;
    try {
      this.preferences = await listPreferences();
    } catch (err) {
      console.error('Failed to load preferences:', err);
    } finally {
      this.loadingPreferences = false;
    }
  }

  async addPreference(topic: string, weight = 1.0): Promise<void> {
    try {
      const pref = await savePreference(topic, weight);
      this.preferences = [...this.preferences.filter((p) => p.topic !== topic), pref];
    } catch (err) {
      console.error('Failed to save preference:', err);
    }
  }

  async removePreference(id: string): Promise<void> {
    try {
      await deletePreference(id);
      this.preferences = this.preferences.filter((p) => p.id !== id);
    } catch (err) {
      console.error('Failed to delete preference:', err);
    }
  }

  async dismiss(id: string): Promise<void> {
    try {
      await dismissArticle(id);
      this.articles = this.articles.map((a) =>
        a.id === id ? { ...a, status: 'dismissed' as const } : a
      );
      if (this.statusFilter === 'new') {
        this.articles = this.articles.filter((a) => a.id !== id);
      }
    } catch (err) {
      console.error('Failed to dismiss article:', err);
    }
  }

  async triggerDiscovery(): Promise<void> {
    this.runningDiscovery = true;
    try {
      await triggerRun();
    } catch (err) {
      console.error('Failed to trigger discovery run:', err);
    } finally {
      this.runningDiscovery = false;
    }
  }

  setStatusFilter(status: string): void {
    this.statusFilter = status;
    void this.loadArticles(status);
  }
}

export const discoveryStore = new DiscoveryStore();
