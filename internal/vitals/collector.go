package vitals

import (
	"log/slog"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/sensors"
)

// Collect gathers current host metrics from the OS using gopsutil.
func Collect() Vitals {
	return Vitals{
		CPUUsagePercent:    collectCPU(),
		MemoryUsagePercent: collectMemory(),
		DiskUsagePercent:   collectDisk(),
		TemperatureCelsius: collectTemperature(),
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

func collectMemory() float32 {
	v, err := mem.VirtualMemory()
	if err != nil {
		slog.Debug("failed to collect memory usage", "err", err)
		return 0
	}
	return float32(v.UsedPercent)
}

func collectDisk() float32 {
	usage, err := disk.Usage("/")
	if err != nil {
		slog.Debug("failed to collect disk usage", "err", err)
		return 0
	}
	return float32(usage.UsedPercent)
}

func collectTemperature() float32 {
	temps, err := sensors.SensorsTemperatures()
	if err != nil || len(temps) == 0 {
		return 0
	}
	// Return the first non-zero reading (typically CPU/SoC temperature).
	for _, t := range temps {
		if t.Temperature > 0 {
			return float32(t.Temperature)
		}
	}
	return 0
}
