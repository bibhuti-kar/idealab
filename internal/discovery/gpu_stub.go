//go:build !cgo || !linux

package discovery

import "fmt"

func (r *realGPUDiscoverer) discoverGPUs() ([]GPUInfo, error) {
	return nil, fmt.Errorf("NVML not available: build with CGO_ENABLED=1 on Linux with NVIDIA drivers, or use MOCK_DISCOVERY=true")
}

func (r *realGPUDiscoverer) close() {}
