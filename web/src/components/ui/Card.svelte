<script lang="ts">
interface Props {
  padding?: 'none' | '3' | '4' | '5' | '6';
  interactive?: boolean;
  children?: import('svelte').Snippet;
}

const { padding = '5', interactive = false, children }: Props = $props();

const paddingClasses: Record<string, string> = {
  none: '',
  '3': 'card-p-3',
  '4': 'card-p-4',
  '5': 'card-p-5',
  '6': 'card-p-6',
};
</script>

<div class="card {paddingClasses[padding]} {interactive ? 'card-interactive' : ''}">
  {#if children}
    {@render children()}
  {/if}
</div>
