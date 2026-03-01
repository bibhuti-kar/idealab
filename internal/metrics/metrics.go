package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const namespace = "idealab"

var (
	GPUTemperature = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "gpu_temperature_celsius",
		Help:      "GPU temperature in degrees Celsius.",
	}, []string{"gpu", "uuid"})

	GPUUtilization = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "gpu_utilization_percent",
		Help:      "GPU core utilization percentage.",
	}, []string{"gpu", "uuid"})

	GPUVRAMTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "gpu_vram_total_mb",
		Help:      "Total GPU VRAM in megabytes.",
	}, []string{"gpu", "uuid"})

	GPUVRAMUsed = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "gpu_vram_used_mb",
		Help:      "Used GPU VRAM in megabytes.",
	}, []string{"gpu", "uuid"})

	GPUPowerWatts = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "gpu_power_watts",
		Help:      "GPU power consumption in watts.",
	}, []string{"gpu", "uuid"})

	ReconcileTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "reconcile_total",
		Help:      "Total number of reconciliation cycles.",
	}, []string{"result"})

	ReconcileDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "reconcile_duration_seconds",
		Help:      "Duration of reconciliation cycles in seconds.",
		Buckets:   prometheus.DefBuckets,
	})

	ConfigMapsGenerated = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "configmaps_generated",
		Help:      "Number of ConfigMaps currently managed by the operator.",
	})
)

// RegisterAll registers all custom metrics with the controller-runtime registry.
func RegisterAll() {
	metrics.Registry.MustRegister(
		GPUTemperature,
		GPUUtilization,
		GPUVRAMTotal,
		GPUVRAMUsed,
		GPUPowerWatts,
		ReconcileTotal,
		ReconcileDuration,
		ConfigMapsGenerated,
	)
}
