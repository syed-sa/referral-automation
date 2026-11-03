-- Phase 4: follow-up message generation, templates and per-user settings.

CREATE TABLE application_settings (
    user_id              uuid        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    followup_delay_hours integer     NOT NULL DEFAULT 24,
    sender_name          text,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT application_settings_delay_check CHECK (followup_delay_hours BETWEEN 1 AND 720)
);

CREATE TABLE message_templates (
    id         uuid        NOT NULL DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       text        NOT NULL,
    kind       text        NOT NULL DEFAULT 'FOLLOWUP',
    body       text        NOT NULL,
    is_default boolean     NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT message_templates_pkey PRIMARY KEY (id),
    CONSTRAINT message_templates_user_id_id_key UNIQUE (user_id, id),
    CONSTRAINT message_templates_kind_check CHECK (kind = 'FOLLOWUP'),
    CONSTRAINT message_templates_name_not_blank CHECK (length(btrim(name)) > 0),
    CONSTRAINT message_templates_body_not_blank CHECK (length(btrim(body)) > 0)
);
CREATE UNIQUE INDEX message_templates_user_name_uidx ON message_templates (user_id, lower(name));
-- At most one default template per user and kind.
CREATE UNIQUE INDEX message_templates_one_default_uidx
    ON message_templates (user_id, kind) WHERE is_default;

CREATE TABLE messages (
    id             uuid        NOT NULL DEFAULT gen_random_uuid(),
    user_id        uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    job_contact_id uuid        NOT NULL,
    template_id    uuid,
    kind           text        NOT NULL DEFAULT 'FOLLOWUP',
    body           text        NOT NULL,
    status         text        NOT NULL DEFAULT 'DRAFT',
    generated_at   timestamptz NOT NULL DEFAULT now(),
    sent_at        timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT messages_pkey PRIMARY KEY (id),
    CONSTRAINT messages_user_id_id_key UNIQUE (user_id, id),
    CONSTRAINT messages_job_contact_fk FOREIGN KEY (user_id, job_contact_id)
        REFERENCES job_contacts (user_id, id) ON DELETE CASCADE,
    -- Deleting a template keeps the messages generated from it (PostgreSQL 15+ syntax).
    CONSTRAINT messages_template_fk FOREIGN KEY (user_id, template_id)
        REFERENCES message_templates (user_id, id) ON DELETE SET NULL (template_id),
    CONSTRAINT messages_kind_check CHECK (kind = 'FOLLOWUP'),
    CONSTRAINT messages_status_check CHECK (status IN ('DRAFT', 'SENT', 'SUPERSEDED'))
);
-- Idempotency backstop: at most one active draft per job contact and kind.
CREATE UNIQUE INDEX messages_one_draft_uidx
    ON messages (user_id, job_contact_id, kind) WHERE status = 'DRAFT';
CREATE INDEX messages_user_job_contact_idx ON messages (user_id, job_contact_id, created_at DESC);
