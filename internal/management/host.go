package management

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/boring-design/elastic-fruit-runner/internal/storage"
	sqlcdb "github.com/boring-design/elastic-fruit-runner/internal/storage/sqlc"
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
	err := svc.queries.InsertRawHostSample(context.Background(), sqlcdb.InsertRawHostSampleParams{
		RecordedAt:           now,
		CpuPercent:           float64(value.CPUUsagePercent),
		MemoryUsedBytes:      value.MemoryUsedBytes,
		MemoryAvailableBytes: value.MemoryAvailableBytes,
		DiskUsedBytes:        value.DiskUsedBytes,
		DiskAvailableBytes:   value.DiskAvailableBytes,
		DiskReadBytes:        value.DiskReadBytes,
		DiskWriteBytes:       value.DiskWriteBytes,
		LoadOne:              value.LoadOne,
		TemperatureCelsius:   float64(value.TemperatureCelsius),
	})
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
	err := svc.queries.RollupHostMinute(context.Background(), sqlcdb.RollupHostMinuteParams{
		MinuteStart: minute,
		MinuteEnd:   minute.Add(time.Minute),
	})
	if err != nil {
		slog.Warn("failed to roll up host resource data", "minute", minute, "err", err)
	}
}

func (svc *Service) cleanHistory(now time.Time) {
	ctx := context.Background()
	historyCutoff := now.Add(-historyRetention)
	_ = svc.queries.DeleteExpiredHostSamples(ctx, sqlcdb.DeleteExpiredHostSamplesParams{
		RawCutoff:     now.Add(-rawHostRetention),
		HistoryCutoff: historyCutoff,
	})
	_ = svc.queries.DeleteExpiredJobLogs(ctx, historyCutoff)
	_ = svc.queries.DeleteExpiredJobResourceSamples(ctx, historyCutoff)
	_ = svc.queries.DeleteExpiredJobs(ctx, sql.NullTime{Time: historyCutoff, Valid: true})

	currentSize := storage.FileSize(svc.databasePath)
	if currentSize <= maxHistoryBytes {
		return
	}
	for currentSize > targetHistoryBytes {
		bytesToFree := currentSize - targetHistoryBytes
		var freed int64
		for freed < bytesToFree {
			oldest, err := svc.queries.GetOldestCompletedJobSize(ctx)
			if err != nil {
				break
			}
			_ = svc.queries.DeleteJobLogs(ctx, oldest.ID)
			_ = svc.queries.DeleteJobResourceSamples(ctx, oldest.ID)
			_ = svc.queries.DeleteCompletedJob(ctx, oldest.ID)
			freed += oldest.EstimatedBytes
		}
		if freed == 0 {
			break
		}
		if err := storage.Compact(ctx, svc.db); err != nil {
			slog.Warn("failed to compact history database", "path", svc.databasePath, "err", err)
			break
		}
		currentSize = storage.FileSize(svc.databasePath)
	}
}

func (svc *Service) HostSamples(from, to time.Time) ([]HostSample, *time.Time) {
	interval := int64(60)
	if from.After(time.Now().Add(-rawHostRetention)) {
		interval = 5
	}
	ctx := context.Background()
	rows, err := svc.queries.ListHostSamples(ctx, sqlcdb.ListHostSamplesParams{
		IntervalSeconds: interval,
		RecordedAt:      from,
		RecordedAt_2:    to,
	})
	if err != nil {
		return nil, nil
	}
	var samples []HostSample
	for _, row := range rows {
		samples = append(samples, HostSample{
			RecordedAt:           row.RecordedAt,
			IntervalSeconds:      int(row.IntervalSeconds),
			CPUPercent:           row.CpuPercent,
			MemoryUsedBytes:      row.MemoryUsedBytes,
			MemoryAvailableBytes: row.MemoryAvailableBytes,
			DiskUsedBytes:        row.DiskUsedBytes,
			DiskAvailableBytes:   row.DiskAvailableBytes,
			DiskReadBytes:        row.DiskReadBytes,
			DiskWriteBytes:       row.DiskWriteBytes,
			LoadOne:              row.LoadOne,
			TemperatureCelsius:   row.TemperatureCelsius,
		})
	}
	earliest, err := svc.queries.GetEarliestHostSampleTime(ctx)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Warn("failed to read earliest host sample time", "err", err)
		}
		return samples, nil
	}
	return samples, &earliest
}
