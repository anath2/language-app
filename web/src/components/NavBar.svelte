<script lang="ts">
import { auth } from '@/features/auth/stores/authStore.svelte';
import { vocabStore } from '@/features/vocab/stores/vocabStore.svelte';
import { router } from '@/lib/router.svelte';

let accountDropdownOpen = $state(false);

// Load due count on mount
$effect(() => {
  if (auth.isAuthenticated) {
    vocabStore.loadDueCount();
    // Refresh every minute
    const interval = setInterval(() => vocabStore.loadDueCount(), 60000);
    return () => clearInterval(interval);
  }
});

function toggleAccountDropdown(event: Event) {
  event.stopPropagation();
  accountDropdownOpen = !accountDropdownOpen;
}

function closeDropdown() {
  accountDropdownOpen = false;
}

function handleAdminClick() {
  accountDropdownOpen = false;
  router.navigateToAdmin();
}

function handleLogoutClick() {
  accountDropdownOpen = false;
  auth.logout();
}
</script>

<svelte:window onclick={closeDropdown} />

<nav class="navbar">
  {#if auth.isAuthenticated}
    <div class="nav-main">
      <div class="nav-brand">
        <span class="brand-text">Language App</span>
      </div>
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
    </div>

    <div class="nav-right">
      <div class="dropdown-container">
        <button
          class="nav-item account-btn"
          onclick={toggleAccountDropdown}
          aria-haspopup="true"
          aria-expanded={accountDropdownOpen}
        >
          <span class="nav-label">Account</span>
          <span class="dropdown-arrow" class:open={accountDropdownOpen}>▼</span>
        </button>

        {#if accountDropdownOpen}
          <div class="dropdown-menu">
            <button class="dropdown-item" onclick={handleAdminClick}>
              <span>Admin</span>
            </button>
            <div class="dropdown-divider"></div>
            <button class="dropdown-item logout-item" onclick={handleLogoutClick}>
              <span>Logout</span>
            </button>
          </div>
        {/if}
      </div>
    </div>
  {:else}
    <div class="nav-main">
      <div class="nav-brand">
        <span class="brand-text">Language App</span>
      </div>
    </div>
    <div class="nav-right">
      <button class="nav-item login-btn" onclick={() => router.navigateToLogin(window.location.pathname)} title="Login">
        <span class="nav-label">Login</span>
      </button>
    </div>
  {/if}
</nav>
