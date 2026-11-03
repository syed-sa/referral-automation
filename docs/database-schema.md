# Database schema

Migrations: `000001_init` (core), `000002_gmail` (Gmail), `000003_followups` (templates, messages, settings).

## ER diagram

```mermaid
erDiagram
    users ||--o{ companies : owns
    users ||--o{ jobs : owns
    users ||--o{ contacts : owns
    users ||--o{ job_contacts : owns
    users ||--o{ workflow_events : owns
    companies ||--o{ jobs : "has (user_id, company_id)"
    companies |o--o{ contacts : "optional (user_id, company_id)"
    jobs ||--o{ job_contacts : "(user_id, job_id)"
    contacts ||--o{ job_contacts : "(user_id, contact_id)"
    job_contacts ||--o{ workflow_events : "(user_id, job_contact_id)"
    job_contacts ||--o{ messages : "(user_id, job_contact_id)"
    message_templates |o--o{ messages : "optional, SET NULL"
    users ||--o| application_settings : has
    users ||--o{ message_templates : owns
    users ||--o{ email_accounts : owns
    email_accounts ||--o{ gmail_sync_history : "(user_id, email_account_id)"
    email_accounts ||--o{ processed_gmail_messages : "(user_id, email_account_id)"
    email_accounts ||--o{ unmatched_gmail_emails : "(user_id, email_account_id)"
    users ||--o{ gmail_oauth_states : starts

    users {
        uuid id PK
        text email
        text display_name
    }
    companies {
        uuid id PK
        uuid user_id FK
        text name
        text website
    }
    jobs {
        uuid id PK
        uuid user_id FK
        uuid company_id
        text title
        text status
    }
    contacts {
        uuid id PK
        uuid user_id FK
        uuid company_id
        text full_name
        text linkedin_url
    }
    job_contacts {
        uuid id PK
        uuid user_id FK
        uuid job_id
        uuid contact_id
        text status
        timestamptz accepted_at
    }
    workflow_events {
        uuid id PK
        uuid user_id FK
        uuid job_contact_id
        text event_type
        text from_status
        text to_status
    }
    messages {
        uuid id PK
        uuid user_id FK
        uuid job_contact_id
        uuid template_id
        text body
        text status
    }
    message_templates {
        uuid id PK
        uuid user_id FK
        text name
        text body
        bool is_default
    }
    application_settings {
        uuid user_id PK
        int followup_delay_hours
        text sender_name
    }
    email_accounts {
        uuid id PK
        uuid user_id FK
        text email_address
        bytea encrypted_refresh_token
        text status
    }
    gmail_oauth_states {
        bytea state_hash PK
        uuid user_id FK
        timestamptz expires_at
        timestamptz used_at
    }
    gmail_sync_history {
        uuid id PK
        uuid user_id FK
        uuid email_account_id
        text status
    }
    processed_gmail_messages {
        uuid id PK
        uuid user_id FK
        uuid email_account_id
        text gmail_message_id
        text outcome
    }
    unmatched_gmail_emails {
        uuid id PK
        uuid user_id FK
        uuid email_account_id
        text gmail_message_id
        text reason
        text status
    }
```

## Tenancy guarantees

- Every user-owned table has `user_id UUID NOT NULL REFERENCES users(id)`.
- Every parent table has `UNIQUE (user_id, id)`.
- Every relationship is a composite foreign key `(user_id, parent_id) -> parent(user_id, id)`: a child can never reference another user's row, even if the application is wrong.
- Every SQL statement filters by `user_id`, including joins (`ON c.user_id = j.user_id AND c.id = j.company_id`).
- RLS is not enabled but the schema is compatible with `USING (user_id = auth.uid())` policies.

## Uniqueness and constraints

| Constraint | Meaning |
| --- | --- |
| `companies_user_name_uidx` | company names unique per user, case-insensitive |
| `contacts_user_linkedin_uidx` | one contact per LinkedIn URL per user (partial, URL not null) |
| `job_contacts_user_job_contact_key` | a contact can be linked to a job once |
| `*_status_check` | status values restricted to the allowed set |
| `gmail_sync_one_running_uidx` | one RUNNING sync per account (concurrency guard) |
| `processed_gmail_messages_user_message_key` | a Gmail message id is processed once per user (idempotency) |
| `unmatched_gmail_emails_user_message_key` | one review-queue row per Gmail message |
| `messages_one_draft_uidx` | at most one active DRAFT per job contact (idempotent generation) |
| `message_templates_one_default_uidx` | at most one default template per user |
| `email_accounts_user_address_key` | one stored account per mailbox per user |

## Delete behavior

| Delete | Effect |
| --- | --- |
| company with jobs or contacts | blocked (409) |
| job | cascades to its job_contacts and their events |
| contact | cascades to its job_contacts and their events |
| job_contact | cascades to its events |
| template | messages made from it are kept (`template_id` becomes NULL; PostgreSQL 15+) |
| email account | cascades to sync history, processed ledger and review queue |
| user | cascades to everything the user owns |

## Indexes

All list queries are served by `(user_id, ...)` indexes: `jobs(user_id, company_id)`, `jobs(user_id, created_at desc)`, `contacts(user_id, company_id)`, `job_contacts(user_id, status)`, `job_contacts(user_id, job_id)`, `workflow_events(user_id, created_at desc)`, and others in the migration.
