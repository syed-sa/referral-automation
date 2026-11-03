-- name: CreateJob :one
INSERT INTO jobs (user_id, company_id, title, job_url, location, status, notes)
VALUES (sqlc.arg(user_id), sqlc.arg(company_id), sqlc.arg(title), sqlc.arg(job_url), sqlc.arg(location), sqlc.arg(status), sqlc.arg(notes))
RETURNING id;

-- name: GetJob :one
SELECT j.id, j.company_id, c.name AS company_name, j.title, j.job_url, j.location,
       j.status, j.notes, j.created_at, j.updated_at
FROM jobs j
JOIN companies c ON c.user_id = j.user_id AND c.id = j.company_id
WHERE j.id = sqlc.arg(id) AND j.user_id = sqlc.arg(user_id);

-- name: ListJobs :many
SELECT j.id, j.company_id, c.name AS company_name, j.title, j.job_url, j.location,
       j.status, j.notes, j.created_at, j.updated_at
FROM jobs j
JOIN companies c ON c.user_id = j.user_id AND c.id = j.company_id
WHERE j.user_id = sqlc.arg(user_id)
  AND (sqlc.narg(company_id)::uuid IS NULL OR j.company_id = sqlc.narg(company_id)::uuid)
  AND (sqlc.narg(status)::text IS NULL OR j.status = sqlc.narg(status)::text)
  AND (sqlc.narg(search)::text IS NULL OR j.title ILIKE '%' || sqlc.narg(search)::text || '%')
ORDER BY j.created_at DESC, j.id DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountJobs :one
SELECT count(*)::bigint
FROM jobs j
WHERE j.user_id = sqlc.arg(user_id)
  AND (sqlc.narg(company_id)::uuid IS NULL OR j.company_id = sqlc.narg(company_id)::uuid)
  AND (sqlc.narg(status)::text IS NULL OR j.status = sqlc.narg(status)::text)
  AND (sqlc.narg(search)::text IS NULL OR j.title ILIKE '%' || sqlc.narg(search)::text || '%');

-- name: UpdateJob :execrows
UPDATE jobs
SET company_id = sqlc.arg(company_id),
    title = sqlc.arg(title),
    job_url = sqlc.arg(job_url),
    location = sqlc.arg(location),
    status = sqlc.arg(status),
    notes = sqlc.arg(notes),
    updated_at = now()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: DeleteJob :execrows
DELETE FROM jobs
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);
