-- Phase 1: foundation + multi-tenant core.
--
-- Tenancy rules enforced by this schema:
--   * every user-owned table has user_id UUID NOT NULL
--   * every parent/child relationship is a COMPOSITE foreign key
--     (user_id, parent_id) -> parent (user_id, id), so a row can never point at
--     another user's row, even if application code has a bug
--   * every parent exposes UNIQUE (user_id, id) to make that possible
-- Row Level Security is NOT enabled; the schema is RLS-compatible
-- (policy would simply be `user_id = auth.uid()`).

CREATE TABLE users (
    -- Same value as the Supabase auth user id (JWT `sub`). Deliberately NOT a
    -- foreign key to auth.users so the schema also runs on plain PostgreSQL.
    id           uuid PRIMARY KEY,
    email        text        NOT NULL DEFAULT '',
    display_name text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE companies (
    id         uuid        NOT NULL DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       text        NOT NULL,
    website    text,
    industry   text,
    location   text,
    notes      text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT companies_pkey PRIMARY KEY (id),
    CONSTRAINT companies_user_id_id_key UNIQUE (user_id, id),
    CONSTRAINT companies_name_not_blank CHECK (length(btrim(name)) > 0)
);
CREATE UNIQUE INDEX companies_user_name_uidx ON companies (user_id, lower(name));

CREATE TABLE jobs (
    id         uuid        NOT NULL DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    company_id uuid        NOT NULL,
    title      text        NOT NULL,
    job_url    text,
    location   text,
    status     text        NOT NULL DEFAULT 'ACTIVE',
    notes      text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT jobs_pkey PRIMARY KEY (id),
    CONSTRAINT jobs_user_id_id_key UNIQUE (user_id, id),
    CONSTRAINT jobs_company_fk FOREIGN KEY (user_id, company_id)
        REFERENCES companies (user_id, id),
    CONSTRAINT jobs_status_check CHECK (status IN ('ACTIVE', 'CLOSED')),
    CONSTRAINT jobs_title_not_blank CHECK (length(btrim(title)) > 0)
);
CREATE INDEX jobs_user_company_idx ON jobs (user_id, company_id);
CREATE INDEX jobs_user_created_idx ON jobs (user_id, created_at DESC);

CREATE TABLE contacts (
    id           uuid        NOT NULL DEFAULT gen_random_uuid(),
    user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    company_id   uuid,       -- optional; composite FK is skipped while NULL (MATCH SIMPLE)
    full_name    text        NOT NULL,
    linkedin_url text,
    title        text,
    email        text,
    notes        text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT contacts_pkey PRIMARY KEY (id),
    CONSTRAINT contacts_user_id_id_key UNIQUE (user_id, id),
    CONSTRAINT contacts_company_fk FOREIGN KEY (user_id, company_id)
        REFERENCES companies (user_id, id),
    CONSTRAINT contacts_name_not_blank CHECK (length(btrim(full_name)) > 0)
);
CREATE UNIQUE INDEX contacts_user_linkedin_uidx
    ON contacts (user_id, linkedin_url) WHERE linkedin_url IS NOT NULL;
CREATE INDEX contacts_user_company_idx ON contacts (user_id, company_id);
CREATE INDEX contacts_user_created_idx ON contacts (user_id, created_at DESC);

-- A job_contact is one referral attempt: "ask <contact> for a referral for <job>".
-- It carries the workflow state machine (see internal/workflow).
CREATE TABLE job_contacts (
    id                   uuid        NOT NULL DEFAULT gen_random_uuid(),
    user_id              uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    job_id               uuid        NOT NULL,
    contact_id           uuid        NOT NULL,
    status               text        NOT NULL DEFAULT 'ADDED',
    invitation_sent_at   timestamptz,
    accepted_at          timestamptz,
    message_sent_at      timestamptz,
    followup_due_at      timestamptz,
    referral_received_at timestamptz,
    notes                text,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT job_contacts_pkey PRIMARY KEY (id),
    CONSTRAINT job_contacts_user_id_id_key UNIQUE (user_id, id),
    CONSTRAINT job_contacts_user_job_contact_key UNIQUE (user_id, job_id, contact_id),
    CONSTRAINT job_contacts_job_fk FOREIGN KEY (user_id, job_id)
        REFERENCES jobs (user_id, id) ON DELETE CASCADE,
    CONSTRAINT job_contacts_contact_fk FOREIGN KEY (user_id, contact_id)
        REFERENCES contacts (user_id, id) ON DELETE CASCADE,
    CONSTRAINT job_contacts_status_check CHECK (status IN (
        'ADDED', 'INVITATION_PENDING', 'CONNECTED', 'MESSAGE_READY',
        'MESSAGE_SENT', 'AWAITING_RESPONSE', 'REFERRAL_RECEIVED', 'DECLINED', 'CLOSED'))
);
CREATE INDEX job_contacts_user_status_idx ON job_contacts (user_id, status);
CREATE INDEX job_contacts_user_job_idx ON job_contacts (user_id, job_id);
CREATE INDEX job_contacts_user_contact_idx ON job_contacts (user_id, contact_id);
CREATE INDEX job_contacts_user_created_idx ON job_contacts (user_id, created_at DESC);

-- Append-only audit trail; every status transition writes one row.
CREATE TABLE workflow_events (
    id             uuid        NOT NULL DEFAULT gen_random_uuid(),
    user_id        uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    job_contact_id uuid        NOT NULL,
    event_type     text        NOT NULL,
    from_status    text,
    to_status      text,
    metadata       jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workflow_events_pkey PRIMARY KEY (id),
    CONSTRAINT workflow_events_job_contact_fk FOREIGN KEY (user_id, job_contact_id)
        REFERENCES job_contacts (user_id, id) ON DELETE CASCADE
);
CREATE INDEX workflow_events_user_created_idx ON workflow_events (user_id, created_at DESC);
CREATE INDEX workflow_events_user_job_contact_idx
    ON workflow_events (user_id, job_contact_id, created_at);
