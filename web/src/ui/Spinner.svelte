<script lang="ts">
interface Props {
  size?: 'sm' | 'md' | 'lg';
  variant?: 'light' | 'dark';
}

const { size = 'md', variant = 'light' }: Props = $props();

const sizeClasses: Record<string, string> = {
  sm: 'spinner-sm',
  md: 'spinner-md',
  lg: 'spinner-lg',
};

const variantClass = variant === 'dark' ? 'spinner-dark' : '';
</script>

<span class="spinner {sizeClasses[size]} {variantClass}"></span>

<style>
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
</style>
