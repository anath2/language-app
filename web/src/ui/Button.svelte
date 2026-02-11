<script lang="ts">
interface Props {
  variant?: 'primary' | 'secondary' | 'ghost';
  size?: 'xs' | 'sm' | 'md' | 'lg';
  shape?: 'default' | 'pill';
  iconOnly?: boolean;
  type?: 'button' | 'submit' | 'reset';
  disabled?: boolean;
  loading?: boolean;
  onclick?: (e: MouseEvent) => void;
  class?: string;
  title?: string;
  id?: string;
  ariaLabel?: string;
  children?: import('svelte').Snippet;
}

const {
  variant = 'primary',
  size = 'md',
  shape = 'default',
  iconOnly = false,
  type = 'button',
  disabled = false,
  loading = false,
  onclick,
  class: className = '',
  title,
  id,
  ariaLabel,
  children,
}: Props = $props();

const variantClasses: Record<string, string> = {
  primary: 'btn-primary',
  secondary: 'btn-secondary',
  ghost: 'btn-ghost',
};

const sizeClasses: Record<string, string> = {
  xs: 'btn-xs',
  sm: 'btn-sm',
  md: 'btn-md',
  lg: 'btn-lg',
};
</script>

<button
  {id}
  {type}
  {title}
  class="btn {variantClasses[variant]} {sizeClasses[size]} {shape === 'pill' ? 'btn-pill' : ''} {iconOnly ? 'btn-icon-only' : ''} {className}"
  disabled={disabled || loading}
  aria-label={ariaLabel}
  onclick={onclick}
>
  {#if loading}
    <span class="spinner spinner-sm"></span>
  {:else if children}
    {@render children()}
  {/if}
</button>

<style>

.btn {
  background: var(--surface);
  color: var(--text-secondary);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  transition: all 0.2s ease;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-2);
}

.btn-xs {
  padding: var(--space-1) var(--space-3);
  font-size: var(--text-xs);
}

.btn-sm {
  padding: var(--space-2) var(--space-4);
  font-size: var(--text-sm);
}

.btn-md {
  padding: var(--space-3) var(--space-4);
  font-size: var(--text-md);
}

.btn-lg {
  padding: var(--space-4) var(--space-8);
  font-size: var(--text-lg);
}

.btn:hover:not(:disabled) {
  background: var(--surface-2);
  color: var(--text-primary);
}

.btn-primary {
  background: var(--primary);
  color: var(--surface);
  border-color: var(--primary);
}

.btn-primary:hover:not(:disabled) {
  background: var(--primary-dark);
  color: var(--surface);
  border-color: var(--primary-dark);
}

.btn-secondary {
  background: var(--background-alt);
  color: var(--text-secondary);
  border-color: var(--border);
}

.btn-secondary:hover:not(:disabled) {
  background: var(--primary);
  color: var(--surface);
  border-color: var(--primary);
}

.btn-ghost {
  background: transparent;
  color: var(--text-secondary);
  border-color: transparent;
}

.btn-ghost:hover:not(:disabled) {
  background: var(--surface-2);
  color: var(--text-primary);
}

.btn-pill {
  border-radius: var(--radius-full);
}

.btn-icon-only {
  padding: var(--space-2);
}

.btn-icon-only.btn-xs {
  padding: var(--space-1);
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>