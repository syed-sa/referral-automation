# Security

## Authentication

- Supabase Auth issues the access token; the Go API verifies it. No custom login.
- Verification (`internal/auth/verifier.go`): signature, `iss` = `{SUPABASE_URL}/auth/v1`, `aud` = `authenticated`, `exp` required (10 s leeway), `role` = `authenticated`, `sub` must be a UUID.
- The signing algorithm is not hard-coded. Asymmetric tokens (ES256, RS256, EdDSA and variants) are verified with the key from the project's JWKS, looked up by `kid`. The JWKS is cached for 10 minutes, refreshed when an unknown `kid` appears (at most once every 30 s), and a stale key is still served if a refresh fails.
- HS256 is accepted **only** if `SUPABASE_JWT_SECRET` is set (legacy projects, local dev). The allowed-algorithm list is fixed per verifier, `none` is never allowed, and an HMAC token can never be verified against a public key, which prevents algorithm-confusion attacks.
- Tokens are never logged; rejection logs contain only a reason.

## Multi-tenant isolation (defense in depth)

1. **Identity**: the user id comes only from the validated token (`auth.UserID(ctx)`). It is never read from path, query or body, and request bodies that include `user_id` are rejected (unknown fields are an error).
2. **Explicit scoping**: services and repositories take `userID` as a parameter (`List(ctx, userID, ...)`); no global or ambient user.
3. **SQL**: every statement has `user_id = $n`, including every join condition. Updates and deletes are `WHERE id = $1 AND user_id = $2`, and a zero row count becomes 404.
4. **Schema**: composite FKs `(user_id, id)` make cross-tenant references impossible even if 1 to 3 fail.
5. **404 over 403**: another tenant's resource looks identical to a missing one.
6. **RLS-ready**: not enabled (the API connects with a privileged role), but policies would be a one-liner per table.

## Secrets and configuration

- Only `.env.example` with placeholders is committed; `.env` is git-ignored.
- The Supabase service-role key is not used by this backend and must never reach the frontend.
- `SUPABASE_JWT_SECRET` should be empty in production when asymmetric signing keys are in use.
- Config validation fails fast at startup; in production `ALLOWED_ORIGINS` is mandatory and `*` is rejected.
- Gmail refresh tokens are stored AES-256-GCM encrypted, with the owning user id as additional authenticated data (a ciphertext copied to another user's row will not decrypt). `TOKEN_ENCRYPTION_KEY` lives only in the environment.
- OAuth tokens, authorization codes and email bodies are never logged. Email bodies are parsed in memory and never stored; only headers and the extracted name and URL go to the review queue.

## Gmail OAuth

- Scope is `gmail.readonly` only, and the callback refuses a grant that lacks it.
- `state` is 32 random bytes, stored as a SHA-256 hash, single-use, valid for 10 minutes, and bound to the user who started the flow. The public callback has no JWT, so this binding is what identifies the user.
- Sync matching is conservative: a state change needs exactly one candidate among contacts with a pending invitation; everything else goes to a human review queue.
- Sync is idempotent (processed ledger) and concurrency-safe (one running sync per account).

## HTTP hardening

- CORS: exact allowed origins only, no credentials, headers limited to `Authorization` and `Content-Type`.
- Security headers: `nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, `Cache-Control: no-store`.
- Body limit 1 MiB, strict JSON decoding, request timeout, server read/write/idle timeouts, graceful shutdown.
- Access logs record the path only, never query strings, headers or bodies.
- All SQL is parameterized (sqlc). `ILIKE` search input has `%`, `_` and `\` escaped.
- Panics are recovered to a generic 500 with the stack in logs only.

- Per-user in-memory rate limiting (stricter on `/gmail/*`); the limiter is per instance, which matches the single-instance design.

## Known gaps

- The database connection uses a privileged role, so RLS is not a second line of defense until enabled.
- The email parser is provisional and does not yet check the DKIM result of the message (needs a real sample).
- No automated tests yet.
- The code has not been compiled or run against a real Supabase project yet.
