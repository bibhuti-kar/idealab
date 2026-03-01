//go:build cgo && linux

package discovery

import (
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

func (r *realGPUDiscoverer) discoverGPUs() ([]GPUInfo, error) {
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("NVML init failed: %s", nvml.ErrorString(ret))
	}
	defer nvml.Shutdown()

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("NVML DeviceGetCount: %s", nvml.ErrorString(ret))
	}

	driverVersion, ret := nvml.SystemGetDriverVersion()
	if ret != nvml.SUCCESS {
		driverVersion = "unknown"
	}

	cudaVersionInt, ret := nvml.SystemGetCudaDriverVersion_v2()
	cudaVersion := "unknown"
	if ret == nvml.SUCCESS {
		cudaVersion = fmt.Sprintf("%d.%d", cudaVersionInt/1000, (cudaVersionInt%1000)/10)
	}

	gpus := make([]GPUInfo, 0, count)
	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			r.logger.Warn("failed to get GPU handle", "index", i, "error", nvml.ErrorString(ret))
			continue
		}

		gpu, err := r.readGPUInfo(device, driverVersion, cudaVersion)
		if err != nil {
			r.logger.Warn("failed to read GPU info", "index", i, "error", err)
			continue
		}
		gpus = append(gpus, gpu)
	}

	if len(gpus) == 0 {
		return nil, fmt.Errorf("no GPUs enumerated (count=%d)", count)
	}
	return gpus, nil
}

func (r *realGPUDiscoverer) readGPUInfo(device nvml.Device, driverVersion, cudaVersion string) (GPUInfo, error) {
	name, ret := device.GetName()
	if ret != nvml.SUCCESS {
		return GPUInfo{}, fmt.Errorf("GetName: %s", nvml.ErrorString(ret))
	}

	uuid, ret := device.GetUUID()
	if ret != nvml.SUCCESS {
		uuid = "unknown"
	}

	memory, ret := device.GetMemoryInfo()
	vramMB := 0
	if ret == nvml.SUCCESS {
		vramMB = int(memory.Total / (1024 * 1024))
	}

	major, minor, ret := device.GetCudaComputeCapability()
	computeCap := "unknown"
	if ret == nvml.SUCCESS {
		computeCap = fmt.Sprintf("%d.%d", major, minor)
	}

	temp := 0
	tempVal, ret := device.GetTemperature(nvml.TEMPERATURE_GPU)
	if ret == nvml.SUCCESS {
		temp = int(tempVal)
	}

	util := 0
	utilRates, ret := device.GetUtilizationRates()
	if ret == nvml.SUCCESS {
		util = int(utilRates.Gpu)
	}

	return GPUInfo{
		Model:             name,
		UUID:              uuid,
		VRAMMB:            vramMB,
		DriverVersion:     driverVersion,
		CUDAVersion:       cudaVersion,
		ComputeCapability: computeCap,
		Temperature:       temp,
		UtilizationPct:    util,
	}, nil
}

func (r *realGPUDiscoverer) close() {
	nvml.Shutdown()
}
