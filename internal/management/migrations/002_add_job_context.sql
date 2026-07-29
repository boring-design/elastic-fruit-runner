-- +goose Up
ALTER TABLE jobs ADD COLUMN repository      TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN workflow_name   TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN workflow_run_id TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE jobs DROP COLUMN workflow_run_id;
ALTER TABLE jobs DROP COLUMN workflow_name;
ALTER TABLE jobs DROP COLUMN repository;
