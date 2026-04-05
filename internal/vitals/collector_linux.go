//go:build linux

package vitals

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
)

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

func collectCPU() float32 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		slog.Debug("failed to read /proc/stat", "err", err)
		return 0
	}

	lines := strings.SplitN(string(data), "\n", 2)
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "cpu ") {
		return 0
	}

	fields := strings.Fields(lines[0])
	if len(fields) < 5 {
		return 0
	}

	// fields: cpu user nice system idle [iowait irq softirq ...]
	user, _ := strconv.ParseUint(fields[1], 10, 64)
	nice, _ := strconv.ParseUint(fields[2], 10, 64)
	sys, _ := strconv.ParseUint(fields[3], 10, 64)
	idle, _ := strconv.ParseUint(fields[4], 10, 64)

	prevCPUMu.Lock()
	defer prevCPUMu.Unlock()

	if !prevCPUInit {
		prevCPUVals = [4]uint64{user, nice, sys, idle}
		prevCPUInit = true
		total := user + nice + sys + idle
		if total == 0 {
			return 0
		}
		return float32(user+nice+sys) * 100.0 / float32(total)
	}

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
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		slog.Debug("failed to read /proc/meminfo", "err", err)
		return 0
	}

	var totalKB, availKB uint64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			totalKB = val
		case "MemAvailable:":
			availKB = val
		}
	}

	if totalKB == 0 {
		return 0
	}
	if availKB >= totalKB {
		return 0
	}
	usedKB := totalKB - availKB
	return float32(usedKB) * 100.0 / float32(totalKB)
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
	// Try thermal zone 0 which is commonly the CPU temperature on Linux.
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return 0
	}
	millideg, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}
	return float32(millideg) / 1000.0
}
