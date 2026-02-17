<script lang="ts">
interface Props {
  id?: string;
  name?: string;
  value?: string;
  placeholder?: string;
  required?: boolean;
  disabled?: boolean;
  rows?: number;
  class?: string;
  oninput?: (e: Event) => void;
}

let {
  id,
  name,
  value = $bindable(''),
  placeholder,
  required = false,
  disabled = false,
  rows = 4,
  class: className = '',
  oninput,
}: Props = $props();
</script>

<textarea
  {id}
  {name}
  bind:value
  {placeholder}
  {required}
  {disabled}
  {rows}
  class="textarea {className}"
  {oninput}
></textarea>

<style>
.textarea {
  width: 100%;
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  font-size: var(--text-base);
  color: var(--text-primary);
  background: var(--background);
  transition: all 0.2s ease;
  resize: vertical;
}

.textarea:focus {
  outline: none;
  border-color: var(--primary);
  box-shadow: 0 0 0 var(--space-unit) var(--primary-alpha);
}

.textarea:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.textarea::placeholder {
  font-size: var(--text-sm);
  color: var(--text-muted);
}
</style>
