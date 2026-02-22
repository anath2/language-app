# Chat Panel Layout Push — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the full-screen chat overlay with a side-by-side layout push so the translation content remains readable and selectable while the chat panel is open.

**Architecture:** `Index.svelte` gets a new outer `.page-wrapper` grid (`2fr 1fr` when chat open). `TranslationChat.svelte` drops `Sidepane` and renders as a plain docked panel in the second column. CSS breakpoints control three states: full-width chat on narrow screens, single-column translation on laptop, two-column translation on desktop.

**Tech Stack:** Svelte 5, CSS grid, CSS custom properties, no new dependencies.

---

### Task 1: Restructure `Index.svelte` — outer grid

**Files:**
- Modify: `web/src/features/translation/translation-result/Index.svelte`

No frontend test framework — verify manually in browser.

**Step 1: Wrap page contents in `.page-wrapper`**

In `Index.svelte`, the current root element is `<div class="page-container">`. Replace it with a two-level structure:

```svelte
<div class="page-wrapper" class:chat-open={chatPaneOpen}>
  <div class="page-content">
    <div class="page-header">
      ...existing header...
    </div>

    <div class="translation-layout" class:chat-open={chatPaneOpen}>
      <div class="translation-left">
        ...existing left column...
      </div>
      <div class="translation-right">
        ...existing right column...
      </div>
    </div>
  </div>

  <TranslationChat
    translationId={translationId}
    open={chatPaneOpen}
    onClose={() => (chatPaneOpen = false)}
  />
</div>
```

Key changes from current structure:
- `page-container` → `page-wrapper` (outer grid) + `page-content` (left column, padded)
- `<TranslationChat>` moves inside `.page-wrapper` as the second grid column (was after the container)
- `class:chat-open={chatPaneOpen}` added to both `.page-wrapper` and `.translation-layout`

**Step 2: Replace the CSS for `page-container`/`page-wrapper`**

Remove the `.page-container` rule. Add:

```css
.page-wrapper {
  display: grid;
  grid-template-columns: 1fr;
  transition: grid-template-columns 0.25s ease;
  min-height: 100vh;
  align-items: start;
}

.page-wrapper.chat-open {
  grid-template-columns: 2fr 1fr;
}

.page-content {
  max-width: 1200px;
  margin: 0 auto;
  padding: 1.5rem;
  min-width: 0; /* prevent grid blowout */
  width: 100%;
  box-sizing: border-box;
}

/* On narrow screens: chat takes full width, translation is behind it */
@media (max-width: 900px) {
  .page-wrapper.chat-open {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .page-content {
    padding: 1rem;
  }
}
```

**Step 3: Verify layout in browser**

Open any translation detail page. Confirm:
- Without chat: page looks identical to before (single column, centered, max-width 1200px)
- Chat button opens chat: page shifts to 2fr/1fr grid, chat panel appears on right
- Chat close button returns to single-column layout with smooth animation
- Translation content remains visible and scrollable while chat is open

**Step 4: Commit**

```bash
git add web/src/features/translation/translation-result/Index.svelte
git commit -m "feat: add page-wrapper grid for chat layout push"
```

---

### Task 2: Collapse inner translation grid on laptop

**Files:**
- Modify: `web/src/features/translation/translation-result/Index.svelte` (CSS only)

**Step 1: Add CSS for inner grid collapse**

In the `<style>` block of `Index.svelte`, add to the `.translation-layout` rules:

```css
/* Laptop: collapse inner 1fr 1fr to single column when chat is open */
@media (max-width: 1400px) {
  .translation-layout.chat-open {
    grid-template-columns: 1fr;
  }

  .translation-layout.chat-open :global(.sticky-top) {
    position: static;
  }
}
```

This means:
- **< 900px, chat open:** outer grid is 1 col (chat full width), inner grid irrelevant
- **900px–1400px, chat open:** outer grid 2fr/1fr, inner translation grid collapses to 1 col
- **≥ 1400px, chat open:** outer grid 2fr/1fr, inner translation grid stays 1fr/1fr

**Step 2: Verify in browser at multiple widths**

Resize the browser window and confirm:
- At 1200px wide with chat open: translation shows as single column (original text on top, segments below)
- At 1500px wide with chat open: translation shows as two columns (original left, segments right)
- Sticky original text card is un-stickied at 1200px when chat open (no overlap issues)

**Step 3: Commit**

```bash
git add web/src/features/translation/translation-result/Index.svelte
git commit -m "feat: collapse inner translation grid on laptop when chat open"
```

---

### Task 3: Convert `TranslationChat.svelte` to docked panel

**Files:**
- Modify: `web/src/features/translation/translation-result/components/TranslationChat.svelte`

This is the largest change. The `<Sidepane>` wrapper (which provides the backdrop + overlay) is replaced by a plain docked panel div that fills its grid column.

