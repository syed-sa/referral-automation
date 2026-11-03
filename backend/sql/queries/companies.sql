-- name: CreateCompany :one
INSERT INTO companies (user_id, name, website, industry, location, notes)
VALUES (sqlc.arg(user_id), sqlc.arg(name), sqlc.arg(website), sqlc.arg(industry), sqlc.arg(location), sqlc.arg(notes))
RETURNING id;

-- name: GetCompany :one
SELECT c.id, c.name, c.website, c.industry, c.location, c.notes, c.created_at, c.updated_at,
       COALESCE((SELECT count(*) FROM jobs j WHERE j.user_id = c.user_id AND j.company_id = c.id), 0)::bigint AS job_count,
       COALESCE((SELECT count(*) FROM contacts ct WHERE ct.user_id = c.user_id AND ct.company_id = c.id), 0)::bigint AS contact_count
FROM companies c
WHERE c.id = sqlc.arg(id) AND c.user_id = sqlc.arg(user_id);

-- name: ListCompanies :many
SELECT c.id, c.name, c.website, c.industry, c.location, c.notes, c.created_at, c.updated_at,
       COALESCE((SELECT count(*) FROM jobs j WHERE j.user_id = c.user_id AND j.company_id = c.id), 0)::bigint AS job_count,
       COALESCE((SELECT count(*) FROM contacts ct WHERE ct.user_id = c.user_id AND ct.company_id = c.id), 0)::bigint AS contact_count
FROM companies c
WHERE c.user_id = sqlc.arg(user_id)
  AND (sqlc.narg(search)::text IS NULL OR c.name ILIKE '%' || sqlc.narg(search)::text || '%')
ORDER BY c.name ASC, c.id ASC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountCompanies :one
SELECT count(*)::bigint
FROM companies c
WHERE c.user_id = sqlc.arg(user_id)
  AND (sqlc.narg(search)::text IS NULL OR c.name ILIKE '%' || sqlc.narg(search)::text || '%');

-- name: UpdateCompany :execrows
UPDATE companies
SET name = sqlc.arg(name),
    website = sqlc.arg(website),
    industry = sqlc.arg(industry),
    location = sqlc.arg(location),
    notes = sqlc.arg(notes),
    updated_at = now()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: DeleteCompany :execrows
DELETE FROM companies
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);
