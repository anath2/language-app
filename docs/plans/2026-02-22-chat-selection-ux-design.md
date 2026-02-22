# Chat Panel Layout Push — Design

**Date:** 2026-02-22
**Branch:** worktree-text-selection
**Issue:** Chat overlay blocks translation content, forcing users to select text before opening chat.

## Problem

The chat pane opens as a full-screen overlay with a darkened backdrop. Clicking anywhere on the backdrop closes the pane. This means:
- Translation content is inaccessible while chat is open
- Users must select text before opening chat — they cannot read or select from the translation while chatting

## Solution

Replace the full-screen overlay with a layout push: when chat opens, the page transitions to a two-column grid with the translation on the left and the chat panel on the right. Both areas are fully visible and interactive simultaneously. Text selection (from PR #30) continues to work naturally — `mouseup` in the translation area populates the selection chip in the chat input.

## Layout Breakpoints

| Screen width | Chat closed | Chat open |
|---|---|---|
| < 900px | translation single-column | chat full width (100%) |
| 900px–1400px (laptop) | translation `1fr 1fr` | outer `2fr 1fr`, inner translation collapses to single-column |
| ≥ 1400px (desktop) | translation `1fr 1fr` | outer `2fr 1fr`, inner translation stays `1fr 1fr` |

Breakpoints are intentional starting points — expected to be tuned after manual inspection on real hardware.

## Component Changes

### `Sidepane.svelte`
- Remove the backdrop `<div>` and `handleBackdropClick` handler
- The panel becomes a plain fixed-right column element (no overlay mechanics)
- Escape key close and explicit close button remain

### `Index.svelte`
- Outer container switches from single-column to `2fr 1fr` grid when `chatPaneOpen` is true
- Transition: `grid-template-columns 0.25s ease` (matches existing chat slide-in duration)
- Below 900px: chat takes `100%` width, translation is not visible while chat is open
- Pass `chatPaneOpen` as a prop into the translation layout component

### Translation layout container
- Accepts `chatOpen` prop
- When `chatOpen && screenWidth < 1400px`: inner grid collapses to single-column
- When `chatOpen && screenWidth >= 1400px`: inner grid stays `1fr 1fr`
- Original text card sits at top of single-column layout and scrolls away naturally (no sticky header)

### `TranslationChat.svelte`
- Chat message bubbles: bump to `text-sm` (14px)
- Textarea input: bump to `text-sm`

## What Is Not Changing

- `mouseup` / `selected_text` capture logic (PR #30) — untouched
- Backend, API, chat message logic — untouched
- Escape key close behavior — retained
- Explicit close button in chat panel header — retained
