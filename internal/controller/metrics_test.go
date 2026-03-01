package controller

import (
	"testing"

	"github.com/bibhuti-kar/idealab/internal/discovery"
)

func TestRecordGPUMetrics(t *testing.T) {
	info := testDeviceInfo()
	// Should not panic with valid GPU data.
	recordGPUMetrics(info)
}

func TestRecordGPUMetrics_NoGPU(t *testing.T) {
	info := discovery.DeviceInfo{
		CPU:    discovery.CPUInfo{Cores: 4},
		Memory: discovery.MemoryInfo{TotalMB: 16384},
	}
	// Should not panic with no GPUs.
	recordGPUMetrics(info)
}

func TestRecordConfigMapCount(t *testing.T) {
	// Should not panic.
	recordConfigMapCount(3)
	recordConfigMapCount(0)
}
