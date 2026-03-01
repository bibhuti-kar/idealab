package controller

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/bibhuti-kar/idealab/internal/discovery"
	"github.com/bibhuti-kar/idealab/internal/metrics"
)

// recordGPUMetrics updates Prometheus gauges from discovered GPU info.
func recordGPUMetrics(info discovery.DeviceInfo) {
	for _, gpu := range info.GPUs {
		labels := prometheus.Labels{
			"gpu":  gpu.Model,
			"uuid": gpu.UUID,
		}
		metrics.GPUTemperature.With(labels).Set(float64(gpu.Temperature))
		metrics.GPUUtilization.With(labels).Set(float64(gpu.UtilizationPct))
		metrics.GPUVRAMTotal.With(labels).Set(float64(gpu.VRAMMB))
		metrics.GPUVRAMUsed.With(labels).Set(float64(gpu.VRAMUsedMB))
		metrics.GPUPowerWatts.With(labels).Set(float64(gpu.PowerWatts))
	}
}

// recordConfigMapCount sets the configmaps_generated gauge.
func recordConfigMapCount(count int) {
	metrics.ConfigMapsGenerated.Set(float64(count))
}