**Step 1: Add the X icon import**

At the top of the `<script>` block, add:

```ts
import { X } from '@lucide/svelte';
```

(Sidepane imported it; we now need it directly.)

**Step 2: Add `svelte:window` Escape handler**

After the closing `</script>` tag, before the panel markup, add:

```svelte
<svelte:window onkeydown={(e) => { if (e.key === 'Escape' && open) onClose(); }} />
```

**Step 3: Replace `<Sidepane>` wrapper with `.chat-panel`**

Current template (line 167):
```svelte
<Sidepane title="Chat" {open} onClose={onClose} width="400px">
  <div class="chat-layout">
    ...
  </div>
</Sidepane>
```

Replace with:
```svelte
{#if open}
  <div class="chat-panel">
    <header class="chat-panel-header">
      <h2 class="chat-panel-title">Chat</h2>
      <Button variant="ghost" size="xs" iconOnly ariaLabel="Close panel" onclick={onClose}>
        <X size={18} />
      </Button>
    </header>
    <div class="chat-panel-content">
      <div class="chat-layout">
        ...existing chat-layout contents unchanged...
      </div>
    </div>
  </div>
{/if}
```

**Step 4: Remove the `Sidepane` import**

Remove from the import at the top of `<script>`:
```ts
import Sidepane from '@/ui/Sidepane.svelte';
```

**Step 5: Replace the `<style>` block — add panel styles, bump text size**

Add these rules to the `<style>` block (keep all existing rules, add new ones):

```css
.chat-panel {
  display: flex;
  flex-direction: column;
  height: 100vh;
  position: sticky;
  top: 0;
  border-left: 1px solid var(--border);
  background: var(--surface);
  box-shadow: -4px 0 12px var(--shadow);
  animation: chat-slide-in 0.25s ease;
  overflow: hidden;
}

.chat-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  padding: var(--space-4) var(--space-5);
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.chat-panel-title {
  margin: 0;
  font-size: var(--text-lg);
  font-weight: 600;
  color: var(--text-primary);
}

.chat-panel-content {
  flex: 1;
  overflow: auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

@keyframes chat-slide-in {
  from { transform: translateX(100%); }
  to { transform: translateX(0); }
}

/* Full-width on narrow screens */
@media (max-width: 900px) {
  .chat-panel {
    position: fixed;
    inset: 0;
    width: 100%;
    height: 100%;
  }
}
```

Update `.chat-bubble-content` font size (bump from `var(--text-sm)` if it's currently smaller, or confirm it stays `var(--text-sm)`):

```css
.chat-bubble-content {
  font-size: var(--text-sm); /* 14px — readable in narrow panel */
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}
```

**Step 6: Verify in browser**

- Chat panel opens as a docked right column, not an overlay
- Clicking the translation content does NOT close the chat
- Close button (X) closes the chat
- Escape key closes the chat
- On screens < 900px: chat takes full width (fixed position)
- Chat messages are readable at text-sm size

**Step 7: Commit**

```bash
git add web/src/features/translation/translation-result/components/TranslationChat.svelte
git commit -m "feat: replace Sidepane overlay with docked chat panel"
```

---

### Task 4: Clean up `Sidepane.svelte` (optional, if no other usages)

**Files:**
- Check: `web/src/` (grep for Sidepane usage)

**Step 1: Check if Sidepane is used anywhere else**

```bash
grep -r "Sidepane" web/src --include="*.svelte" -l
```

If the only result is `Sidepane.svelte` itself (no other consumers), the component can be left as-is for future use. No action needed.

If there are other consumers, confirm they still work correctly — the component is unchanged.

**Step 2: Commit if any cleanup was done**

```bash
git add web/src/ui/Sidepane.svelte
git commit -m "chore: clean up Sidepane if unused"
```

---

### Task 5: Manual cross-breakpoint verification

No automated frontend tests. Verify at each breakpoint:

| Width | Chat closed | Chat open |
|---|---|---|
| 1440px+ | ✓ 2-col translation | ✓ 2fr/1fr outer, 2-col inner translation |
| 1100px | ✓ 2-col translation | ✓ 2fr/1fr outer, 1-col inner translation |
| 700px | ✓ 1-col translation | ✓ chat full width (fixed) |

Also verify:
- [ ] Text selection in translation area works while chat is open (select text → chip appears in chat input)
- [ ] Sending a message with selected text works end-to-end
- [ ] Chat messages scroll correctly in the docked panel
- [ ] Review cards display correctly in the docked panel
- [ ] No layout blowout at any breakpoint (min-width: 0 on grid children)

**Step 3: Final commit**

```bash
git add -p  # stage any remaining tweaks
git commit -m "fix: layout polish after chat panel cross-breakpoint verification"
```
