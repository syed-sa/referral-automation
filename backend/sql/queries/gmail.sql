-- name: CreateOAuthState :exec
INSERT INTO gmail_oauth_states (state_hash, user_id, expires_at)
VALUES (sqlc.arg(state_hash), sqlc.arg(user_id), sqlc.arg(expires_at));

-- name: ConsumeOAuthState :one
-- Single use and time limited. Returns the user who started the flow.
UPDATE gmail_oauth_states
SET used_at = now()
WHERE state_hash = sqlc.arg(state_hash)
  AND used_at IS NULL
  AND expires_at > now()
RETURNING user_id;

-- name: DeleteExpiredOAuthStates :exec
DELETE FROM gmail_oauth_states WHERE expires_at < now() - interval '1 day';

-- name: UpsertEmailAccount :one
INSERT INTO email_accounts (user_id, email_address, encrypted_refresh_token, scope)
VALUES (sqlc.arg(user_id), sqlc.arg(email_address), sqlc.arg(encrypted_refresh_token), sqlc.arg(scope))
ON CONFLICT (user_id, email_address) DO UPDATE
SET encrypted_refresh_token = EXCLUDED.encrypted_refresh_token,
    scope = EXCLUDED.scope,
    status = 'ACTIVE',
    updated_at = now()
RETURNING id;

-- name: GetEmailAccount :one
SELECT id, email_address, encrypted_refresh_token, scope, status, last_synced_at, created_at
FROM email_accounts
WHERE user_id = sqlc.arg(user_id)
ORDER BY created_at ASC
LIMIT 1;

-- name: SetEmailAccountStatus :exec
UPDATE email_accounts
SET status = sqlc.arg(status), updated_at = now()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: SetEmailAccountSynced :exec
UPDATE email_accounts
SET last_synced_at = sqlc.arg(last_synced_at), updated_at = now()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: DeleteEmailAccount :execrows
DELETE FROM email_accounts
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: FailStaleSyncs :exec
-- A crashed sync must not block future syncs forever.
UPDATE gmail_sync_history
SET status = 'FAILED', finished_at = now(), error = 'sync timed out'
WHERE user_id = sqlc.arg(user_id)
  AND status = 'RUNNING'
  AND started_at < now() - interval '10 minutes';

-- name: StartSync :one
-- The partial unique index allows only one RUNNING row per account (23505 otherwise).
INSERT INTO gmail_sync_history (user_id, email_account_id)
VALUES (sqlc.arg(user_id), sqlc.arg(email_account_id))
RETURNING id;

-- name: FinishSync :exec
UPDATE gmail_sync_history
SET status = sqlc.arg(status),
    finished_at = now(),
    messages_seen = sqlc.arg(messages_seen),
    matched = sqlc.arg(matched),
    unmatched = sqlc.arg(unmatched),
    ignored = sqlc.arg(ignored),
    error = sqlc.arg(error)
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: ListSyncHistory :many
SELECT id, status, started_at, finished_at, messages_seen, matched, unmatched, ignored, error
FROM gmail_sync_history
WHERE user_id = sqlc.arg(user_id)
ORDER BY started_at DESC
LIMIT sqlc.arg(row_limit)::int;

-- name: FilterProcessedMessages :many
-- Returns the subset of ids that were already processed.
SELECT gmail_message_id
FROM processed_gmail_messages
WHERE user_id = sqlc.arg(user_id)
  AND gmail_message_id = ANY(sqlc.arg(message_ids)::text[]);

-- name: InsertProcessedMessage :execrows
-- 0 rows affected means another sync already handled this message.
INSERT INTO processed_gmail_messages (user_id, email_account_id, gmail_message_id, outcome)
VALUES (sqlc.arg(user_id), sqlc.arg(email_account_id), sqlc.arg(gmail_message_id), sqlc.arg(outcome))
ON CONFLICT (user_id, gmail_message_id) DO NOTHING;

-- name: InsertUnmatchedEmail :exec
INSERT INTO unmatched_gmail_emails (user_id, email_account_id, gmail_message_id, from_address, subject,
                                    received_at, extracted_name, extracted_profile_url, reason)
VALUES (sqlc.arg(user_id), sqlc.arg(email_account_id), sqlc.arg(gmail_message_id), sqlc.arg(from_address),
        sqlc.arg(subject), sqlc.arg(received_at), sqlc.arg(extracted_name), sqlc.arg(extracted_profile_url),
        sqlc.arg(reason))
ON CONFLICT (user_id, gmail_message_id) DO NOTHING;

-- name: GetUnmatchedEmail :one
SELECT id, from_address, subject, received_at, extracted_name, extracted_profile_url, reason, status, resolved_at, created_at
FROM unmatched_gmail_emails
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: ListUnmatchedEmails :many
SELECT id, from_address, subject, received_at, extracted_name, extracted_profile_url, reason, status, resolved_at, created_at
FROM unmatched_gmail_emails
WHERE user_id = sqlc.arg(user_id)
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
ORDER BY received_at DESC, id DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountUnmatchedEmails :one
SELECT count(*)::bigint
FROM unmatched_gmail_emails
WHERE user_id = sqlc.arg(user_id)
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text);

-- name: ResolveUnmatchedEmail :execrows
-- Only PENDING items can be resolved or dismissed (idempotent against double clicks).
UPDATE unmatched_gmail_emails
SET status = sqlc.arg(status), resolved_at = now()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND status = 'PENDING';
