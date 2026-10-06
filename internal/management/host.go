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

// HostStore records host resource samples and serves their history.
// It is shared by the standalone management service and the cloud agent.
type HostStore struct {
	db              *sql.DB
	queries         *sqlcdb.Queries
	databasePath    string
	hostSampleCount int
}

// NewHostStore creates a host sample store on an already opened database.
func NewHostStore(db *sql.DB, databasePath string) *HostStore {
	return &HostStore{
		db:           db,
		queries:      sqlcdb.New(db),
		databasePath: databasePath,
	}
}

func (store *HostStore) RecordHostVitals(value vitals.Vitals) {
	now := time.Now().UTC()
	err := store.queries.InsertRawHostSample(context.Background(), sqlcdb.InsertRawHostSampleParams{
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
	store.hostSampleCount++
	if store.hostSampleCount%12 == 0 {
		store.rollupHostMinute(now)
		store.cleanHistory(now)
	}
}

func (store *HostStore) rollupHostMinute(now time.Time) {
	minute := now.Truncate(time.Minute).Add(-time.Minute)
	err := store.queries.RollupHostMinute(context.Background(), sqlcdb.RollupHostMinuteParams{
		MinuteStart: minute,
		MinuteEnd:   minute.Add(time.Minute),
	})
	if err != nil {
		slog.Warn("failed to roll up host resource data", "minute", minute, "err", err)
	}
}

func (store *HostStore) cleanHistory(now time.Time) {
	ctx := context.Background()
	historyCutoff := now.Add(-historyRetention)
	_ = store.queries.DeleteExpiredHostSamples(ctx, sqlcdb.DeleteExpiredHostSamplesParams{
		RawCutoff:     now.Add(-rawHostRetention),
		HistoryCutoff: historyCutoff,
	})
	_ = store.queries.DeleteExpiredJobLogs(ctx, historyCutoff)
	_ = store.queries.DeleteExpiredJobResourceSamples(ctx, historyCutoff)
	_ = store.queries.DeleteExpiredJobs(ctx, sql.NullTime{Time: historyCutoff, Valid: true})

	currentSize := storage.FileSize(store.databasePath)
	if currentSize <= maxHistoryBytes {
		return
	}
	for currentSize > targetHistoryBytes {
		bytesToFree := currentSize - targetHistoryBytes
		var freed int64
		for freed < bytesToFree {
			oldest, err := store.queries.GetOldestCompletedJobSize(ctx)
			if err != nil {
				break
			}
			_ = store.queries.DeleteJobLogs(ctx, oldest.ID)
			_ = store.queries.DeleteJobResourceSamples(ctx, oldest.ID)
			_ = store.queries.DeleteCompletedJob(ctx, oldest.ID)
			freed += oldest.EstimatedBytes
		}
		if freed == 0 {
			break
		}
		if err := storage.Compact(ctx, store.db); err != nil {
			slog.Warn("failed to compact history database", "path", store.databasePath, "err", err)
			break
		}
		currentSize = storage.FileSize(store.databasePath)
	}
}

func (store *HostStore) HostSamples(from, to time.Time) ([]HostSample, *time.Time) {
	interval := int64(60)
	if from.After(time.Now().Add(-rawHostRetention)) {
		interval = 5
	}
	ctx := context.Background()
	rows, err := store.queries.ListHostSamples(ctx, sqlcdb.ListHostSamplesParams{
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
	earliest, err := store.queries.GetEarliestHostSampleTime(ctx)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Warn("failed to read earliest host sample time", "err", err)
		}
		return samples, nil
	}
	return samples, &earliest
}
