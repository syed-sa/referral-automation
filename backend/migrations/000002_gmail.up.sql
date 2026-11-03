-- Phase 3: Gmail (read-only) integration.
-- Same tenancy rules as 000001: user_id on every row, composite FKs.

CREATE TABLE email_accounts (
    id                      uuid        NOT NULL DEFAULT gen_random_uuid(),
    user_id                 uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider                text        NOT NULL DEFAULT 'gmail',
    email_address           text        NOT NULL,
    -- AES-256-GCM: nonce || ciphertext, bound to user_id as AAD. Never plaintext.
    encrypted_refresh_token bytea       NOT NULL,
    scope                   text        NOT NULL,
    status                  text        NOT NULL DEFAULT 'ACTIVE',
    last_synced_at          timestamptz,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT email_accounts_pkey PRIMARY KEY (id),
    CONSTRAINT email_accounts_user_id_id_key UNIQUE (user_id, id),
    CONSTRAINT email_accounts_user_address_key UNIQUE (user_id, email_address),
    CONSTRAINT email_accounts_provider_check CHECK (provider = 'gmail'),
    CONSTRAINT email_accounts_status_check CHECK (status IN ('ACTIVE', 'NEEDS_REAUTH'))
);

-- One row per started OAuth flow. The random state is stored hashed and is
-- single-use with a short expiry; it also binds the callback to the user who
-- started the flow (the callback itself carries no Authorization header).
CREATE TABLE gmail_oauth_states (
    state_hash bytea       PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX gmail_oauth_states_expires_idx ON gmail_oauth_states (expires_at);

CREATE TABLE gmail_sync_history (
    id               uuid        NOT NULL DEFAULT gen_random_uuid(),
    user_id          uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    email_account_id uuid        NOT NULL,
    status           text        NOT NULL DEFAULT 'RUNNING',
    started_at       timestamptz NOT NULL DEFAULT now(),
    finished_at      timestamptz,
    messages_seen    integer     NOT NULL DEFAULT 0,
    matched          integer     NOT NULL DEFAULT 0,
    unmatched        integer     NOT NULL DEFAULT 0,
    ignored          integer     NOT NULL DEFAULT 0,
    error            text,
    CONSTRAINT gmail_sync_history_pkey PRIMARY KEY (id),
    CONSTRAINT gmail_sync_history_account_fk FOREIGN KEY (user_id, email_account_id)
        REFERENCES email_accounts (user_id, id) ON DELETE CASCADE,
    CONSTRAINT gmail_sync_history_status_check CHECK (status IN ('RUNNING', 'SUCCEEDED', 'FAILED'))
);
-- At most one running sync per account: makes concurrent manual syncs safe.
CREATE UNIQUE INDEX gmail_sync_one_running_uidx
    ON gmail_sync_history (user_id, email_account_id) WHERE status = 'RUNNING';
CREATE INDEX gmail_sync_history_user_started_idx ON gmail_sync_history (user_id, started_at DESC);

-- Idempotency ledger: a Gmail message id is processed at most once per user.
CREATE TABLE processed_gmail_messages (
    id               uuid        NOT NULL DEFAULT gen_random_uuid(),
    user_id          uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    email_account_id uuid        NOT NULL,
    gmail_message_id text        NOT NULL,
    outcome          text        NOT NULL,
    processed_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT processed_gmail_messages_pkey PRIMARY KEY (id),
    CONSTRAINT processed_gmail_messages_user_message_key UNIQUE (user_id, gmail_message_id),
    CONSTRAINT processed_gmail_messages_account_fk FOREIGN KEY (user_id, email_account_id)
        REFERENCES email_accounts (user_id, id) ON DELETE CASCADE,
    CONSTRAINT processed_gmail_messages_outcome_check CHECK (outcome IN ('MATCHED', 'UNMATCHED', 'IGNORED'))
);

-- Review queue for acceptance emails that could not be matched confidently.
-- Only headers and the extracted name/URL are stored, never the email body.
CREATE TABLE unmatched_gmail_emails (
    id                    uuid        NOT NULL DEFAULT gen_random_uuid(),
    user_id               uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    email_account_id      uuid        NOT NULL,
    gmail_message_id      text        NOT NULL,
    from_address          text        NOT NULL,
    subject               text        NOT NULL,
    received_at           timestamptz NOT NULL,
    extracted_name        text,
    extracted_profile_url text,
    reason                text        NOT NULL,
    status                text        NOT NULL DEFAULT 'PENDING',
    resolved_at           timestamptz,
    created_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT unmatched_gmail_emails_pkey PRIMARY KEY (id),
    CONSTRAINT unmatched_gmail_emails_user_id_id_key UNIQUE (user_id, id),
    CONSTRAINT unmatched_gmail_emails_user_message_key UNIQUE (user_id, gmail_message_id),
    CONSTRAINT unmatched_gmail_emails_account_fk FOREIGN KEY (user_id, email_account_id)
        REFERENCES email_accounts (user_id, id) ON DELETE CASCADE,
    CONSTRAINT unmatched_gmail_emails_reason_check CHECK (reason IN ('NO_CANDIDATE', 'AMBIGUOUS')),
    CONSTRAINT unmatched_gmail_emails_status_check CHECK (status IN ('PENDING', 'RESOLVED', 'DISMISSED'))
);
CREATE INDEX unmatched_gmail_emails_user_status_idx
    ON unmatched_gmail_emails (user_id, status, received_at DESC);
