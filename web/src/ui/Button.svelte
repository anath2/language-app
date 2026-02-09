<script lang="ts">
interface Props {
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost';
  size?: 'sm' | 'md' | 'lg';
  type?: 'button' | 'submit' | 'reset';
  disabled?: boolean;
  loading?: boolean;
  onclick?: (e: MouseEvent) => void;
  children?: import('svelte').Snippet;
}

const {
  variant = 'primary',
  size = 'md',
  type = 'button',
  disabled = false,
  loading = false,
  onclick,
  children,
}: Props = $props();

const variantClasses: Record<string, string> = {
  primary: 'btn-primary',
  secondary: 'btn-secondary',
  danger: 'btn-danger',
  ghost: 'btn-ghost',
};

const sizeClasses: Record<string, string> = {
  sm: 'btn-sm',
  md: '',
  lg: 'btn-lg',
};
</script>

<button
  {type}
  class="btn {variantClasses[variant]} {sizeClasses[size]}"
  disabled={disabled || loading}
  onclick={onclick}
>
  {#if loading}
    <span class="spinner spinner-sm"></span>
  {/if}
  {#if children}
    {@render children()}
  {/if}
</button>
