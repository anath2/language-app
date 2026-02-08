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
