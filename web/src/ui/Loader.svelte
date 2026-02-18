<script lang="ts">
interface Props {
  variant?: 'spin' | 'chat';
  size?: 'sm' | 'md' | 'lg';
  color?: 'light' | 'dark';
}

const { variant = 'spin', size = 'md', color = 'light' }: Props = $props();

const sizeClasses: Record<string, string> = {
  sm: 'spinner-sm',
  md: 'spinner-md',
  lg: 'spinner-lg',
};

const colorClass = $derived(color === 'dark' ? 'spinner-dark' : '');
</script>

{#if variant === 'chat'}
  <span class="chat-dots" aria-label="Loading">
    <span class="dot"></span>
    <span class="dot"></span>
    <span class="dot"></span>
  </span>
{:else}
  <span class="spinner {sizeClasses[size]} {colorClass}"></span>
{/if}

<style>
  /* spin variant */
  .spinner {
    width: var(--space-4);
    height: var(--space-4);
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-top-color: white;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  .spinner-sm {
    width: 14px;
    height: 14px;
    border-width: 2px;
  }

  .spinner-md {
    width: var(--space-5);
    height: var(--space-5);
    border-width: 2px;
  }

  .spinner-lg {
    width: var(--space-8);
    height: var(--space-8);
    border-width: 3px;
  }

  .spinner-dark {
    border-color: rgba(108, 190, 237, 0.3);
    border-top-color: var(--primary);
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* chat variant */
  .chat-dots {
    display: inline-flex;
    align-items: center;
    gap: 5px;
  }

  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--text-muted);
    animation: bounce 1.2s ease-in-out infinite;
  }

  .dot:nth-child(1) { animation-delay: 0ms; }
  .dot:nth-child(2) { animation-delay: 150ms; }
  .dot:nth-child(3) { animation-delay: 300ms; }

  @keyframes bounce {
    0%, 60%, 100% {
      transform: translateY(0);
      opacity: 0.5;
    }
    30% {
      transform: translateY(-5px);
      opacity: 1;
    }
  }
</style>
