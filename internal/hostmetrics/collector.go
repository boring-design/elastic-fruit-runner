//go:build darwin

package hostmetrics

import (
	"log/slog"
	"sync"

	"golang.org/x/sys/unix"
)

// prevCPU stores the previous kern.cp_time sample so we can compute
// delta-based CPU utilization instead of boot-time averages.
var (
	prevCPUMu   sync.Mutex
	prevCPUVals [4]uint64
	prevCPUInit bool
)

// Collect gathers current host metrics from the OS.
func Collect() Vitals {
	return Vitals{
		CPUUsagePercent:    collectCPU(),
		MemoryUsagePercent: collectMemory(),
		DiskUsagePercent:   collectDisk(),
		TemperatureCelsius: collectTemperature(),
	}
}

// readCPUTicks reads kern.cp_time and returns (user, nice, sys, idle).
// On 64-bit Darwin, kern.cp_time exposes 5 counters as C `long` values
// (8 bytes each), so the raw payload is 40 bytes.
func readCPUTicks() (user, nice, sys, idle uint64, ok bool) {
	cpTime, err := unix.SysctlRaw("kern.cp_time")
	if err != nil {
		slog.Debug("failed to read kern.cp_time", "err", err)
		return 0, 0, 0, 0, false
	}

	// 5 counters x 8 bytes (uint64 / long on 64-bit Darwin) = 40 bytes.
	if len(cpTime) < 40 {
		return 0, 0, 0, 0, false
	}

	// Parse as 5 little-endian uint64 values.
	vals := make([]uint64, 5)
	for i := range 5 {
		offset := i * 8
		vals[i] = uint64(cpTime[offset]) |
			uint64(cpTime[offset+1])<<8 |
			uint64(cpTime[offset+2])<<16 |
			uint64(cpTime[offset+3])<<24 |
			uint64(cpTime[offset+4])<<32 |
			uint64(cpTime[offset+5])<<40 |
			uint64(cpTime[offset+6])<<48 |
			uint64(cpTime[offset+7])<<56
	}

	return vals[0], vals[1], vals[2], vals[3], true
}

func collectCPU() float32 {
	user, nice, sys, idle, ok := readCPUTicks()
	if !ok {
		return 0
	}

	prevCPUMu.Lock()
	defer prevCPUMu.Unlock()

	if !prevCPUInit {
		// First sample: store counters and fall back to cumulative average.
		prevCPUVals = [4]uint64{user, nice, sys, idle}
		prevCPUInit = true
		total := user + nice + sys + idle
		if total == 0 {
			return 0
		}
		return float32(user+nice+sys) * 100.0 / float32(total)
	}

	// Compute delta from previous sample.
	dUser := user - prevCPUVals[0]
	dNice := nice - prevCPUVals[1]
	dSys := sys - prevCPUVals[2]
	dIdle := idle - prevCPUVals[3]

	prevCPUVals = [4]uint64{user, nice, sys, idle}

	dActive := dUser + dNice + dSys
	dTotal := dActive + dIdle
	if dTotal == 0 {
		return 0
	}
	return float32(dActive) * 100.0 / float32(dTotal)
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
	// Guard against unsigned underflow: the two sysctl calls are not atomic,
	// so freeBytes can transiently exceed totalBytes.
	if freeBytes >= totalBytes {
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
