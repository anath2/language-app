<script lang="ts">
import { auth } from '@/features/auth/stores/authStore.svelte';
import Input from '@/ui/Input.svelte';

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
        <Input
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

<style>
  .login-container {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, var(--background) 0%, var(--background-alt) 100%);
    padding: var(--space-4);
  }

  .login-card {
    background: var(--surface);
    border-radius: var(--radius-2xl);
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
    padding: var(--space-10);
    width: 100%;
    max-width: 400px;
    border: 1px solid var(--border);
  }

  .login-card h1 {
    text-align: center;
    margin-bottom: var(--space-6);
    color: var(--text-primary);
    font-size: var(--text-xl);
  }

  .login-card .form-group {
    margin-bottom: var(--space-5);
  }

  .login-card label {
    display: block;
    margin-bottom: var(--space-2);
    color: var(--text-secondary);
    font-size: var(--text-sm);
  }

  .login-card button[type="submit"] {
    width: 100%;
    padding: calc(var(--space-unit) * 3.5);
    background: var(--primary);
    color: white;
    border: none;
    border-radius: var(--radius-lg);
    font-size: var(--text-base);
    font-weight: 500;
    cursor: pointer;
    transition: all 0.15s ease;
    position: relative;
  }

  .login-card button[type="submit"]:hover:not(:disabled) {
    background: var(--primary-dark);
  }

  .login-card button[type="submit"]:disabled {
    opacity: 0.7;
    cursor: not-allowed;
  }

  .login-card .btn-loading {
    display: none;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
  }

  .login-card form.loading .btn-text {
    display: none;
  }

  .login-card form.loading .btn-loading {
    display: inline-flex;
  }

  .login-card .error-message {
    background: var(--error-bg);
    color: var(--error);
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-lg);
    font-size: var(--text-sm);
    margin-bottom: var(--space-4);
    border: 1px solid var(--error-border);
  }

  .login-card .btn-text {
    display: inline;
  }

  .login-card .spinner {
    width: var(--space-4);
    height: var(--space-4);
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-top-color: white;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>