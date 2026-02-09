<script lang="ts">
import { auth } from '@/features/auth/stores/authStore.svelte';

interface Props {
  returnUrl?: string;
}

const { returnUrl = '/' }: Props = $props();

let password = $state('');
let isSubmitting = $state(false);

async function handleLogin(e: Event) {
  e.preventDefault();
  if (!password.trim() || isSubmitting) return;

  isSubmitting = true;
  const success = await auth.login(password);
  isSubmitting = false;

  if (success) {
    // Redirect to return URL or home
    window.location.href = returnUrl.startsWith('/') ? `#${returnUrl}` : `/${returnUrl}`;
  }
}

// Focus password input on mount
$effect(() => {
  const input = document.querySelector('input[type="password"]') as HTMLInputElement;
  input?.focus();
});
</script>

<div class="login-container">
  <div class="login-card">
    <h1>登录 / Login</h1>
    <form onsubmit={handleLogin} class:loading={isSubmitting}>
      <div class="form-group">
        <label for="password">密码 / Password</label>
        <input
          id="password"
          type="password"
          bind:value={password}
          placeholder="Enter password"
          disabled={isSubmitting}
          autocomplete="current-password"
        />
      </div>

      {#if auth.error}
        <div class="error-message">{auth.error}</div>
      {/if}

      <button type="submit" disabled={isSubmitting}>
        <span class="btn-text">登录 / Login</span>
        <span class="btn-loading">
          <span class="spinner"></span>
          Logging in...
        </span>
      </button>
    </form>
  </div>
</div>