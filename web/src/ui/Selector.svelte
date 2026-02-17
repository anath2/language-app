<script lang="ts">
interface Option {
  label: string;
  value: string;
}

interface Props {
  options: Option[];
  value: string;
  onchange?: (value: string) => void;
  class?: string;
}

const {
  options,
  value,
  onchange,
  class: className = '',
}: Props = $props();
</script>

<div class="selector {className}">
  {#each options as option}
    <button
      class="selector-btn"
      class:active={value === option.value}
      onclick={() => onchange?.(option.value)}
    >
      {option.label}
    </button>
  {/each}
</div>

<style>
.selector {
  display: flex;
  gap: var(--space-1);
  background: var(--surface-2);
  border-radius: var(--radius-full);
  min-height: 40px;
  width: 100%;
}

.selector-btn {
  flex: 1;
  background: none;
  border: none;
  border-radius: var(--radius-full);
  color: var(--text-secondary);
  cursor: pointer;
  font-size: var(--text-sm);
  font-weight: 500;
  padding: var(--space-1) var(--space-3);
  transition: all 0.15s ease;
  white-space: nowrap;
}

.selector-btn.active {
  background: var(--surface);
  box-shadow: 0 1px 3px var(--shadow);
  color: var(--text-primary);
}
</style>
