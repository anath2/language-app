<script lang="ts">
import { ExternalLink, X } from '@lucide/svelte';
import Button from '@/ui/Button.svelte';
import Card from '@/ui/Card.svelte';
import type { DiscoveryArticle } from '../types';

interface Props {
  article: DiscoveryArticle;
  onDismiss: (id: string) => void;
}

const { article, onDismiss }: Props = $props();

function difficultyLabel(score: number): string {
  if (score < 0.3) return 'Easy';
  if (score < 0.5) return 'Moderate';
  if (score < 0.7) return 'Challenging';
  return 'Hard';
}

function difficultyClass(score: number): string {
  if (score < 0.3) return 'diff-easy';
  if (score < 0.5) return 'diff-moderate';
  if (score < 0.7) return 'diff-challenging';
  return 'diff-hard';
}

function hostname(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, '');
  } catch {
    return url;
  }
}
</script>

<Card padding="4" class="article-card">
  <div class="card-top">
    <div class="card-meta">
      <span class="source">{hostname(article.url)}</span>
      <span class="difficulty-badge {difficultyClass(article.difficulty_score)}">
        {difficultyLabel(article.difficulty_score)}
      </span>
    </div>
    {#if article.status === 'new'}
      <Button variant="ghost" size="xs" iconOnly ariaLabel="Dismiss" onclick={() => onDismiss(article.id)}>
        <X size={14} />
      </Button>
    {/if}
  </div>

  <h3 class="article-title">
    {article.title || 'Untitled article'}
  </h3>

  {#if article.summary}
    <p class="article-summary">{article.summary}</p>
  {/if}

  <div class="word-stats">
    <span class="stat known">{article.known_words} known</span>
    <span class="stat learning">{article.learning_words} learning</span>
    <span class="stat unknown">{article.unknown_words} unknown</span>
  </div>

  <div class="card-actions">
    <a href={article.url} target="_blank" rel="noopener noreferrer" class="link-btn">
      <ExternalLink size={14} />
      <span>Open</span>
    </a>
  </div>
</Card>

<style>
  :global(.article-card) {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .card-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .card-meta {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }

  .source {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .difficulty-badge {
    font-size: var(--text-xs);
    font-weight: 600;
    padding: 2px var(--space-2);
    border-radius: var(--radius-full);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .diff-easy {
    background: rgba(108, 190, 237, 0.15);
    color: #5AA5D6;
  }

  .diff-moderate {
    background: rgba(108, 190, 237, 0.25);
    color: #3D8DB8;
  }

  .diff-challenging {
    background: rgba(228, 198, 189, 0.3);
    color: #B5624D;
  }

  .diff-hard {
    background: rgba(228, 198, 189, 0.5);
    color: #9B3B2A;
  }

  .article-title {
    font-size: var(--text-base);
    font-weight: 600;
    color: var(--text-primary);
    line-height: 1.4;
    margin: 0;
  }

  .article-summary {
    font-size: var(--text-sm);
    color: var(--text-secondary);
    line-height: var(--leading-relaxed);
    margin: 0;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .word-stats {
    display: flex;
    gap: var(--space-3);
    flex-wrap: wrap;
  }

  .stat {
    font-size: var(--text-xs);
    padding: 2px var(--space-2);
    border-radius: var(--radius-sm);
  }

  .stat.known {
    background: rgba(108, 190, 237, 0.12);
    color: var(--vocab-known);
  }

  .stat.learning {
    background: rgba(228, 198, 189, 0.2);
    color: #C47A5A;
  }

  .stat.unknown {
    background: var(--surface-2);
    color: var(--text-muted);
  }

  .card-actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-top: var(--space-1);
  }

  .link-btn {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    font-size: var(--text-sm);
    color: var(--text-secondary);
    text-decoration: none;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-md);
    border: 1px solid var(--border);
    background: transparent;
    transition: all 0.2s ease;
  }

  .link-btn:hover {
    color: var(--text-primary);
    background: var(--surface-2);
  }
</style>
