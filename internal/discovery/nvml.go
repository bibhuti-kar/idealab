// Package discovery provides hardware enumeration for CPU, GPU, and memory.
//
// The NVMLDiscoverer uses NVIDIA Management Library bindings to query GPU
// properties. On systems without NVIDIA GPUs or drivers, use MockDiscoverer.
package discovery

import (
	"fmt"
	"log/slog"
	"os"
)

// NVMLDiscoverer enumerates hardware using NVML for GPU and /proc for CPU/memory.
type NVMLDiscoverer struct {
	Logger  *slog.Logger
	gpuImpl gpuDiscoverer
}

// gpuDiscoverer abstracts GPU enumeration for testability.
type gpuDiscoverer interface {
	discoverGPUs() ([]GPUInfo, error)
	close()
}

// NewNVMLDiscoverer creates a discoverer that uses real NVML bindings.
// Pass nil logger to use slog.Default().
func NewNVMLDiscoverer(logger *slog.Logger) *NVMLDiscoverer {
	if logger == nil {
		logger = slog.Default()
	}
	return &NVMLDiscoverer{
		Logger:  logger,
		gpuImpl: &realGPUDiscoverer{logger: logger},
	}
}

// Discover enumerates all hardware: CPU, GPU(s), and memory.
func (d *NVMLDiscoverer) Discover() (DeviceInfo, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
		d.Logger.Warn("failed to get hostname", "error", err)
	}

	cpu, err := discoverCPU(d.Logger)
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("cpu discovery: %w", err)
	}

	mem, err := discoverMemory(d.Logger)
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("memory discovery: %w", err)
	}

	gpus, err := d.gpuImpl.discoverGPUs()
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("gpu discovery: %w", err)
	}

	info := DeviceInfo{
		Hostname: hostname,
		CPU:      cpu,
		GPUs:     gpus,
		Memory:   mem,
	}

	d.Logger.Info("device discovery complete",
		"hostname", info.Hostname,
		"cpu", info.CPU.Model,
		"gpuCount", len(info.GPUs),
		"memoryMB", info.Memory.TotalMB,
	)
	return info, nil
}

// realGPUDiscoverer uses NVML to enumerate GPUs.
// Implementation is in gpu_nvml.go (cgo) or gpu_stub.go (!cgo).
type realGPUDiscoverer struct {
	logger *slog.Logger
}

// mockGPUDiscoverer returns pre-configured GPU info for testing.
type mockGPUDiscoverer struct {
	gpus []GPUInfo
	err  error
}

func (m *mockGPUDiscoverer) discoverGPUs() ([]GPUInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.gpus, nil
}

func (m *mockGPUDiscoverer) close() {}

// NewTestableDiscoverer creates a discoverer with a mock GPU backend.
// Useful for unit tests that need CPU/memory from the real system
// but want to inject GPU data.
func NewTestableDiscoverer(logger *slog.Logger, gpus []GPUInfo, gpuErr error) *NVMLDiscoverer {
	if logger == nil {
		logger = slog.Default()
	}
	return &NVMLDiscoverer{
		Logger:  logger,
		gpuImpl: &mockGPUDiscoverer{gpus: gpus, err: gpuErr},
	}
}
