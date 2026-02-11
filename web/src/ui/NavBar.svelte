<script lang="ts">
import { UserRound, LogOut } from '@lucide/svelte';
import { auth } from '@/features/auth/stores/authStore.svelte';
import { vocabStore } from '@/features/translation/stores/vocabStore.svelte';
import { router } from '@/lib/router.svelte';
import Button from '@/ui/Button.svelte';

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
        <img src="/logo.svg" alt="" class="logo-img" />
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
        <span class="nav-label">Projects</span>
      </button>
      <button
        class="nav-item"
        onclick={() => router.navigateToVocab()}
      >
        <span class="nav-label">Explore</span>
      </button>

    </div>

    <div class="nav-right">
      <div class="dropdown-container">
        <button
          class="btn btn-ghost nav-item account-btn"
          onclick={toggleAccountDropdown}
          aria-haspopup="true"
          aria-expanded={accountDropdownOpen}
        >
          <UserRound size={24} class="account-btn-icon" />
        </button>

        {#if accountDropdownOpen}
          <div class="dropdown-menu">
            <button class="dropdown-item" onclick={handleAdminClick}>
              <span class="nav-label">Admin</span>
            </button>
            <button class="dropdown-item" onclick={handleLogoutClick}>
              <span class="nav-label">Logout</span>
            </button>
          </div>
        {/if}
      </div>
    </div>
  {:else}
    <div class="nav-main">
      <div class="nav-brand">
        <img src="/logo.svg" alt="Logo" class="logo-img" />
      </div>
    </div>
    <div class="nav-right">
      <Button variant="primary" size="md" class="nav-item login-btn" onclick={() => router.navigateToLogin(window.location.pathname)} title="Login">
        <UserRound size={16} />
      </Button>
    </div>
  {/if}
</nav>

<style>
.navbar {
  top: 0;
  display: flex;
  position: sticky;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-6) var(--space-12);
  background: var(--surface);
  border-bottom: 1px solid var(--border);
  z-index: var(--z-sticky);
}

.nav-brand img {
  height: 32px;
  width: auto;
  display: block;
}

.nav-main {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  flex: 1;
}

.nav-right {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.nav-item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4);
    border: none;
    background: transparent;
    color: var(--text-primary);
    font-size: calc(var(--text-unit) * 1.086);
    cursor: pointer;
    border-radius: var(--radius-md);
    transition: all 0.2s ease;
    position: relative;
  }

.nav-main .nav-item::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: var(--space-4);
  right: var(--space-4);
  height: 3px;
  background: var(--primary);
  border-radius: 2px;
  transform: scaleX(0);
  transition: transform 0.2s ease;
}

.nav-main .nav-item:hover::after {
  transform: scaleX(1);
}

/* dropdown/account styles are local to avoid global navbar.css coupling */
.dropdown-container {
  position: relative;
  display: inline-flex;
  align-items: center;
}

.dropdown-menu {
  position: absolute;
  top: calc(100% + var(--space-2));
  right: 0;
  left: auto;
  min-width: 160px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  box-shadow: 0 4px 12px var(--shadow);
  z-index: calc(var(--z-sticky) + 1);
  overflow: hidden;
}
 
.dropdown-item {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  width: 100%;
  text-align: left;
  background: transparent;
  cursor: pointer;
  border: none;
  padding: var(--space-2) var(--space-4);
  font-size: calc(var(--text-unit) * 1.086);
  color: var(--text-secondary);
  transition: all 0.2s ease;
}

.dropdown-item:hover {
  background: var(--primary-alpha);
  color: var(--text-primary);
}

@media (max-width: 480px) {
  .navbar {
    padding: var(--space-3) var(--space-4);
  }

  .nav-label {
    display: none;
  }

  .dropdown-item .nav-label {
    display: inline;
  }

  .nav-item {
    padding: var(--space-2);
  }

  .dropdown-menu {
    right: 0;
    min-width: 120px;
  }
}

</style>
