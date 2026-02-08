<script lang="ts">
  import { auth } from '../../lib/auth.svelte';

  interface Props {
    returnUrl?: string;
  }

  let { returnUrl = '/' }: Props = $props();

  let password = $state('');
  let isSubmitting = $state(false);

  async function handleLogin(e: Event) {
    e.preventDefault();
    if (!password || isSubmitting) return;

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

<style>
  .login-container {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    padding: 2rem;
    background: linear-gradient(135deg, var(--background) 0%, #f0f0f0 100%);
  }

  .login-card {
    background: var(--surface);
    border-radius: 16px;
    padding: 2.5rem;
    width: 100%;
    max-width: 400px;
    box-shadow: 0 10px 30px var(--shadow);
    border: 1px solid var(--border);
  }

  h1 {
    text-align: center;
    margin-bottom: 2rem;
    color: var(--text-primary);
    font-size: 1.75rem;
    font-weight: 600;
  }

  form {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  label {
    font-weight: 500;
    color: var(--text-secondary);
    font-size: 0.875rem;
  }

  input[type="password"] {
    padding: 0.75rem 1rem;
    border: 2px solid var(--border);
    border-radius: 8px;
    font-size: 1rem;
    transition: all 0.2s ease;
    background: var(--background);
  }

  input[type="password"]:focus {
    outline: none;
    border-color: var(--primary);
    box-shadow: 0 0 0 3px var(--primary-alpha);
  }

  input[type="password"]:disabled {
    background: var(--surface-2);
    cursor: not-allowed;
  }

  button[type="submit"] {
    padding: 0.75rem 1.5rem;
    background: var(--primary);
    color: white;
    border: none;
    border-radius: 8px;
    font-size: 1rem;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s ease;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
  }

  button[type="submit"]:hover:not(:disabled) {
    background: var(--primary-hover);
    transform: translateY(-1px);
  }

  button[type="submit"]:active:not(:disabled) {
    transform: translateY(0);
  }

  button[type="submit"]:disabled {
    background: var(--text-disabled);
    cursor: not-allowed;
    opacity: 0.8;
  }

  .error-message {
    background: var(--error-bg);
    color: var(--error);
    padding: 0.75rem 1rem;
    border-radius: 8px;
    font-size: 0.875rem;
    border: 1px solid var(--error-border);
  }

  .btn-loading {
    display: none;
  }

  form.loading .btn-text {
    display: none;
  }

  form.loading .btn-loading {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
  }

  .spinner {
    width: 16px;
    height: 16px;
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-top-color: white;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* Responsive */
  @media (max-width: 480px) {
    .login-container {
      padding: 1rem;
    }

    .login-card {
      padding: 2rem;
    }

    h1 {
      font-size: 1.5rem;
    }
  }
</style>