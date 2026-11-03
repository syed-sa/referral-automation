-- name: CreateWorkflowEvent :exec
INSERT INTO workflow_events (user_id, job_contact_id, event_type, from_status, to_status, metadata)
VALUES (sqlc.arg(user_id), sqlc.arg(job_contact_id), sqlc.arg(event_type), sqlc.narg(from_status), sqlc.narg(to_status), sqlc.arg(metadata));

-- name: ListJobContactEvents :many
SELECT e.id, e.event_type, e.from_status, e.to_status, e.metadata, e.created_at
FROM workflow_events e
WHERE e.user_id = sqlc.arg(user_id) AND e.job_contact_id = sqlc.arg(job_contact_id)
ORDER BY e.created_at ASC, e.id ASC;
