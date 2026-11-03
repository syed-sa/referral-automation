# Architecture

## Shape

A single Go service (modular monolith). No schedulers, no background workers, no queues: everything happens inside a request (rate limiter cleanup is lazy, not a goroutine).

```
Frontend (Vercel) --Supabase JWT--> Go API (Render) --pgx--> Supabase PostgreSQL
        \------------- Supabase Auth issues the JWTs -------------/
```

## Layering

`handler -> service -> repository -> sqlc queries`

- **handler** decodes the request, takes the user id from the context, calls the service, writes JSON. It never touches SQL.
- **service** validates and normalizes input and applies business rules.
- **repository** (small interface per feature, pgx/sqlc implementation) maps database errors to `apperr` errors.
- **sqlc** generates type-safe, parameterized queries from `sql/queries/*.sql`.

Dependencies are wired by constructor injection in `internal/server`.

## Packages

| Package | Responsibility |
| --- | --- |
| `internal/auth` | JWKS-based Supabase JWT verification, `Principal` in context |
| `internal/middleware` | authentication, user provisioning, request logging, recovery, security headers |
| `internal/user` | lazy `users` row provisioning, `GET /me` |
| `internal/company`, `job`, `contact` | CRUD |
| `internal/jobcontact` | links a contact to a job, listing, audit trail, creation event (transactional) |
| `internal/workflow` | status constants, the transition table and allowed-next helper |
| `internal/mailsync` | Gmail OAuth, manual sync, parser interface, conservative matching, review queue |
| `internal/followup` | template rendering, follow-up draft generation, templates, settings |
| `internal/secretbox` | AES-256-GCM encryption for refresh tokens |
| `internal/dashboard` | summary counts |
| `internal/platform/*` | config, errors, HTTP helpers, pagination, validation |
| `internal/database` | pool, migrations, Postgres error helpers |

## Request lifecycle

1. `RequestID`, `RequestLogger` (path only), `Recoverer`, `SecurityHeaders`, CORS (exact origins), timeout.
2. `Authenticate`: verifies signature, `iss`, `aud`, `exp`, `role=authenticated`, and parses `sub` as a UUID.
3. `ProvisionUser`: idempotent upsert of `users` (cached in memory after the first success).
4. Handler gets `userID` from the context and passes it explicitly to the service and repository.

## Errors and responses

Handlers return `error`; `httpx.Wrap` renders them in one place. Success: `{"data": ...}` (lists add `pagination`). Failure: `{"error": {"code", "message", "details?", "request_id"}}`. Unexpected errors are logged with the request id and returned as a generic 500.

## Workflow (state machine)

```
ADDED -> INVITATION_PENDING -> CONNECTED -> MESSAGE_READY -> MESSAGE_SENT -> AWAITING_RESPONSE -> REFERRAL_RECEIVED
                                                                                         \-> DECLINED
any status except CLOSED -> CLOSED
```

Job contacts start in `ADDED` with a `JOB_CONTACT_CREATED` event. **Every** status change goes through `jobcontact.ApplyTransition`, which must run inside a transaction: it locks the row (`SELECT ... FOR UPDATE`), validates with `workflow.CanTransition`, updates the status and the matching timestamp, and appends a `workflow_events` row. Manual transitions, Gmail matches, review-queue resolutions and follow-up generation all use it, so the audit trail is complete and concurrent changes serialize.

## Gmail and follow-ups

- **Sync** (`mailsync`): list Gmail ids, drop already processed ones, then per message one transaction writes the ledger row, applies transitions and stores any unmatched item. Re-running is harmless; overlapping syncs are blocked by a unique `RUNNING` row.
- **Follow-ups** (`followup`): generation locks the job contact, checks the delay, returns the existing draft if there is one (idempotent), otherwise renders the template (explicit, default or built-in), supersedes older drafts, and moves `CONNECTED` to `MESSAGE_READY`, all in one transaction.
- No scheduler: due follow-ups are a query (`GET /followups/due`) the user opens; nothing runs in the background.

## Decisions worth knowing

- `users.id` equals the Supabase user id but is not a foreign key to `auth.users`, so the schema also runs on plain PostgreSQL (local Docker).
- Statuses are `text` + `CHECK` constraints, not Postgres enums, to keep migrations simple.
- Transaction-pooler compatibility: pgx uses `QueryExecModeCacheDescribe`; migrations use a direct connection.
