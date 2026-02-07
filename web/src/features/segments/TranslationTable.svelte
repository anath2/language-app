<script lang="ts">
  import type { SegmentResult } from "./types";

  let { results }: {
    results: SegmentResult[];
  } = $props();

  let showDetails = $state(false);
</script>

{#if results.length > 0}
  <div class="p-4 mt-4 rounded-xl" style="background: var(--surface); box-shadow: 0 1px 3px var(--shadow); border: 1px solid var(--border);">
    <button class="flex items-center justify-between w-full text-left" onclick={() => (showDetails = !showDetails)}>
      <h3 class="font-semibold" style="color: var(--text-primary); font-size: var(--text-base);">Translation Details</h3>
      <span style="color: var(--text-muted); font-size: var(--text-lg);">{showDetails ? "\u2212" : "+"}</span>
    </button>
    {#if showDetails}
      <div class="mt-3 overflow-x-auto">
        <table class="w-full text-left">
          <thead>
            <tr style="border-bottom: 1px solid var(--border);">
              <th class="py-1.5 px-2 font-semibold uppercase tracking-wider" style="color: var(--text-muted); font-size: var(--text-xs);">Chinese</th>
              <th class="py-1.5 px-2 font-semibold uppercase tracking-wider" style="color: var(--text-muted); font-size: var(--text-xs);">Pinyin</th>
              <th class="py-1.5 px-2 font-semibold uppercase tracking-wider" style="color: var(--text-muted); font-size: var(--text-xs);">English</th>
            </tr>
          </thead>
          <tbody>
            {#each results as item}
              <tr class="cursor-pointer translation-row" style="border-bottom: 1px solid var(--background-alt);">
                <td class="py-2 px-2" style="font-family: var(--font-chinese); font-size: var(--text-chinese); color: var(--text-primary);">{item.segment}</td>
                <td class="py-2 px-2" style="color: var(--text-secondary); font-size: var(--text-sm);">{item.pinyin}</td>
                <td class="py-2 px-2" style="color: var(--secondary-dark); font-size: var(--text-sm);">{item.english}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>
{/if}
