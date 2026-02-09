<script lang="ts">
interface Props {
  icon?: string;
  title: string;
  description?: string;
  children?: import('svelte').Snippet;
}

const { icon = '📭', title, description, children }: Props = $props();
</script>

<div class="empty-state">
  <div class="empty-state-icon">{icon}</div>
  <div class="empty-state-title">{title}</div>
  {#if description}
    <div class="empty-state-text">{description}</div>
  {/if}
  {#if children}
    {@render children()}
  {/if}
</div>
