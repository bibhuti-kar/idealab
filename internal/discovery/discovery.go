package discovery

import "fmt"

// CPUInfo holds discovered CPU properties.
type CPUInfo struct {
	Model        string   `json:"model"`
	Cores        int      `json:"cores"`
	Threads      int      `json:"threads"`
	Architecture string   `json:"architecture"`
	Features     []string `json:"features,omitempty"`
}

// GPUInfo holds discovered GPU properties from NVML.
type GPUInfo struct {
	Model             string `json:"model"`
	UUID              string `json:"uuid"`
	VRAMMB            int    `json:"vramMB"`
	VRAMUsedMB        int    `json:"vramUsedMB,omitempty"`
	DriverVersion     string `json:"driverVersion"`
	CUDAVersion       string `json:"cudaVersion"`
	ComputeCapability string `json:"computeCapability"`
	Temperature       int    `json:"temperature,omitempty"`
	UtilizationPct    int    `json:"utilizationPct,omitempty"`
	PowerWatts        int    `json:"powerWatts,omitempty"`
}

// MemoryInfo holds discovered system memory properties.
type MemoryInfo struct {
	TotalMB     int `json:"totalMB"`
	AvailableMB int `json:"availableMB"`
}

// DeviceInfo is the complete hardware discovery result.
type DeviceInfo struct {
	Hostname string     `json:"hostname"`
	CPU      CPUInfo    `json:"cpu"`
	GPUs     []GPUInfo  `json:"gpus"`
	Memory   MemoryInfo `json:"memory"`
}

// Discoverer enumerates hardware on the current node.
type Discoverer interface {
	Discover() (DeviceInfo, error)
}

// PrimaryGPU returns the first GPU or an error if none found.
func (d DeviceInfo) PrimaryGPU() (GPUInfo, error) {
	if len(d.GPUs) == 0 {
		return GPUInfo{}, fmt.Errorf("no GPUs discovered")
	}
	return d.GPUs[0], nil
}
