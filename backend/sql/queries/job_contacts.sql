-- name: CreateJobContact :one
INSERT INTO job_contacts (user_id, job_id, contact_id, notes)
VALUES (sqlc.arg(user_id), sqlc.arg(job_id), sqlc.arg(contact_id), sqlc.arg(notes))
RETURNING id;

-- name: GetJobContact :one
SELECT jc.id, jc.job_id, j.title AS job_title, j.company_id, c.name AS company_name,
       jc.contact_id, ct.full_name AS contact_name, ct.linkedin_url AS contact_linkedin_url,
       jc.status, jc.invitation_sent_at, jc.accepted_at, jc.message_sent_at,
       jc.followup_due_at, jc.referral_received_at, jc.notes, jc.created_at, jc.updated_at
FROM job_contacts jc
JOIN jobs j ON j.user_id = jc.user_id AND j.id = jc.job_id
JOIN companies c ON c.user_id = j.user_id AND c.id = j.company_id
JOIN contacts ct ON ct.user_id = jc.user_id AND ct.id = jc.contact_id
WHERE jc.id = sqlc.arg(id) AND jc.user_id = sqlc.arg(user_id);

-- name: ListJobContacts :many
SELECT jc.id, jc.job_id, j.title AS job_title, j.company_id, c.name AS company_name,
       jc.contact_id, ct.full_name AS contact_name, ct.linkedin_url AS contact_linkedin_url,
       jc.status, jc.invitation_sent_at, jc.accepted_at, jc.message_sent_at,
       jc.followup_due_at, jc.referral_received_at, jc.notes, jc.created_at, jc.updated_at
FROM job_contacts jc
JOIN jobs j ON j.user_id = jc.user_id AND j.id = jc.job_id
JOIN companies c ON c.user_id = j.user_id AND c.id = j.company_id
JOIN contacts ct ON ct.user_id = jc.user_id AND ct.id = jc.contact_id
WHERE jc.user_id = sqlc.arg(user_id)
  AND (sqlc.narg(job_id)::uuid IS NULL OR jc.job_id = sqlc.narg(job_id)::uuid)
  AND (sqlc.narg(contact_id)::uuid IS NULL OR jc.contact_id = sqlc.narg(contact_id)::uuid)
  AND (sqlc.narg(status)::text IS NULL OR jc.status = sqlc.narg(status)::text)
ORDER BY jc.created_at DESC, jc.id DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountJobContacts :one
SELECT count(*)::bigint
FROM job_contacts jc
WHERE jc.user_id = sqlc.arg(user_id)
  AND (sqlc.narg(job_id)::uuid IS NULL OR jc.job_id = sqlc.narg(job_id)::uuid)
  AND (sqlc.narg(contact_id)::uuid IS NULL OR jc.contact_id = sqlc.narg(contact_id)::uuid)
  AND (sqlc.narg(status)::text IS NULL OR jc.status = sqlc.narg(status)::text);

-- name: LockJobContact :one
-- Row lock used by every status transition so concurrent requests serialize.
SELECT status
FROM job_contacts
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id)
FOR UPDATE;

-- name: UpdateJobContactStatus :execrows
UPDATE job_contacts
SET status = sqlc.arg(status),
    invitation_sent_at   = COALESCE(sqlc.narg(invitation_sent_at)::timestamptz, invitation_sent_at),
    accepted_at          = COALESCE(sqlc.narg(accepted_at)::timestamptz, accepted_at),
    message_sent_at      = COALESCE(sqlc.narg(message_sent_at)::timestamptz, message_sent_at),
    followup_due_at      = COALESCE(sqlc.narg(followup_due_at)::timestamptz, followup_due_at),
    referral_received_at = COALESCE(sqlc.narg(referral_received_at)::timestamptz, referral_received_at),
    updated_at = now()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: UpdateJobContactNotes :execrows
UPDATE job_contacts
SET notes = sqlc.arg(notes), updated_at = now()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: ListPendingInvitationTargets :many
-- Candidates for Gmail acceptance matching: contacts with an outstanding invitation.
SELECT jc.id AS job_contact_id, jc.contact_id, ct.full_name, ct.linkedin_url
FROM job_contacts jc
JOIN contacts ct ON ct.user_id = jc.user_id AND ct.id = jc.contact_id
WHERE jc.user_id = sqlc.arg(user_id) AND jc.status = 'INVITATION_PENDING';

-- name: ListPendingInvitationsForContact :many
SELECT jc.id
FROM job_contacts jc
WHERE jc.user_id = sqlc.arg(user_id)
  AND jc.contact_id = sqlc.arg(contact_id)
  AND jc.status = 'INVITATION_PENDING';

-- name: DeleteJobContact :execrows
DELETE FROM job_contacts
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);
