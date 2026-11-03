-- name: GetDashboardCounts :one
SELECT
    COALESCE((SELECT count(*) FROM companies WHERE user_id = sqlc.arg(user_id)), 0)::bigint AS companies,
    COALESCE((SELECT count(*) FROM jobs WHERE user_id = sqlc.arg(user_id) AND status = 'ACTIVE'), 0)::bigint AS active_jobs,
    COALESCE((SELECT count(*) FROM contacts WHERE user_id = sqlc.arg(user_id)), 0)::bigint AS contacts,
    -- Follow-up eligibility: connection accepted at least the user's delay ago (default 24h)
    -- and no message prepared yet.
    COALESCE((SELECT count(*) FROM job_contacts
              WHERE user_id = sqlc.arg(user_id)
                AND status = 'CONNECTED'
                AND accepted_at IS NOT NULL
                AND accepted_at <= now() - make_interval(hours =>
                    COALESCE((SELECT followup_delay_hours FROM application_settings
                              WHERE user_id = sqlc.arg(user_id)), 24))), 0)::bigint AS followups_due;

-- name: CountJobContactsByStatus :many
SELECT status, count(*)::bigint AS total
FROM job_contacts
WHERE user_id = sqlc.arg(user_id)
GROUP BY status;

-- name: ListRecentWorkflowEvents :many
SELECT e.id, e.job_contact_id, ct.full_name AS contact_name, e.event_type,
       e.from_status, e.to_status, e.created_at
FROM workflow_events e
JOIN job_contacts jc ON jc.user_id = e.user_id AND jc.id = e.job_contact_id
JOIN contacts ct ON ct.user_id = jc.user_id AND ct.id = jc.contact_id
WHERE e.user_id = sqlc.arg(user_id)
ORDER BY e.created_at DESC, e.id DESC
LIMIT sqlc.arg(row_limit)::int;
