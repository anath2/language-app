<script lang="ts">
import TranslateForm from '@/features/translation/translation-home/components/TranslateForm.svelte';
import TranslationList from '@/features/translation/translation-home/components/TranslationList.svelte';
import VocabReviewCard from '@/features/translation/translation-home/components/VocabReviewCard.svelte';
import { translationStore } from '@/features/translation/stores/translationStore.svelte';
import { router } from '@/lib/router.svelte';

$effect(() => {
  void translationStore.loadTranslations();
});

async function handleSubmit(text: string) {
  const id = await translationStore.submitTranslation(text);
  if (id) {
    router.navigateTo(id);
  }
}

async function handleDelete(id: string) {
  if (!confirm('Delete this translation?')) return;
  await translationStore.deleteTranslation(id);
}

function handleSelect(id: string) {
  router.navigateTo(id);
}
</script>

<div class="translate-text-layout">
  <section class="translate-text-left">
    <TranslateForm
      onSubmit={handleSubmit}
      loading={translationStore.loadingState === 'loading'}
    />

    <VocabReviewCard />
  </section>


  <section class="translate-text-right">
      <TranslationList
        translations={translationStore.translations}
        onSelect={handleSelect}
        onDelete={handleDelete}
      />
  </section>
</div>


<style>
  .translate-text-layout {
    display: grid;
    grid-template-columns: 1fr 380px;
    gap: 1.5rem;
    align-items: start;
  }

  .translate-text-left,
  .translate-text-right {
    min-width: 0;
  }

  @media (max-width: 960px) {
    .translate-text-layout {
      grid-template-columns: 1fr;
    }
  }
</style>
