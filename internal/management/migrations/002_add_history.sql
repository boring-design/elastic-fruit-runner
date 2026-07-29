-- +goose Up
ALTER TABLE jobs ADD COLUMN owner TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN repository TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN workflow_ref TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN display_name TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN workflow_run_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE jobs ADD COLUMN event_name TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN labels_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE jobs ADD COLUMN queued_at DATETIME;
ALTER TABLE jobs ADD COLUMN scale_set_assigned_at DATETIME;
ALTER TABLE jobs ADD COLUMN runner_assigned_at DATETIME;
ALTER TABLE jobs ADD COLUMN backend TEXT NOT NULL DEFAULT '';

CREATE TABLE job_logs (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id TEXT NOT NULL,
    recorded_at DATETIME NOT NULL,
    text TEXT NOT NULL
);

CREATE INDEX idx_job_logs_job_sequence ON job_logs (job_id, sequence);

CREATE TABLE job_resource_samples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id TEXT NOT NULL,
    recorded_at DATETIME NOT NULL,
    source TEXT NOT NULL,
    accuracy TEXT NOT NULL,
    cpu_percent REAL NOT NULL DEFAULT 0,
    memory_used_bytes INTEGER NOT NULL DEFAULT 0,
    memory_available_bytes INTEGER NOT NULL DEFAULT 0,
    disk_used_bytes INTEGER NOT NULL DEFAULT 0,
    disk_available_bytes INTEGER NOT NULL DEFAULT 0,
    disk_read_bytes INTEGER NOT NULL DEFAULT 0,
    disk_write_bytes INTEGER NOT NULL DEFAULT 0,
    network_receive_bytes INTEGER NOT NULL DEFAULT 0,
    network_send_bytes INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_job_samples_job_time ON job_resource_samples (job_id, recorded_at);

CREATE TABLE host_resource_samples (
    recorded_at DATETIME NOT NULL,
    interval_seconds INTEGER NOT NULL,
    cpu_percent REAL NOT NULL DEFAULT 0,
    memory_used_bytes INTEGER NOT NULL DEFAULT 0,
    memory_available_bytes INTEGER NOT NULL DEFAULT 0,
    disk_used_bytes INTEGER NOT NULL DEFAULT 0,
    disk_available_bytes INTEGER NOT NULL DEFAULT 0,
    disk_read_bytes INTEGER NOT NULL DEFAULT 0,
    disk_write_bytes INTEGER NOT NULL DEFAULT 0,
    load_one REAL NOT NULL DEFAULT 0,
    temperature_celsius REAL NOT NULL DEFAULT 0,
    PRIMARY KEY (recorded_at, interval_seconds)
);

CREATE INDEX idx_host_samples_time ON host_resource_samples (recorded_at);

-- +goose Down
DROP TABLE host_resource_samples;
DROP TABLE job_resource_samples;
DROP TABLE job_logs;
