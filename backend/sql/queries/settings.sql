-- name: GetFollowupDelayHours :one
SELECT COALESCE((SELECT followup_delay_hours FROM application_settings WHERE user_id = sqlc.arg(user_id)), 24)::int AS delay_hours;

-- name: GetSettings :one
SELECT followup_delay_hours, sender_name, updated_at
FROM application_settings
WHERE user_id = sqlc.arg(user_id);

-- name: UpsertSettings :exec
INSERT INTO application_settings (user_id, followup_delay_hours, sender_name)
VALUES (sqlc.arg(user_id), sqlc.arg(followup_delay_hours), sqlc.arg(sender_name))
ON CONFLICT (user_id) DO UPDATE
SET followup_delay_hours = EXCLUDED.followup_delay_hours,
    sender_name = EXCLUDED.sender_name,
    updated_at = now();
