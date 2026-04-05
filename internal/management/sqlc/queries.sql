-- name: InsertJob :exec
INSERT INTO jobs (id, runner_name, runner_set_name, result, started_at)
VALUES (?, ?, ?, ?, ?);

-- name: UpdateJobCompleted :execresult
UPDATE jobs
SET result = ?, completed_at = ?
WHERE id = ?;

-- name: InsertCompletedJob :exec
INSERT OR IGNORE INTO jobs (id, runner_name, runner_set_name, result, started_at, completed_at)
VALUES (?, '', '', ?, ?, ?);

-- name: ListRecentJobs :many
SELECT id, runner_name, runner_set_name, result, started_at, completed_at
FROM jobs
ORDER BY started_at DESC
LIMIT ?;
