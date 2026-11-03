-- ---------- templates ----------

-- name: CreateTemplate :one
INSERT INTO message_templates (user_id, name, body, is_default)
VALUES (sqlc.arg(user_id), sqlc.arg(name), sqlc.arg(body), sqlc.arg(is_default))
RETURNING id;

-- name: GetTemplate :one
SELECT id, name, kind, body, is_default, created_at, updated_at
FROM message_templates
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: GetDefaultTemplate :one
SELECT id, name, kind, body, is_default, created_at, updated_at
FROM message_templates
WHERE user_id = sqlc.arg(user_id) AND kind = 'FOLLOWUP' AND is_default;

-- name: ListTemplates :many
SELECT id, name, kind, body, is_default, created_at, updated_at
FROM message_templates
WHERE user_id = sqlc.arg(user_id)
ORDER BY is_default DESC, lower(name) ASC;

-- name: ClearDefaultTemplates :exec
UPDATE message_templates
SET is_default = false, updated_at = now()
WHERE user_id = sqlc.arg(user_id) AND is_default;

-- name: UpdateTemplate :execrows
UPDATE message_templates
SET name = sqlc.arg(name), body = sqlc.arg(body), is_default = sqlc.arg(is_default), updated_at = now()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: DeleteTemplate :execrows
DELETE FROM message_templates
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- ---------- messages ----------

-- name: CreateMessage :one
INSERT INTO messages (user_id, job_contact_id, template_id, body)
VALUES (sqlc.arg(user_id), sqlc.arg(job_contact_id), sqlc.arg(template_id), sqlc.arg(body))
RETURNING id;

-- name: GetMessage :one
SELECT id, job_contact_id, template_id, kind, body, status, generated_at, sent_at, created_at, updated_at
FROM messages
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: GetDraftMessage :one
SELECT id, job_contact_id, template_id, kind, body, status, generated_at, sent_at, created_at, updated_at
FROM messages
WHERE user_id = sqlc.arg(user_id) AND job_contact_id = sqlc.arg(job_contact_id)
  AND kind = 'FOLLOWUP' AND status = 'DRAFT';

-- name: ListMessagesForJobContact :many
SELECT id, job_contact_id, template_id, kind, body, status, generated_at, sent_at, created_at, updated_at
FROM messages
WHERE user_id = sqlc.arg(user_id) AND job_contact_id = sqlc.arg(job_contact_id)
ORDER BY created_at DESC, id DESC;

-- name: SupersedeDraftMessages :execrows
UPDATE messages
SET status = 'SUPERSEDED', updated_at = now()
WHERE user_id = sqlc.arg(user_id) AND job_contact_id = sqlc.arg(job_contact_id)
  AND kind = 'FOLLOWUP' AND status = 'DRAFT';

-- name: UpdateDraftMessageBody :execrows
UPDATE messages
SET body = sqlc.arg(body), updated_at = now()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND status = 'DRAFT';

-- name: MarkMessageSent :execrows
UPDATE messages
SET status = 'SENT', sent_at = now(), updated_at = now()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND status = 'DRAFT';

-- ---------- generation context and due list ----------

-- name: GetFollowupContext :one
SELECT jc.status, jc.accepted_at,
       ct.full_name AS contact_name, j.title AS job_title, j.job_url, c.name AS company_name
FROM job_contacts jc
JOIN jobs j ON j.user_id = jc.user_id AND j.id = jc.job_id
JOIN companies c ON c.user_id = j.user_id AND c.id = j.company_id
JOIN contacts ct ON ct.user_id = jc.user_id AND ct.id = jc.contact_id
WHERE jc.id = sqlc.arg(id) AND jc.user_id = sqlc.arg(user_id);

-- name: ListFollowupsDue :many
SELECT jc.id, jc.job_id, j.title AS job_title, c.name AS company_name,
       jc.contact_id, ct.full_name AS contact_name, jc.accepted_at, jc.followup_due_at
FROM job_contacts jc
JOIN jobs j ON j.user_id = jc.user_id AND j.id = jc.job_id
JOIN companies c ON c.user_id = j.user_id AND c.id = j.company_id
JOIN contacts ct ON ct.user_id = jc.user_id AND ct.id = jc.contact_id
WHERE jc.user_id = sqlc.arg(user_id)
  AND jc.status = 'CONNECTED'
  AND jc.accepted_at IS NOT NULL
  AND jc.accepted_at <= now() - make_interval(hours => sqlc.arg(delay_hours)::int)
ORDER BY jc.accepted_at ASC, jc.id ASC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountFollowupsDue :one
SELECT count(*)::bigint
FROM job_contacts jc
WHERE jc.user_id = sqlc.arg(user_id)
  AND jc.status = 'CONNECTED'
  AND jc.accepted_at IS NOT NULL
  AND jc.accepted_at <= now() - make_interval(hours => sqlc.arg(delay_hours)::int);
