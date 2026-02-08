<script lang="ts">
import { auth } from '@/features/auth/stores/authStore.svelte';
import { vocabStore } from '@/features/vocab/stores/vocabStore.svelte';
import { router } from '@/lib/router.svelte';

// Load due count on mount
$effect(() => {
  if (auth.isAuthenticated) {
    vocabStore.loadDueCount();
    // Refresh every minute
    const interval = setInterval(() => vocabStore.loadDueCount(), 60000);
    return () => clearInterval(interval);
  }
});
</script>

<nav class="navbar">
  <div class="nav-brand">
    <span class="brand-text">Language App</span>
  </div>

  {#if auth.isAuthenticated}
    <div class="nav-items">
      <button
        class="nav-item"
onclick={() => router.navigateHome()}
      >
        <span class="nav-label">Translate</span>
      </button>

      <button
        class="nav-item"
        onclick={() => router.navigateToVocab()}
      >
        <span class="nav-label">Vocab</span>
        {#if vocabStore.dueCount > 0}
          <span class="badge">{vocabStore.dueCount}</span>
        {/if}
      </button>

      <button
        class="nav-item"
        onclick={() => router.navigateToAdmin()}
      >
        <span class="nav-label">Admin</span>
      </button>

      <button class="nav-item logout-btn" onclick={auth.logout} title="Logout">
        <span class="nav-label">Logout</span>
      </button>
    </div>
  {:else}
    <div class="nav-items">
      <button class="nav-item login-btn" onclick={() => router.navigateToLogin(window.location.pathname)} title="Login">
        <span class="nav-label">Login</span>
      </button>
    </div>
  {/if}
</nav>

<style>
  .navbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.75rem 1.5rem;
    background: var(--surface);
    border-bottom: 1px solid var(--border);
    box-shadow: 0 1px 3px var(--shadow);
    position: sticky;
    top: 0;
    z-index: 100;
  }

  .nav-brand {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-weight: 600;
    font-size: 1.1rem;
    color: var(--text-primary);
  }

  .brand-icon {
    font-size: 1.25rem;
  }

  .nav-items {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .nav-item {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 1rem;
    border: none;
    background: transparent;
    color: var(--text-secondary);
    font-size: 0.95rem;
    cursor: pointer;
    border-radius: 6px;
    transition: all 0.2s ease;
    position: relative;
  }

  .nav-item:hover {
    background: rgba(124, 158, 178, 0.08);
    color: var(--text-primary);
  }

  .badge {
    position: absolute;
    top: 2px;
    right: 2px;
    min-width: 18px;
    height: 18px;
    padding: 0 5px;
    background: var(--secondary);
    color: white;
    font-size: 0.7rem;
    font-weight: 600;
    border-radius: 9px;
    display: flex;
    align-items: center;
    justify-content: center;
  }


  @media (max-width: 480px) {
    .navbar {
      padding: 0.75rem 1rem;
    }

    .brand-text {
      display: none;
    }

    .nav-label {
      display: none;
    }

    .nav-item {
      padding: 0.5rem;
    }
  }

  .login-btn, .logout-btn {
    margin-left: 1rem;
    background: rgba(124, 158, 178, 0.08);
    border: 1px solid var(--border);
  }

  .login-btn:hover, .logout-btn:hover {
    background: rgba(124, 158, 178, 0.12);
  }
</style>
