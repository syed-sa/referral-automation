# Gmail integration

Read-only, manual, and optional. The app never sends, deletes or modifies email.

## Google setup

1. In Google Cloud Console create a project and enable the **Gmail API**.
2. Configure the OAuth consent screen. Request only `https://www.googleapis.com/auth/gmail.readonly`. While the app is in "Testing", add yourself as a test user (refresh tokens of testing apps expire after 7 days; publish the app to avoid that).
3. Create an OAuth client of type **Web application**. Authorized redirect URI: `https://<api-host>/oauth/gmail/callback` (and `http://localhost:8080/oauth/gmail/callback` for local use).
4. Set `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`, `TOKEN_ENCRYPTION_KEY` (`openssl rand -base64 32`) and `FRONTEND_URL`. All four Google/encryption values are required together; without them the Gmail endpoints answer 503.

## Connect flow

1. The frontend calls `POST /api/v1/gmail/connect` (authenticated) and receives `{"url": ...}`.
2. The browser goes to Google. A random 32-byte `state` was stored **hashed**, single-use, valid 10 minutes, and bound to the user who started the flow.
3. Google redirects to `GET /oauth/gmail/callback?code&state` (no Authorization header). The backend consumes the state (unknown, reused or expired states are rejected), exchanges the code, checks that `gmail.readonly` was granted and a refresh token was returned, reads the mailbox address, encrypts the refresh token with AES-256-GCM (additional data = the user id) and stores it.
4. The browser is redirected to `{FRONTEND_URL}/settings/gmail?status=connected` (other statuses: `denied`, `invalid_state`, `exchange_failed`, `no_refresh_token`, `scope_missing`, `error`).

`DELETE /api/v1/gmail/connection` revokes the token at Google (best effort) and deletes the stored account, its sync history, processed-message ledger and review queue.

## Sync

`POST /api/v1/gmail/sync` runs synchronously inside the request (there are no background workers).

- Searches `from:linkedin.com after:<epoch>`: the first sync looks back `GMAIL_LOOKBACK_DAYS`, later syncs start 2 days before the previous sync.
- Handles at most `GMAIL_SYNC_MAX_MESSAGES` new messages per call; `has_more: true` means call again.
- **Idempotent**: each Gmail message id is written to `processed_gmail_messages` (unique per user) in the same transaction as its effects, so re-running or overlapping windows never apply anything twice.
- **Concurrency-safe**: a partial unique index allows one `RUNNING` sync per account; a second call gets 409. A sync stuck for 10 minutes is marked failed automatically.
- A message that fails to fetch or process is not recorded, so the next sync retries it. If Google reports `invalid_grant`, the account is marked `NEEDS_REAUTH` and the API answers 409 `gmail_reauth_required`.
- Message bodies are parsed in memory only. Only headers and the extracted name/URL are stored (review queue); nothing from the body is logged.

## Matching rules (conservative on purpose)

An acceptance email changes state only when **exactly one** contact fits among contacts that have an `INVITATION_PENDING` job contact:

1. If the email contains a LinkedIn profile URL, only an exact match on the normalized URL counts. A URL that matches nothing is never rescued by name.
2. Otherwise the extracted name must equal exactly one contact's full name (case and whitespace insensitive).

All pending invitations of the matched contact move to `CONNECTED` (with the email's received time as `accepted_at`). Everything else that looks like an acceptance goes to the review queue with reason `NO_CANDIDATE` or `AMBIGUOUS`. Emails that are not acceptances are recorded as `IGNORED`.

## Review queue

- `GET /api/v1/gmail/unmatched?status=PENDING`
- `POST /api/v1/gmail/unmatched/{id}/resolve` with `{"contact_id": "<uuid>"}` links the email to a contact and applies the transition.
- `POST /api/v1/gmail/unmatched/{id}/dismiss`

## The parser is provisional

`mailsync.Parser` is an interface. The shipped `LinkedInAcceptanceParser` was written **without a real sample**: it requires a sender at `linkedin.com` and a subject like `<name> accepted your invitation`, and looks for a `linkedin.com/in/...` link in the body. If the real format differs, acceptances are ignored or land in the review queue; nothing is applied wrongly.

**To write the exact parser, send a sanitized real LinkedIn acceptance email** (raw headers and body, with names, URLs and ids replaced by placeholders, but the structure intact). Hardening to add at the same time: verifying the `Authentication-Results` DKIM result for linkedin.com.
