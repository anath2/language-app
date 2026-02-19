
<script lang="ts">
// TODO
// Clean up and migration styles/components
// fix svelte issues in translation-result
import NavBar from '@/ui/NavBar.svelte';
import Admin from '@/features/admin/components/Admin.svelte';
import Login from '@/features/auth/components/Login.svelte';
import { auth } from '@/features/auth/stores/authStore.svelte';
import DiscoverIndex from '@/features/discovery/Index.svelte';
import TranslateTextIndex from '@/features/translation/translation-home/Index.svelte';
import TranslationResultIndex from '@/features/translation/translation-result/Index.svelte';
import { router } from '@/lib/router.svelte';

const currentPage = $derived(router.route.page);
const translationId = $derived(router.route.page === 'translation' ? router.route.id : null);

// Check authentication on mount
$effect(() => {
  void auth.checkAuthStatus();
});

function backToList() {
  router.navigateHome();
}
</script>


{#if !auth.isAuthenticated && !auth.isLoading && router.route.page !== "login"}
  <Login returnUrl={window.location.pathname + window.location.search} />
{:else if auth.isAuthenticated || router.route.page === "login"}
  <NavBar />

  {#if currentPage === "login"}
    {#if router.route.page === "login"}
      <Login returnUrl={router.route.returnUrl} />
    {/if}
  {:else}

    {#if currentPage === "home"}
  <div class="page-container">
    <TranslateTextIndex />
  </div>

{:else if currentPage === "translation"}
  <div class="page-container">
    <TranslationResultIndex translationId={translationId} onBack={backToList} />
  </div>

{:else if currentPage === "admin"}
    <!-- Admin Page -->
    <div class="page-container max-w-4xl">
      <Admin />
    </div>

{:else if currentPage === "discover"}
    <!-- Discover/Explore Page -->
    <div class="page-container">
      <DiscoverIndex />
    </div>
  {/if}

  {/if}

{/if}

<style>
  .page-container {
    max-width: 1200px;
    margin: 0 auto;
    padding: 1.5rem;
  }

  .page-container.max-w-4xl {
    max-width: 56rem;
  }

  @media (max-width: 640px) {
    .page-container {
      padding: 1rem;
    }
  }
</style>
