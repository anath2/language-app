<script lang="ts">
interface Props {
  maxWidth?: 'default' | 'content';
  children?: import('svelte').Snippet;
}

const { maxWidth = 'default', children }: Props = $props();

const maxWidthClass = maxWidth === 'content' ? 'max-w-content' : '';
</script>

<div class="page-container {maxWidthClass}">
  {#if children}
    {@render children()}
  {/if}
</div>

<style>
  .page-container {
    max-width: var(--max-width-page);
    margin: 0 auto;
    padding: 1.5rem;
  }

  .max-w-content {
    max-width: var(--max-width-content);
  }

  @media (max-width: 640px) {
    .page-container {
      padding: 1rem;
    }
  }
</style>
