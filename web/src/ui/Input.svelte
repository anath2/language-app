<script lang="ts">
interface Props {
  type?: 'text' | 'password' | 'email';
  id?: string;
  name?: string;
  value?: string;
  placeholder?: string;
  required?: boolean;
  disabled?: boolean;
  autocomplete?: HTMLInputElement['autocomplete'];
  class?: string;
  oninput?: (e: Event) => void;
}

let {
  type = 'text',
  id,
  name,
  value = $bindable(''),
  placeholder,
  required = false,
  disabled = false,
  autocomplete,
  class: className = '',
  oninput,
}: Props = $props();
</script>

<input
  {type}
  {id}
  {name}
  bind:value
  {placeholder}
  {required}
  {disabled}
  {autocomplete}
  class="input {className}"
  {oninput}
/>

<style>
.input {
  width: 100%;
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  font-size: var(--text-base);
  color: var(--text-primary);
  background: var(--background);
  transition: all 0.2s ease;
}

.input:focus {
  outline: none;
  border-color: var(--primary);
  box-shadow: 0 0 0 var(--space-unit) var(--primary-alpha);
}

.input:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.input::placeholder {
  font-size: var(--text-sm);
  color: var(--text-muted);
}
</style>
