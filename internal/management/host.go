package management

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/boring-design/elastic-fruit-runner/internal/vitals"
)

const (
	rawHostRetention   = 24 * time.Hour
	historyRetention   = 30 * 24 * time.Hour
	maxHistoryBytes    = int64(10 * 1024 * 1024 * 1024)
	targetHistoryBytes = int64(9 * 1024 * 1024 * 1024)
)

type HostSample struct {
	RecordedAt           time.Time
	IntervalSeconds      int
	CPUPercent           float64
	MemoryUsedBytes      int64
	MemoryAvailableBytes int64
	DiskUsedBytes        int64
	DiskAvailableBytes   int64
	DiskReadBytes        int64
	DiskWriteBytes       int64
	LoadOne              float64
	TemperatureCelsius   float64
}

func (svc *Service) RecordHostVitals(value vitals.Vitals) {
	now := time.Now().UTC()
	_, err := svc.db.ExecContext(context.Background(), `
		INSERT INTO host_resource_samples (
			recorded_at, interval_seconds, cpu_percent, memory_used_bytes,
			memory_available_bytes, disk_used_bytes, disk_available_bytes,
			disk_read_bytes, disk_write_bytes, load_one, temperature_celsius
		) VALUES (?, 5, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		now, value.CPUUsagePercent, value.MemoryUsedBytes, value.MemoryAvailableBytes,
		value.DiskUsedBytes, value.DiskAvailableBytes, value.DiskReadBytes,
		value.DiskWriteBytes, value.LoadOne, value.TemperatureCelsius,
	)
	if err != nil {
		slog.Warn("failed to record host resource data", "err", err)
		return
	}
	svc.hostSampleCount++
	if svc.hostSampleCount%12 == 0 {
		svc.rollupHostMinute(now)
		svc.cleanHistory(now)
	}
}

func (svc *Service) rollupHostMinute(now time.Time) {
	minute := now.Truncate(time.Minute).Add(-time.Minute)
	next := minute.Add(time.Minute)
	_, err := svc.db.ExecContext(context.Background(), `
		INSERT OR REPLACE INTO host_resource_samples (
			recorded_at, interval_seconds, cpu_percent, memory_used_bytes,
			memory_available_bytes, disk_used_bytes, disk_available_bytes,
			disk_read_bytes, disk_write_bytes, load_one, temperature_celsius
		)
		SELECT ?, 60, AVG(cpu_percent), AVG(memory_used_bytes),
			AVG(memory_available_bytes), AVG(disk_used_bytes), AVG(disk_available_bytes),
			MAX(disk_read_bytes), MAX(disk_write_bytes), AVG(load_one),
			AVG(temperature_celsius)
		FROM host_resource_samples
		WHERE interval_seconds = 5 AND recorded_at >= ? AND recorded_at < ?`,
		minute, minute, next,
	)
	if err != nil {
		slog.Warn("failed to roll up host resource data", "minute", minute, "err", err)
	}
}

func (svc *Service) cleanHistory(now time.Time) {
	ctx := context.Background()
	_, _ = svc.db.ExecContext(ctx, `
		DELETE FROM host_resource_samples
		WHERE (interval_seconds = 5 AND recorded_at < ?)
			OR recorded_at < ?`,
		now.Add(-rawHostRetention), now.Add(-historyRetention),
	)
	_, _ = svc.db.ExecContext(ctx, `DELETE FROM job_logs WHERE recorded_at < ?`, now.Add(-historyRetention))
	_, _ = svc.db.ExecContext(ctx, `DELETE FROM job_resource_samples WHERE recorded_at < ?`, now.Add(-historyRetention))

	if databaseSize(svc.databasePath) <= maxHistoryBytes {
		return
	}
	for databaseSize(svc.databasePath) > targetHistoryBytes {
		var jobID string
		err := svc.db.QueryRowContext(ctx, `
			SELECT id FROM jobs
			WHERE result != 'running'
			ORDER BY COALESCE(completed_at, started_at)
			LIMIT 1`).Scan(&jobID)
		if err != nil {
			break
		}
		_, _ = svc.db.ExecContext(ctx, `DELETE FROM job_logs WHERE job_id = ?`, jobID)
		_, _ = svc.db.ExecContext(ctx, `DELETE FROM job_resource_samples WHERE job_id = ?`, jobID)
		_, _ = svc.db.ExecContext(ctx, `DELETE FROM jobs WHERE id = ? AND result != 'running'`, jobID)
	}
	_, _ = svc.db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`)
}

func (svc *Service) HostSamples(from, to time.Time) ([]HostSample, *time.Time) {
	interval := 60
	if from.After(time.Now().Add(-rawHostRetention)) {
		interval = 5
	}
	rows, err := svc.db.QueryContext(context.Background(), `
		SELECT recorded_at, interval_seconds, cpu_percent, memory_used_bytes,
			memory_available_bytes, disk_used_bytes, disk_available_bytes,
			disk_read_bytes, disk_write_bytes, load_one, temperature_celsius
		FROM host_resource_samples
		WHERE interval_seconds = ? AND recorded_at >= ? AND recorded_at <= ?
		ORDER BY recorded_at`, interval, from, to)
	if err != nil {
		return nil, nil
	}
	defer rows.Close()
	var samples []HostSample
	for rows.Next() {
		var sample HostSample
		if err := rows.Scan(
			&sample.RecordedAt, &sample.IntervalSeconds, &sample.CPUPercent,
			&sample.MemoryUsedBytes, &sample.MemoryAvailableBytes,
			&sample.DiskUsedBytes, &sample.DiskAvailableBytes,
			&sample.DiskReadBytes, &sample.DiskWriteBytes, &sample.LoadOne,
			&sample.TemperatureCelsius,
		); err == nil {
			samples = append(samples, sample)
		}
	}
	var earliest sql.NullTime
	_ = svc.db.QueryRowContext(context.Background(), `SELECT MIN(recorded_at) FROM host_resource_samples`).Scan(&earliest)
	if earliest.Valid {
		value := earliest.Time
		return samples, &value
	}
	return samples, nil
}

func databaseSize(path string) int64 {
	if path == "" || path == ":memory:" {
		return 0
	}
	var size int64
	for _, suffix := range []string{"", "-wal", "-shm"} {
		info, err := os.Stat(filepath.Clean(path + suffix))
		if err == nil {
			size += info.Size()
		}
	}
	return size
}
