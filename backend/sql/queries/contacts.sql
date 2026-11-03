-- name: CreateContact :one
INSERT INTO contacts (user_id, company_id, full_name, linkedin_url, title, email, notes)
VALUES (sqlc.arg(user_id), sqlc.arg(company_id), sqlc.arg(full_name), sqlc.arg(linkedin_url), sqlc.arg(title), sqlc.arg(email), sqlc.arg(notes))
RETURNING id;

-- name: GetContact :one
SELECT ct.id, ct.company_id, c.name AS company_name, ct.full_name, ct.linkedin_url,
       ct.title, ct.email, ct.notes, ct.created_at, ct.updated_at
FROM contacts ct
LEFT JOIN companies c ON c.user_id = ct.user_id AND c.id = ct.company_id
WHERE ct.id = sqlc.arg(id) AND ct.user_id = sqlc.arg(user_id);

-- name: ListContacts :many
SELECT ct.id, ct.company_id, c.name AS company_name, ct.full_name, ct.linkedin_url,
       ct.title, ct.email, ct.notes, ct.created_at, ct.updated_at
FROM contacts ct
LEFT JOIN companies c ON c.user_id = ct.user_id AND c.id = ct.company_id
WHERE ct.user_id = sqlc.arg(user_id)
  AND (sqlc.narg(company_id)::uuid IS NULL OR ct.company_id = sqlc.narg(company_id)::uuid)
  AND (sqlc.narg(search)::text IS NULL
       OR ct.full_name ILIKE '%' || sqlc.narg(search)::text || '%'
       OR ct.title ILIKE '%' || sqlc.narg(search)::text || '%')
ORDER BY ct.full_name ASC, ct.id ASC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountContacts :one
SELECT count(*)::bigint
FROM contacts ct
WHERE ct.user_id = sqlc.arg(user_id)
  AND (sqlc.narg(company_id)::uuid IS NULL OR ct.company_id = sqlc.narg(company_id)::uuid)
  AND (sqlc.narg(search)::text IS NULL
       OR ct.full_name ILIKE '%' || sqlc.narg(search)::text || '%'
       OR ct.title ILIKE '%' || sqlc.narg(search)::text || '%');

-- name: UpdateContact :execrows
UPDATE contacts
SET company_id = sqlc.arg(company_id),
    full_name = sqlc.arg(full_name),
    linkedin_url = sqlc.arg(linkedin_url),
    title = sqlc.arg(title),
    email = sqlc.arg(email),
    notes = sqlc.arg(notes),
    updated_at = now()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: DeleteContact :execrows
DELETE FROM contacts
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);
