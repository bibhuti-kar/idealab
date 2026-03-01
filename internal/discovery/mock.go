package discovery

// MockDiscoverer returns pre-configured DeviceInfo for testing.
type MockDiscoverer struct {
	Info DeviceInfo
	Err  error
}

// Discover returns the mocked DeviceInfo or error.
func (m *MockDiscoverer) Discover() (DeviceInfo, error) {
	if m.Err != nil {
		return DeviceInfo{}, m.Err
	}
	return m.Info, nil
}

// NewMockDiscoverer creates a MockDiscoverer with realistic test data
// resembling a GTX 1660 Ti Mobile system.
func NewMockDiscoverer() *MockDiscoverer {
	return &MockDiscoverer{
		Info: DeviceInfo{
			Hostname: "gaming-pc",
			CPU: CPUInfo{
				Model:        "Intel(R) Core(TM) i5-9300H CPU @ 2.40GHz",
				Cores:        4,
				Threads:      8,
				Architecture: "x86_64",
				Features:     []string{"SSE4.2", "AVX", "AVX2", "FMA"},
			},
			GPUs: []GPUInfo{
				{
					Model:             "NVIDIA GeForce GTX 1660 Ti",
					UUID:              "GPU-12345678-1234-1234-1234-123456789012",
					VRAMMB:            6144,
					VRAMUsedMB:        512,
					DriverVersion:     "560.35.03",
					CUDAVersion:       "12.6",
					ComputeCapability: "7.5",
					Temperature:       45,
					UtilizationPct:    0,
					PowerWatts:        15,
				},
			},
			Memory: MemoryInfo{
				TotalMB:     16384,
				AvailableMB: 12288,
			},
		},
	}
}
