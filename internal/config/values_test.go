package config

import (
	"testing"
)

func testHardware() HardwareInfo {
	return HardwareInfo{
		GPUModel:          "NVIDIA GeForce GTX 1660 Ti",
		VRAMMB:            6144,
		GPUCount:          1,
		ComputeCapability: "7.5",
		CUDAVersion:       "12.6",
		DriverVersion:     "560.35.03",
		CPUCores:          4,
		MemoryTotalMB:     16384,
	}
}

func TestGenerateValues_HardwareDefaults(t *testing.T) {
	profile := ProfileInput{Name: "test", HelmChart: "test/chart"}
	vals := GenerateValues(profile, testHardware())

	gpu, ok := vals["gpu"].(map[string]any)
	if !ok {
		t.Fatal("expected gpu map")
	}
	if gpu["model"] != "NVIDIA GeForce GTX 1660 Ti" {
		t.Errorf("unexpected model: %v", gpu["model"])
	}
	// 6144 - 512 = 5632
	if gpu["vramMB"] != 5632 {
		t.Errorf("expected vramMB=5632, got %v", gpu["vramMB"])
	}

	res, ok := vals["resources"].(map[string]any)
	if !ok {
		t.Fatal("expected resources map")
	}
	limits, ok := res["limits"].(map[string]any)
	if !ok {
		t.Fatal("expected limits map")
	}
	if limits["nvidia.com/gpu"] != "1" {
		t.Errorf("expected gpu limit=1, got %v", limits["nvidia.com/gpu"])
	}
}

func TestGenerateValues_UserOverrides(t *testing.T) {
	profile := ProfileInput{
		Name:      "custom",
		HelmChart: "test/chart",
		HelmValues: map[string]any{
			"gpu": map[string]any{
				"vramMB":  9999,
				"custom":  true,
			},
			"extra": "value",
		},
	}
	vals := GenerateValues(profile, testHardware())

	gpu, ok := vals["gpu"].(map[string]any)
	if !ok {
		t.Fatal("expected gpu map")
	}
	// User override wins
	if gpu["vramMB"] != 9999 {
		t.Errorf("expected user vramMB=9999, got %v", gpu["vramMB"])
	}
	// Hardware default preserved
	if gpu["model"] != "NVIDIA GeForce GTX 1660 Ti" {
		t.Errorf("expected model preserved, got %v", gpu["model"])
	}
	// User addition preserved
	if gpu["custom"] != true {
		t.Errorf("expected custom=true, got %v", gpu["custom"])
	}
	if vals["extra"] != "value" {
		t.Errorf("expected extra=value, got %v", vals["extra"])
	}
}

func TestGenerateValues_NoGPU(t *testing.T) {
	hw := HardwareInfo{CPUCores: 4, MemoryTotalMB: 16384}
	profile := ProfileInput{Name: "cpu-only", HelmChart: "test/chart"}
	vals := GenerateValues(profile, hw)

	if _, ok := vals["gpu"]; ok {
		t.Error("expected no gpu key for CPU-only hardware")
	}

	res, ok := vals["resources"].(map[string]any)
	if !ok {
		t.Fatal("expected resources map")
	}
	limits := res["limits"].(map[string]any)
	if _, ok := limits["nvidia.com/gpu"]; ok {
		t.Error("expected no GPU resource limit")
	}
}

func TestGenerateValues_ProfileResourceLimits(t *testing.T) {
	profile := ProfileInput{
		Name:        "limited",
		HelmChart:   "test/chart",
		CPULimit:    "2",
		MemoryLimit: "4Gi",
		GPUCount:    1,
	}
	vals := GenerateValues(profile, testHardware())

	limits := vals["resources"].(map[string]any)["limits"].(map[string]any)
	if limits["cpu"] != "2" {
		t.Errorf("expected cpu=2, got %v", limits["cpu"])
	}
	if limits["memory"] != "4Gi" {
		t.Errorf("expected memory=4Gi, got %v", limits["memory"])
	}
}

func TestGenerateValues_VRAMReserve(t *testing.T) {
	hw := HardwareInfo{GPUModel: "tiny", VRAMMB: 256, GPUCount: 1}
	profile := ProfileInput{Name: "test", HelmChart: "test/chart"}
	vals := GenerateValues(profile, hw)

	gpu := vals["gpu"].(map[string]any)
	if gpu["vramMB"] != 0 {
		t.Errorf("expected vramMB=0 (below reserve), got %v", gpu["vramMB"])
	}
}

func TestParseGPUMemoryMB(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"4Gi", 4096},
		{"2048Mi", 2048},
		{"1024", 1024},
		{"", 0},
		{"invalid", 0},
	}
	for _, tt := range tests {
		got := ParseGPUMemoryMB(tt.input)
		if got != tt.want {
			t.Errorf("ParseGPUMemoryMB(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
