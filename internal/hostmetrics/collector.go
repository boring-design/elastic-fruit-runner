package hostmetrics

import (
	"context"
	"log/slog"
	"time"

	"golang.org/x/sys/unix"
)

// Vitals holds a point-in-time snapshot of host resource metrics.
type Vitals struct {
	CPUUsagePercent    float32
	MemoryUsagePercent float32
	DiskUsagePercent   float32
	TemperatureCelsius float32
}

// CollectFunc is the signature for a function that collects host metrics.
type CollectFunc func() Vitals

// RunCollector periodically collects host metrics and calls onUpdate.
// Blocks until ctx is cancelled.
func RunCollector(ctx context.Context, interval time.Duration, onUpdate func(Vitals)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Collect immediately on start.
	onUpdate(Collect())

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			onUpdate(Collect())
		}
	}
}

// Collect gathers current host metrics from the OS.
func Collect() Vitals {
	return Vitals{
		CPUUsagePercent:    collectCPU(),
		MemoryUsagePercent: collectMemory(),
		DiskUsagePercent:   collectDisk(),
		TemperatureCelsius: collectTemperature(),
	}
}

func collectCPU() float32 {
	// Use sysctl kern.cp_time to get CPU ticks.
	// On macOS this returns an array of 5 uint32: user, nice, system, idle, ???
	// We approximate utilization as (user+nice+system) / total.
	cpTime, err := unix.SysctlRaw("kern.cp_time")
	if err != nil {
		slog.Debug("failed to read kern.cp_time", "err", err)
		return 0
	}

	if len(cpTime) < 20 {
		return 0
	}

	// Parse as 5 little-endian uint32 values.
	vals := make([]uint64, 5)
	for i := range 5 {
		offset := i * 4
		vals[i] = uint64(cpTime[offset]) |
			uint64(cpTime[offset+1])<<8 |
			uint64(cpTime[offset+2])<<16 |
			uint64(cpTime[offset+3])<<24
	}

	user, nice, sys, idle := vals[0], vals[1], vals[2], vals[3]
	total := user + nice + sys + idle
	if total == 0 {
		return 0
	}
	return float32(user+nice+sys) * 100.0 / float32(total)
}

func collectMemory() float32 {
	// Total physical memory.
	totalBytes, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		slog.Debug("failed to read hw.memsize", "err", err)
		return 0
	}

	// Page size.
	pageSize, err := unix.SysctlUint32("vm.pagesize")
	if err != nil {
		slog.Debug("failed to read vm.pagesize", "err", err)
		return 0
	}

	// vm.page_free_count gives free pages.
	freePages, err := unix.SysctlUint32("vm.page_free_count")
	if err != nil {
		slog.Debug("failed to read vm.page_free_count", "err", err)
		return 0
	}

	freeBytes := uint64(freePages) * uint64(pageSize)
	if totalBytes == 0 {
		return 0
	}
	usedBytes := totalBytes - freeBytes
	return float32(usedBytes) * 100.0 / float32(totalBytes)
}

func collectDisk() float32 {
	var stat unix.Statfs_t
	if err := unix.Statfs("/", &stat); err != nil {
		slog.Debug("failed to statfs /", "err", err)
		return 0
	}
	total := stat.Blocks * uint64(stat.Bsize)
	avail := stat.Bavail * uint64(stat.Bsize)
	if total == 0 {
		return 0
	}
	used := total - avail
	return float32(used) * 100.0 / float32(total)
}

func collectTemperature() float32 {
	// Reading Apple Silicon temperature requires IOKit/SMC access which needs
	// CGO and IOKit framework linking. For now return 0 as a placeholder.
	// TODO: implement via IOKit SMC reading or powermetrics parsing.
	return 0
}
