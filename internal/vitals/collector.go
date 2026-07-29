package vitals

import (
	"log/slog"
	"strings"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/sensors"
)

// Collect gathers current host metrics from the OS using gopsutil.
func Collect() Vitals {
	memoryPercent, memoryUsed, memoryAvailable, swapUsed := collectMemory()
	diskPercent, diskUsed, diskAvailable, diskRead, diskWrite := collectDisk()
	return Vitals{
		CPUUsagePercent:      collectCPU(),
		MemoryUsagePercent:   memoryPercent,
		MemoryUsedBytes:      memoryUsed,
		MemoryAvailableBytes: memoryAvailable,
		SwapUsedBytes:        swapUsed,
		DiskUsagePercent:     diskPercent,
		DiskUsedBytes:        diskUsed,
		DiskAvailableBytes:   diskAvailable,
		DiskReadBytes:        diskRead,
		DiskWriteBytes:       diskWrite,
		LoadOne:              collectLoad(),
		TemperatureCelsius:   collectTemperature(),
	}
}

func collectCPU() float32 {
	percentages, err := cpu.Percent(0, false)
	if err != nil || len(percentages) == 0 {
		slog.Debug("failed to collect CPU usage", "err", err)
		return 0
	}
	return float32(percentages[0])
}

func collectMemory() (usagePercent float32, usedBytes, availableBytes, swapUsedBytes int64) {
	v, err := mem.VirtualMemory()
	if err != nil {
		slog.Debug("failed to collect memory usage", "err", err)
		return 0, 0, 0, 0
	}
	swap, _ := mem.SwapMemory()
	var swapUsed uint64
	if swap != nil {
		swapUsed = swap.Used
	}
	return float32(v.UsedPercent), int64(v.Used), int64(v.Available), int64(swapUsed)
}

func collectDisk() (usagePercent float32, usedBytes, availableBytes, readBytes, writeBytes int64) {
	usage, err := disk.Usage("/")
	if err != nil {
		slog.Debug("failed to collect disk usage", "err", err)
		return 0, 0, 0, 0, 0
	}
	var readTotal, writeTotal uint64
	if counters, counterErr := disk.IOCounters(); counterErr == nil {
		for _, counter := range counters {
			readTotal += counter.ReadBytes
			writeTotal += counter.WriteBytes
		}
	}
	return float32(usage.UsedPercent), int64(usage.Used), int64(usage.Free), int64(readTotal), int64(writeTotal)
}

func collectLoad() float64 {
	average, err := load.Avg()
	if err != nil {
		return 0
	}
	return average.Load1
}

func collectTemperature() float32 {
	temps, err := sensors.SensorsTemperatures()
	if err != nil || len(temps) == 0 {
		return 0
	}
	// On Apple Silicon, "PMU tdie*" sensors report CPU die temperatures.
	// Use the highest die temperature as the representative value.
	// Fall back to the highest reading from any sensor.
	var maxDie, maxAny float64
	for _, t := range temps {
		if t.Temperature > maxAny {
			maxAny = t.Temperature
		}
		if strings.Contains(t.SensorKey, "tdie") && t.Temperature > maxDie {
			maxDie = t.Temperature
		}
	}
	if maxDie > 0 {
		return float32(maxDie)
	}
	return float32(maxAny)
}
