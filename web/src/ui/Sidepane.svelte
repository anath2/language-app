<script lang="ts">
import { X } from '@lucide/svelte';
import Button from '@/ui/Button.svelte';

interface Props {
  open?: boolean;
  onClose?: () => void;
  title?: string;
  position?: 'left' | 'right';
  width?: string;
  class?: string;
  children?: import('svelte').Snippet;
}

const {
  open = false,
  onClose,
  title = '',
  position = 'right',
  width = '380px',
  class: className = '',
  children,
}: Props = $props();

function handleBackdropClick(e: MouseEvent) {
  if (e.target === e.currentTarget && onClose) onClose();
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && onClose) onClose();
}
</script>

{#if open}
  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <div
    class="sidepane-backdrop sidepane-backdrop-{position} {className}"
    role="presentation"
    onkeydown={handleKeydown}
    onclick={handleBackdropClick}
  >
    <div
      class="sidepane-panel sidepane-panel-{position}"
      style="--sidepane-width: {width}"
      role="dialog"
      aria-modal="true"
      aria-label={title || 'Panel'}
      tabindex="-1"
    >
      <header class="sidepane-header">
        {#if title}
          <h2 class="sidepane-title">{title}</h2>
        {/if}
        <Button
          variant="ghost"
          size="xs"
          iconOnly
          ariaLabel="Close panel"
          onclick={onClose}
        >
          <X size={18} />
        </Button>
      </header>
      <div class="sidepane-content">
        {#if children}
          {@render children()}
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .sidepane-backdrop {
    position: fixed;
    inset: 0;
    z-index: 100;
    display: flex;
    align-items: stretch;
    background: rgba(0, 0, 0, 0.25);
    animation: sidepane-fade-in 0.2s ease;
  }

  .sidepane-backdrop-right {
    justify-content: flex-end;
  }

  .sidepane-backdrop-left {
    justify-content: flex-start;
  }

  .sidepane-panel {
    width: var(--sidepane-width, 380px);
    max-width: 100%;
    background: var(--surface);
    display: flex;
    flex-direction: column;
    outline: none;
  }

  .sidepane-panel-right {
    border-left: 1px solid var(--border);
    box-shadow: -4px 0 12px var(--shadow);
    animation: sidepane-slide-in-right 0.25s ease;
  }

  .sidepane-panel-left {
    border-right: 1px solid var(--border);
    box-shadow: 4px 0 12px var(--shadow);
    animation: sidepane-slide-in-left 0.25s ease;
  }

  .sidepane-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    padding: var(--space-4) var(--space-5);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .sidepane-title {
    margin: 0;
    font-size: var(--text-lg);
    font-weight: 600;
    color: var(--text-primary);
  }

  .sidepane-content {
    flex: 1;
    overflow: auto;
    min-height: 0;
  }

  @keyframes sidepane-fade-in {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }

  @keyframes sidepane-slide-in-right {
    from {
      transform: translateX(100%);
    }
    to {
      transform: translateX(0);
    }
  }

  @keyframes sidepane-slide-in-left {
    from {
      transform: translateX(-100%);
    }
    to {
      transform: translateX(0);
    }
  }
</style>
