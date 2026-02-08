<script lang="ts">
interface Props {
  top?: string;
  children?: import('svelte').Snippet;
}

const { top = '80px', children }: Props = $props();
</script>

<div class="sticky-panel" style="top: {top}">
  {#if children}
    {@render children()}
  {/if}
</div>

<style>
  .sticky-panel {
    position: sticky;
  }

  @media (max-width: 960px) {
    .sticky-panel {
      position: static;
    }
  }
</style>
