# Route Contract Freeze

This document freezes the Python server contract from `server_old/` so Go migration can preserve path and behavior parity.

## Global Auth Behavior

- Session cookie name: `session`
- Excluded paths from auth: `GET/POST /login`, `GET /health`, any `/css/*`
- Unauthenticated request behavior:
  - HTMX (`HX-Request: true`) -> `401` with header `HX-Redirect: /login`
  - HTML request (`Accept` contains `text/html`) -> `303` redirect to `/login`
  - API/other -> `401` JSON `{"detail":"Not authenticated"}`

## Static + SPA

- `GET /css/*` static assets from `web/public/css`
- `GET /assets/*` static assets from `web/dist/assets`
- SPA fallback for non-API/static paths (`/{path:path}`), except `/api/*`, `/css/*`, `/assets/*`

## Route Inventory

| Method | Path | Domain | Typical Response | Auth Required |
| --- | --- | --- | --- | --- |
| GET | `/health` | health | JSON `{"status":"ok"}` | no |
| GET | `/login` | auth | HTML (SPA) or redirect | no |
| POST | `/login` | auth | `303` redirect or `401` | no |
| POST | `/logout` | auth | `303` redirect | yes |
| GET | `/` | auth/ui | HTML (SPA) | yes |
| GET | `/translations` | auth/ui | HTML (SPA) | yes |
| POST | `/api/translations` | translations | JSON | yes |
| GET | `/api/translations` | translations | JSON | yes |
| GET | `/api/translations/{translation_id}` | translations | JSON | yes |
| GET | `/api/translations/{translation_id}/status` | translations | JSON | yes |
| DELETE | `/api/translations/{translation_id}` | translations | JSON | yes |
| GET | `/api/translations/{translation_id}/stream` | translations | SSE | yes |
| POST | `/api/texts` | api | JSON | yes |
| GET | `/api/texts/{text_id}` | api | JSON | yes |
| POST | `/api/events` | api | JSON | yes |
| POST | `/api/vocab/save` | api | JSON | yes |
| POST | `/api/vocab/status` | api | JSON | yes |
| POST | `/api/vocab/lookup` | api | JSON | yes |
| GET | `/api/vocab/srs-info` | api | JSON | yes |
| GET | `/api/review/queue` | api | JSON | yes |
| POST | `/api/review/answer` | api | JSON | yes |
| GET | `/api/review/count` | api | JSON | yes |
| POST | `/api/segments/translate-batch` | api | JSON | yes |
| GET | `/admin` | admin | HTML (SPA) | yes |
| GET | `/admin/progress/export` | admin | JSON file download | yes |
| POST | `/admin/progress/import` | admin | JSON | yes |
| GET | `/admin/api/profile` | admin | JSON | yes |
| POST | `/admin/api/profile` | admin | JSON | yes |
