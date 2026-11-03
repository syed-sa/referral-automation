# LinkedIn Referral Automation Platform

A multi-tenant tracker for LinkedIn referral outreach. You add companies, jobs and contacts, link contacts to jobs, and track every referral request through a workflow. **There is no LinkedIn automation or scraping**: you do the LinkedIn actions yourself and the app tracks them.

The backend (Go, `backend/`) is complete for all five phases and the frontend (React, `frontend/`) is built. This is a monorepo: the API deploys to Render and the frontend to Vercel, independently.

| Phase | Scope | Status |
| --- | --- | --- |
| 1 | Foundation, Supabase JWT auth, multi-tenant schema, company/job/contact CRUD, job-contact linking, dashboard | written |
| 2 | Invitation tracking: workflow transitions with audit events | written |
| 3 | Gmail (read-only) connect and manual sync, conservative matching, unmatched review queue | written |
| 4 | Follow-up generation (templates, 24h rule), settings | written |
| 5 | Dockerfile, Render blueprint, CI, rate limiting, deployment docs | written |

> **Nothing has been compiled or tested yet.** The code was written in an environment that could not download Go modules. Follow "First run" below and send back any error output.

## Stack

Go, Chi, pgx, sqlc, golang-migrate, slog. Supabase PostgreSQL and Supabase Auth. Backend on Render. Frontend: React, TypeScript, Vite, Tailwind, shadcn/ui, TanStack Query, Supabase Auth, on Vercel (see `frontend/README.md`).

## Layout

```
frontend/            React app (Vercel)
backend/
  cmd/api            HTTP server
  cmd/migrate        migration runner (up / down / version)
  cmd/devtoken       local-only HS256 token minting
  migrations/        000001 core, 000002 gmail, 000003 follow-ups
  sql/queries/       sqlc query files
  internal/db/       sqlc output (generated, commit it)
  internal/...       auth, middleware, platform helpers, feature packages
docs/                architecture, database-schema, api-contracts, security, gmail, deployment
render.yaml          Render blueprint
docker-compose.yml   local Postgres
```

## First run

`internal/db` is produced by sqlc and does not exist yet:

```bash
cd backend
go mod tidy          # resolves versions and writes go.sum (commit go.sum, the Dockerfile needs it)
sqlc generate        # sqlc 1.27 or newer; creates internal/db
go build ./...       # send me any errors
```

## Run locally (no Supabase project needed)

```bash
docker compose up -d                    # Postgres on :5432 (repo root)
cp backend/.env.example backend/.env
cd backend
go run ./cmd/migrate up
go run ./cmd/api
```

Minimal `.env`:

```
DATABASE_URL=postgres://postgres:postgres@localhost:5432/referral?sslmode=disable
SUPABASE_URL=http://localhost:54321
SUPABASE_JWT_SECRET=some-long-random-local-secret
```

Try it:

```bash
TOKEN=$(go run ./cmd/devtoken -sub 11111111-1111-1111-1111-111111111111)
curl -s localhost:8080/healthz
curl -s -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/me
curl -s -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Acme"}' localhost:8080/api/v1/companies
```

Use a second `-sub` to confirm users cannot see each other's data (you get 404s).

## The referral workflow

```
ADDED -> INVITATION_PENDING -> CONNECTED -> MESSAGE_READY -> MESSAGE_SENT -> AWAITING_RESPONSE -> REFERRAL_RECEIVED | DECLINED
(any status except CLOSED) -> CLOSED
```

1. Add a contact to a job: `POST /api/v1/jobs/{id}/contacts` (status `ADDED`).
2. After you send the LinkedIn invitation yourself: `POST /api/v1/job-contacts/{id}/transitions` with `{"to_status":"INVITATION_PENDING"}`.
3. When they accept: mark it manually (`CONNECTED`) or run a Gmail sync (`POST /api/v1/gmail/sync`).
4. After the follow-up delay (24h by default): `POST /api/v1/job-contacts/{id}/followup` generates a draft and moves the status to `MESSAGE_READY`.
5. You send the message yourself, then `POST /api/v1/job-contacts/{id}/followup/mark-sent` moves it to `AWAITING_RESPONSE`.
6. Finish with `REFERRAL_RECEIVED`, `DECLINED` or `CLOSED` via transitions.

## Docs

- `docs/architecture.md`
- `docs/database-schema.md` (ER diagram)
- `docs/api-contracts.md`
- `docs/security.md`
- `docs/gmail.md` (Google setup, matching rules, the parser sample request)
- `docs/deployment.md` (Supabase, Render, Vercel)
- `frontend/README.md` (local run, environment variables, Vercel settings)
