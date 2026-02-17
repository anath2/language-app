<script lang="ts">
interface Props {
  message: string;
}

const { message }: Props = $props();
</script>

<div class="error-message">
  {message}
</div>

<style>
  .error-message {
    background: var(--error-bg);
    color: var(--error);
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-lg);
    font-size: var(--text-sm);
    border: 1px solid var(--error-border);
  }
</style>
