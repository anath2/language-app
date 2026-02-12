# Route Contract

This document defines the REST API route contract for the Go backend.

## Global Auth Behavior

- Session cookie name: `session`
- Excluded paths from auth: `POST /api/auth/login`, `GET /health`
- Unauthenticated request response: `401` JSON `{"detail":"Not authenticated"}`

## Route Inventory

| Method | Path | Tag | Typical Response | Auth Required |
| --- | --- | --- | --- | --- |
| GET | `/health` | health | JSON `{"status":"ok"}` | no |
| POST | `/api/auth/login` | auth | JSON `{"ok":true}` + Set-Cookie | no |
| POST | `/api/auth/logout` | auth | JSON `{"ok":true}` | yes |
| POST | `/api/translations` | translations | JSON | yes |
| GET | `/api/translations` | translations | JSON | yes |
| GET | `/api/translations/{translation_id}` | translations | JSON | yes |
| GET | `/api/translations/{translation_id}/status` | translations | JSON | yes |
| DELETE | `/api/translations/{translation_id}` | translations | JSON | yes |
| GET | `/api/translations/{translation_id}/stream` | translations | SSE (`text/event-stream`) | yes |
| POST | `/api/texts` | texts | JSON | yes |
| GET | `/api/texts/{text_id}` | texts | JSON | yes |
| POST | `/api/events` | events | JSON | yes |
| POST | `/api/vocab/save` | vocab | JSON | yes |
| POST | `/api/vocab/status` | vocab | JSON | yes |
| POST | `/api/vocab/lookup` | vocab | JSON | yes |
| GET | `/api/vocab/srs-info` | vocab | JSON | yes |
| GET | `/api/review/queue` | review | JSON | yes |
| POST | `/api/review/answer` | review | JSON | yes |
| GET | `/api/review/count` | review | JSON | yes |
| POST | `/api/segments/translate-batch` | segments | JSON | yes |
| GET | `/api/admin/progress/export` | admin | JSON file download | yes |
| POST | `/api/admin/progress/import` | admin | JSON | yes |
| GET | `/api/admin/profile` | admin | JSON | yes |
| POST | `/api/admin/profile` | admin | JSON | yes |
| POST | `/api/extract-text` | ocr | JSON | yes |
