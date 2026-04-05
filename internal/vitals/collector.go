package vitals

import (
	"log/slog"
	"strings"

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
