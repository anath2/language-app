<script lang="ts">
interface Props {
  children?: import('svelte').Snippet;
}

const { children }: Props = $props();
</script>

<div class="two-column-layout">
  {#if children}
    {@render children()}
  {/if}
</div>

<style>
  .two-column-layout {
    display: grid;
    grid-template-columns: 1fr 380px;
    gap: 1.5rem;
    align-items: start;
  }

  @media (max-width: 960px) {
    .two-column-layout {
      grid-template-columns: 1fr;
    }
  }
</style>
