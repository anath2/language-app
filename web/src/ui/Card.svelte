<script lang="ts">
interface Props {
  padding?: 'none' | '3' | '4' | '5' | '6';
  interactive?: boolean;
  shadow?: boolean;
  important?: boolean;
  collapsible?: boolean;
  class?: string;
  id?: string;
  children?: import('svelte').Snippet;
}

const {
  padding = '5',
  interactive = false,
  shadow = false,
  important = false,
  collapsible = false,
  class: className = '',
  id,
  children,
}: Props = $props();

const paddingClasses: Record<string, string> = {
  none: 'card-p-none',
  '3': 'card-p-3',
  '4': 'card-p-4',
  '5': 'card-p-5',
  '6': 'card-p-6',
};
</script>

<div
  {id}
  class="card {paddingClasses[padding]} {interactive ? 'card-interactive' : ''} {shadow ? 'card-shadow' : ''} {important ? 'card-important' : ''} {collapsible ? 'card-collapsible' : ''} {className}"
>
  {#if children}
    {@render children()}
  {/if}
</div>

<style>
.card {
  background: var(--surface);
  border-radius: var(--radius-xl);
  border: 1px solid var(--border);
  box-shadow: 0 1px 3px var(--shadow);
}

.card-p-none {
  padding: 0;
}

.card-p-3 {
  padding: var(--space-3);
}

.card-p-4 {
  padding: var(--space-4);
}

.card-p-5 {
  padding: var(--space-5);
}

.card-p-6 {
  padding: var(--space-6);
}

.card-interactive {
  cursor: pointer;
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.card-interactive:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px var(--shadow);
}

.card-shadow {
  box-shadow: 0 4px 12px var(--shadow), 0 2px 6px rgba(0, 0, 0, 0.04);
}

.card-important {
  border-left: 3px solid var(--primary);
}

.card-collapsible {
  display: block;
}
</style>
