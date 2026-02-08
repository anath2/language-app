# Svelte Migration Plan

## Goals
- Improve maintainability and readability by moving UI logic into Svelte components.
- Preserve current functionality (job queue, translation streaming, segment editing, review panel, SRS updates).
- Migrate incrementally to avoid large rewrites and regressions.

## Guiding Principles
- Keep server-rendered templates until Svelte is fully responsible for the UI.
- Migrate one UI surface at a time, behind feature flags if needed.
- Preserve existing API contracts and SSE streaming behavior.
- Prefer small, focused components with clear props and events.

## Phase 0: Repo Split + Foundations
1. Split the repo into separate workspaces:
   - `server/` (FastAPI app, Python dependencies, tests).
   - `web/` (Svelte app, Node tooling, build outputs).
2. Move current backend sources into `server/` (including `app/`, `tests/`, and `pyproject.toml`).
3. Create `web/` with Vite + Svelte tooling and its own `package.json`.
4. Decide how to serve frontend assets:
   - Development: run Vite dev server and point FastAPI templates to it (or use proxy).
   - Production: build to `web/dist` and have FastAPI serve static assets from there.
5. Update template bootstrapping:
   - Keep `server/app/templates/index.html` for initial mount.
   - Replace script tags with the Svelte bundle entry once ready.

## Phase 1: Shared State and Services
1. Port `State` to a Svelte store module (e.g., `web/src/stores/appState.ts`).
2. Move API helpers from `server/app/static/js/modules/api.js` into `web/src/lib/api.ts`.
3. Port utilities from `server/app/static/js/modules/utils.js` into `web/src/lib/utils.ts`.

## Phase 2: Migrate Non-Streaming UI Panels
1. Review panel:
   - Componentize (`web/src/components/ReviewPanel.svelte`) with internal state and events.
   - Implement the current flow (load queue, reveal answer, grade).
2. Job queue list:
   - Componentize (`web/src/components/JobQueue.svelte`, `web/src/components/JobCard.svelte`).
   - Keep data fetching and rendering parity with the existing UI.

## Phase 3: Translation Results + Streaming
1. Translation results container (`web/src/components/TranslationResults.svelte`).
2. Port progress UI and stream handling (SSE) into a Svelte store + actions.
3. Preserve current segmentation layout and styling.

## Phase 4: Segment Interactions + Editor
1. Tooltip + segment interaction logic:
   - Create `web/src/components/Segment.svelte` and `web/src/components/SegmentTooltip.svelte`.
   - Replace direct DOM listeners with component events and store updates.
2. Segment editor:
   - Extract split/join logic into a Svelte store or helper module.
   - Provide edit mode UI via `web/src/components/SegmentEditor.svelte`.

## Phase 5: Cleanup and Removal
1. Remove old JS modules in `server/app/static/js` once all UI is ported.
2. Remove `window.App` globals and inline `onclick` handlers from `server/app/templates`.
3. Delete unused CSS selectors and consolidate styles as needed.

## Validation Checklist
- Job queue loads, expands, streams, and updates counts correctly.
- Translation stream shows progress and final results.
- Segment edit mode supports split/join, cancel, save.
- Tooltip and SRS interactions behave identically to current app.
- Review panel flow works end-to-end.

## Risks / Mitigations
- **Risk:** Regression in SSE streaming UI.
  - **Mitigation:** Add integration tests or manual test checklist per phase.
- **Risk:** CSS drift when componentizing.
  - **Mitigation:** Keep current CSS and re-use existing class names.
- **Risk:** Partial migration causes double-binding.
  - **Mitigation:** Use feature flags to disable old JS when Svelte equivalent is live.
