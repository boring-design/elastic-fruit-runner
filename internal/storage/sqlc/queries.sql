-- name: UpsertStartedJob :exec
INSERT INTO jobs (
    id, runner_name, runner_set_name, result, started_at, owner, repository,
    workflow_ref, display_name, workflow_run_id, event_name, labels_json,
    queued_at, scale_set_assigned_at, runner_assigned_at, backend
) VALUES (?, ?, ?, 'running', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    runner_name = excluded.runner_name,
    runner_set_name = excluded.runner_set_name,
    result = 'running',
    owner = excluded.owner,
    repository = excluded.repository,
    workflow_ref = excluded.workflow_ref,
    display_name = excluded.display_name,
    workflow_run_id = excluded.workflow_run_id,
    event_name = excluded.event_name,
    labels_json = excluded.labels_json,
    queued_at = excluded.queued_at,
    scale_set_assigned_at = excluded.scale_set_assigned_at,
    runner_assigned_at = excluded.runner_assigned_at,
    backend = excluded.backend;

-- name: UpdateJobCompleted :execresult
UPDATE jobs
SET result = ?, completed_at = ?
WHERE id = ?;

-- name: InsertCompletedJob :exec
INSERT OR IGNORE INTO jobs (id, runner_name, runner_set_name, result, started_at, completed_at)
VALUES (?, '', '', ?, ?, ?);

-- name: InsertJobLog :exec
INSERT INTO job_logs (job_id, recorded_at, text)
VALUES (?, ?, ?);

-- name: InsertJobResourceSample :exec
INSERT INTO job_resource_samples (
    job_id, recorded_at, source, accuracy, cpu_percent, memory_used_bytes,
    memory_available_bytes, disk_used_bytes, disk_available_bytes,
    disk_read_bytes, disk_write_bytes, network_receive_bytes, network_send_bytes
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListJobsNewestFirst :many
SELECT * FROM jobs
ORDER BY started_at DESC
LIMIT 2000;

-- name: GetJob :one
SELECT * FROM jobs
WHERE id = ?;

-- name: ListJobLogsAfterSequence :many
SELECT sequence, recorded_at, text
FROM job_logs
WHERE job_id = ? AND sequence > ?
ORDER BY sequence
LIMIT ?;

-- name: ListJobResourceSamples :many
SELECT recorded_at, source, accuracy, cpu_percent, memory_used_bytes,
    memory_available_bytes, disk_used_bytes, disk_available_bytes,
    disk_read_bytes, disk_write_bytes, network_receive_bytes, network_send_bytes
FROM job_resource_samples
WHERE job_id = ?
ORDER BY recorded_at;

-- name: InsertRawHostSample :exec
INSERT INTO host_resource_samples (
    recorded_at, interval_seconds, cpu_percent, memory_used_bytes,
    memory_available_bytes, disk_used_bytes, disk_available_bytes,
    disk_read_bytes, disk_write_bytes, load_one, temperature_celsius
) VALUES (?, 5, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: RollupHostMinute :exec
INSERT OR REPLACE INTO host_resource_samples (
    recorded_at, interval_seconds, cpu_percent, memory_used_bytes,
    memory_available_bytes, disk_used_bytes, disk_available_bytes,
    disk_read_bytes, disk_write_bytes, load_one, temperature_celsius
)
SELECT sqlc.arg(minute_start), 60, AVG(raw.cpu_percent), AVG(raw.memory_used_bytes),
    AVG(raw.memory_available_bytes), AVG(raw.disk_used_bytes), AVG(raw.disk_available_bytes),
    MAX(raw.disk_read_bytes), MAX(raw.disk_write_bytes), AVG(raw.load_one),
    AVG(raw.temperature_celsius)
FROM host_resource_samples AS raw
WHERE raw.interval_seconds = 5 AND raw.recorded_at >= sqlc.arg(minute_start) AND raw.recorded_at < sqlc.arg(minute_end);

-- name: DeleteExpiredHostSamples :exec
DELETE FROM host_resource_samples
WHERE (interval_seconds = 5 AND recorded_at < sqlc.arg(raw_cutoff))
    OR recorded_at < sqlc.arg(history_cutoff);

-- name: DeleteExpiredJobLogs :exec
DELETE FROM job_logs
WHERE recorded_at < sqlc.arg(cutoff) OR job_id IN (
    SELECT id FROM jobs
    WHERE result != 'running' AND COALESCE(completed_at, started_at) < sqlc.arg(cutoff)
);

-- name: DeleteExpiredJobResourceSamples :exec
DELETE FROM job_resource_samples
WHERE recorded_at < sqlc.arg(cutoff) OR job_id IN (
    SELECT id FROM jobs
    WHERE result != 'running' AND COALESCE(completed_at, started_at) < sqlc.arg(cutoff)
);

-- name: DeleteExpiredJobs :exec
DELETE FROM jobs
WHERE result != 'running' AND COALESCE(completed_at, started_at) < ?;

-- name: GetOldestCompletedJobSize :one
SELECT jobs.id,
    CAST(4096
        + COALESCE((SELECT SUM(LENGTH(text)) FROM job_logs WHERE job_id = jobs.id), 0)
        + COALESCE((SELECT COUNT(*) * 256 FROM job_resource_samples WHERE job_id = jobs.id), 0)
    AS INTEGER) AS estimated_bytes
FROM jobs
WHERE result != 'running'
ORDER BY COALESCE(completed_at, started_at)
LIMIT 1;

-- name: DeleteJobLogs :exec
DELETE FROM job_logs WHERE job_id = ?;

-- name: DeleteJobResourceSamples :exec
DELETE FROM job_resource_samples WHERE job_id = ?;

-- name: DeleteCompletedJob :exec
DELETE FROM jobs WHERE id = ? AND result != 'running';

-- name: ListHostSamples :many
SELECT recorded_at, interval_seconds, cpu_percent, memory_used_bytes,
    memory_available_bytes, disk_used_bytes, disk_available_bytes,
    disk_read_bytes, disk_write_bytes, load_one, temperature_celsius
FROM host_resource_samples
WHERE interval_seconds = ? AND recorded_at >= ? AND recorded_at <= ?
ORDER BY recorded_at;

-- name: GetEarliestHostSampleTime :one
SELECT recorded_at FROM host_resource_samples
ORDER BY recorded_at
LIMIT 1;

-- name: CountAdmin :one
SELECT COUNT(*) FROM console_admin WHERE id = 1;

-- name: UpsertAdminPassword :exec
INSERT INTO console_admin (id, password_hash, created_at)
VALUES (1, ?, ?)
ON CONFLICT(id) DO UPDATE SET password_hash = excluded.password_hash, created_at = excluded.created_at;

-- name: GetAdminPasswordHash :one
SELECT password_hash FROM console_admin WHERE id = 1;

-- name: GetSession :one
SELECT csrf_token, expires_at
FROM console_sessions
WHERE token_hash = ?;

-- name: DeleteSession :exec
DELETE FROM console_sessions WHERE token_hash = ?;

-- name: DeleteAllSessions :exec
DELETE FROM console_sessions;

-- name: DeleteAdmin :exec
DELETE FROM console_admin;

-- name: DeleteExpiredSessions :exec
DELETE FROM console_sessions WHERE expires_at <= ?;

-- name: InsertSession :exec
INSERT INTO console_sessions (token_hash, csrf_token, expires_at, created_at)
VALUES (?, ?, ?, ?);

-- name: GetLastActiveConfigYAML :one
SELECT config_yaml FROM config_revisions
WHERE active = 1
ORDER BY created_at DESC
LIMIT 1;

-- name: ListRecentConfigRevisions :many
SELECT id, created_at, source, config_hash
FROM config_revisions
ORDER BY created_at DESC
LIMIT 10;

-- name: GetConfigRevisionYAML :one
SELECT config_yaml FROM config_revisions WHERE id = ?;

-- name: ClearActiveConfigRevision :exec
UPDATE config_revisions SET active = 0;

-- name: InsertConfigRevision :exec
INSERT INTO config_revisions (created_at, source, config_hash, config_yaml, active)
VALUES (?, ?, ?, ?, ?);

-- name: TrimConfigRevisions :exec
DELETE FROM config_revisions WHERE id NOT IN (
    SELECT id FROM config_revisions ORDER BY created_at DESC LIMIT 10
);
