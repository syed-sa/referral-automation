# API contracts

Base path: `/api/v1`. All routes require `Authorization: Bearer <Supabase access token>` except the health endpoints. The owner is always the token's `sub`; request bodies containing `user_id` (or any unknown field) are rejected with 400.

## Conventions

- Success: `{"data": <object|array>}`. Lists: `{"data": [...], "pagination": {"page","page_size","total","total_pages"}}`.
- Errors: `{"error": {"code","message","details?","request_id"}}`.
- Pagination: `?page=1&page_size=20` (page_size max 100).
- Resources owned by someone else return **404**, same as missing ones.
- `PUT` replaces all writable fields. Optional fields omitted or blank become `null`.

| Status | Code | When |
| --- | --- | --- |
| 400 | `bad_request` | malformed JSON, unknown field, bad id or query parameter |
| 401 | `unauthorized` | missing/invalid/expired token |
| 404 | `not_found` | missing or not yours |
| 409 | `conflict` and specific codes | duplicates, company still referenced, invalid workflow transition, follow-up not eligible, Gmail state problems |
| 429 | `rate_limited` | too many requests |
| 502 / 503 | `gmail_error` / `gmail_not_configured` | Gmail problems |
| 413 | `payload_too_large` | body over 1 MiB |
| 422 | `validation_failed` | field errors in `details` |
| 500 | `internal_error` | unexpected |

## Health (no auth)

- `GET /healthz` -> `{"status":"ok"}`
- `GET /readyz` -> 200 `{"status":"ready"}` or 503 when the database ping fails

## Me and dashboard

- `GET /api/v1/me` -> `{id, email, display_name, created_at, updated_at}`
- `GET /api/v1/dashboard` ->

```json
{"data": {
  "companies": 3, "active_jobs": 5, "contacts": 8, "followups_due": 1,
  "job_contacts_by_status": {"ADDED": 2, "INVITATION_PENDING": 0, "CONNECTED": 1, "...": 0},
  "recent_activity": [{"id","job_contact_id","contact_name","event_type","from_status","to_status","created_at"}]
}}
```

`followups_due` counts job contacts in `CONNECTED` whose `accepted_at` is at least 24 hours old.

## Companies

`GET /companies?q=&page=&page_size=`, `POST /companies`, `GET|PUT|DELETE /companies/{id}`

Body: `{"name": "Acme", "website": "https://acme.com", "industry": "...", "location": "...", "notes": "..."}` (`name` required, max 200; `website` http(s) URL).
Response adds `id`, `job_count`, `contact_count`, `created_at`, `updated_at`.

## Jobs

`GET /jobs?company_id=&status=&q=&page=&page_size=`, `POST /jobs`, `GET|PUT|DELETE /jobs/{id}`

Body: `{"company_id": "<uuid>", "title": "Backend Engineer", "job_url": "https://...", "location": "...", "status": "ACTIVE", "notes": "..."}`
`status` is `ACTIVE` (default) or `CLOSED`. An unknown or foreign `company_id` returns 422 with `details.company_id`.
Response adds `id`, `company_name`, timestamps.

## Contacts

`GET /contacts?company_id=&q=&page=&page_size=`, `POST /contacts`, `GET|PUT|DELETE /contacts/{id}`

Body: `{"company_id": "<uuid|null>", "full_name": "Jane Doe", "linkedin_url": "https://www.linkedin.com/in/jane", "title": "...", "email": "...", "notes": "..."}`
`linkedin_url` is normalized (https, `www.linkedin.com`, lower-case path, no query/trailing slash); duplicates return 409.

## Job contacts (referral attempts)

- `POST /jobs/{id}/contacts` body `{"contact_id": "<uuid>", "notes": "..."}` -> 201 job contact in status `ADDED`. Unknown job: 404. Unknown/foreign contact: 422. Already linked: 409. Also writes a `JOB_CONTACT_CREATED` workflow event in the same transaction.
- `GET /job-contacts?job_id=&contact_id=&status=&page=&page_size=`
- `GET /job-contacts/{id}`
- `DELETE /job-contacts/{id}`
- `GET /job-contacts/{id}/events` -> audit trail, oldest first

