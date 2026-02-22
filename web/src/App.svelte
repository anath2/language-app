
<script lang="ts">
// TODO
// Clean up and migration styles/components
// fix svelte issues in translation-result
import NavBar from '@/ui/NavBar.svelte';
import Admin from '@/features/admin/components/Admin.svelte';
import Login from '@/features/auth/components/Login.svelte';
import ProjectsIndex from '@/features/projects/Index.svelte';
import { auth } from '@/features/auth/stores/authStore.svelte';
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
<main class="page-container">
  <Login returnUrl={window.location.pathname + window.location.search} />
</main>
{:else if auth.isAuthenticated || router.route.page === "login"}
  <NavBar />
  <main class="page-container">
    {#if currentPage === "login"}
      {#if router.route.page === "login"}
        <Login returnUrl={router.route.returnUrl} />
      {/if}
    {:else}
      {#if currentPage === "home"}
        <TranslateTextIndex />
      {:else if currentPage === "translation"}
        <TranslationResultIndex translationId={translationId} onBack={backToList} />
      {:else if currentPage === "vocab"}
        <ProjectsIndex />
      {:else if currentPage === "admin"}
        <div class="page-narrow">
          <Admin />
        </div>
      {/if}
    {/if}
  </main>

{/if}
<style>
  .page-container {
    max-width: 1200px;
    margin: 0 auto;
    padding: 1.5rem;
  }

  .page-narrow {
    max-width: 56rem;
    margin: 0 auto;
  }

  @media (max-width: 640px) {
    .page-container {
      padding: 1rem;
    }
  }
</style>
