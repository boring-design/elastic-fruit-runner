-- name: InsertJob :exec
INSERT INTO jobs (
    id,
    runner_name,
    runner_set_name,
    result,
    started_at,
    repository,
    workflow_name,
    workflow_run_id
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateJobCompleted :execresult
UPDATE jobs
SET result = ?, completed_at = ?
WHERE id = ?;

-- name: InsertCompletedJob :exec
INSERT OR IGNORE INTO jobs (id, runner_name, runner_set_name, result, started_at, completed_at)
VALUES (?, '', '', ?, ?, ?);

-- name: ListRecentJobs :many
SELECT id,
       runner_name,
       runner_set_name,
       result,
       started_at,
       completed_at,
       repository,
       workflow_name,
       workflow_run_id
FROM jobs
ORDER BY started_at DESC
LIMIT ?;