Job contact response:

```json
{"id","job_id","job_title","company_id","company_name","contact_id","contact_name",
 "contact_linkedin_url","status","invitation_sent_at","accepted_at","message_sent_at",
 "followup_due_at","referral_received_at","notes","created_at","updated_at"}
```

- `PATCH /job-contacts/{id}` body `{"notes": "..."}`
- `POST /job-contacts/{id}/transitions` body `{"to_status": "INVITATION_PENDING", "occurred_at": "<optional RFC3339, not in the future>", "note": "<optional>"}` -> updated job contact. Invalid moves (not in the workflow table) return 409. The row is locked for the transition, the matching timestamp is set (`invitation_sent_at`, `accepted_at` plus `followup_due_at`, `message_sent_at`, `referral_received_at`) and a `STATUS_CHANGED` event is written, all in one transaction.

Every job contact response includes `allowed_next_statuses`.

## Follow-ups, messages, templates, settings

- `GET /followups/due?page=&page_size=`: `CONNECTED` job contacts whose accepted time is older than the follow-up delay (default 24h).
- `POST /job-contacts/{id}/followup` optional body `{"template_id": "<uuid>", "regenerate": false}` -> message draft. 201 when a draft is created (status moves `CONNECTED` to `MESSAGE_READY`), 200 when the existing draft is returned. Idempotent and concurrency-safe. 409 `followup_not_eligible` before the delay has passed or in other statuses. `regenerate: true` or `template_id` supersedes the current draft with a newly rendered one.
- `POST /job-contacts/{id}/followup/mark-sent` -> `{"message": {...}, "job_contact_status": "AWAITING_RESPONSE"}`. Call it after you sent the message yourself. Requires `MESSAGE_READY` and a draft.
- `GET /job-contacts/{id}/messages`
- `PUT /messages/{id}` body `{"body": "..."}` edits a DRAFT only.
- `GET|POST /templates`, `GET|PUT|DELETE /templates/{id}`: body `{"name","body","is_default"}`. Placeholders: `{{contact_name}}`, `{{contact_first_name}}`, `{{company}}`, `{{job_title}}`, `{{job_url}}`, `{{sender_name}}`; unknown placeholders are rejected. Setting `is_default` clears the previous default. Without a default the built-in text is used.
- `GET|PUT /settings`: `{"followup_delay_hours": 24, "sender_name": "..."}` (hours 1 to 720).

Message: `{"id","job_contact_id","template_id","kind":"FOLLOWUP","body","status":"DRAFT|SENT|SUPERSEDED","generated_at","sent_at","created_at","updated_at"}`

## Gmail

All under `/api/v1/gmail` except the callback. Without Google configuration they answer 503 `gmail_not_configured` (status answers `enabled: false`). See `docs/gmail.md`.

- `GET /gmail/status` -> `{enabled, connected, email_address, status, last_synced_at, recent_syncs[]}`
- `POST /gmail/connect` -> `{"url": "<Google consent URL>"}`
- `GET /oauth/gmail/callback` (public, root path, called by Google): redirects to `{FRONTEND_URL}/settings/gmail?status=...`
- `POST /gmail/sync` -> `{sync_id, processed, matched, unmatched, ignored, failed, has_more}`. 409 `gmail_not_connected`, `gmail_reauth_required`, or `conflict` when a sync is already running. 502 `gmail_error` when Gmail fails.
- `DELETE /gmail/connection`
- `GET /gmail/unmatched?status=PENDING|RESOLVED|DISMISSED&page=&page_size=`
- `POST /gmail/unmatched/{id}/resolve` body `{"contact_id": "<uuid>"}`; `POST /gmail/unmatched/{id}/dismiss`

## Rate limiting

Per authenticated user, in memory: 20 requests/second with a burst of 60 by default (`RATE_LIMIT_*`), and 0.2/second with a burst of 5 on `/gmail/*`. Exceeding it returns 429 `rate_limited` with `Retry-After: 1`.
