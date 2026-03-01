package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()

	collectors := []prometheus.Collector{
		GPUTemperature,
		GPUUtilization,
		GPUVRAMTotal,
		GPUVRAMUsed,
		GPUPowerWatts,
		ReconcileTotal,
		ReconcileDuration,
		ConfigMapsGenerated,
	}

	for _, c := range collectors {
		if err := reg.Register(c); err != nil {
			t.Errorf("failed to register metric: %v", err)
		}
	}
}

func TestGPUGaugesSetValues(t *testing.T) {
	labels := prometheus.Labels{"gpu": "GTX-1660-Ti", "uuid": "GPU-12345"}

	GPUTemperature.With(labels).Set(45)
	GPUUtilization.With(labels).Set(80)
	GPUVRAMTotal.With(labels).Set(6144)
	GPUVRAMUsed.With(labels).Set(2048)
	GPUPowerWatts.With(labels).Set(80)

	// If no panic, the gauges work correctly with labels.
}

func TestCounterIncrement(t *testing.T) {
	ReconcileTotal.With(prometheus.Labels{"result": "success"}).Inc()
	ReconcileTotal.With(prometheus.Labels{"result": "error"}).Inc()
}

func TestConfigMapsGauge(t *testing.T) {
	ConfigMapsGenerated.Set(3)
}
