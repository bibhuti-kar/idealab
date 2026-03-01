package discovery

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestMockDiscoverer_Success(t *testing.T) {
	mock := NewMockDiscoverer()
	info, err := mock.Discover()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if info.Hostname != "gaming-pc" {
		t.Errorf("expected hostname gaming-pc, got %s", info.Hostname)
	}
	if info.CPU.Model == "" {
		t.Error("expected CPU model to be set")
	}
	if len(info.GPUs) != 1 {
		t.Fatalf("expected 1 GPU, got %d", len(info.GPUs))
	}
	if info.GPUs[0].VRAMMB != 6144 {
		t.Errorf("expected 6144 MB VRAM, got %d", info.GPUs[0].VRAMMB)
	}
}

func TestMockDiscoverer_Error(t *testing.T) {
	mock := &MockDiscoverer{
		Err: errors.New("discovery failed"),
	}
	_, err := mock.Discover()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "discovery failed" {
		t.Errorf("expected 'discovery failed', got %s", err.Error())
	}
}

func TestDeviceInfo_PrimaryGPU_Found(t *testing.T) {
	info := DeviceInfo{
		GPUs: []GPUInfo{
			{Model: "GTX 1660 Ti", VRAMMB: 6144},
		},
	}
	gpu, err := info.PrimaryGPU()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if gpu.Model != "GTX 1660 Ti" {
		t.Errorf("expected GTX 1660 Ti, got %s", gpu.Model)
	}
}

func TestDeviceInfo_PrimaryGPU_NotFound(t *testing.T) {
	info := DeviceInfo{GPUs: nil}
	_, err := info.PrimaryGPU()
	if err == nil {
		t.Fatal("expected error for no GPUs")
	}
}

func TestDeviceInfo_JSONSerialization(t *testing.T) {
	mock := NewMockDiscoverer()
	info, _ := mock.Discover()

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var decoded DeviceInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if decoded.Hostname != info.Hostname {
		t.Errorf("hostname mismatch: %s vs %s", decoded.Hostname, info.Hostname)
	}
	if decoded.CPU.Cores != info.CPU.Cores {
		t.Errorf("cores mismatch: %d vs %d", decoded.CPU.Cores, info.CPU.Cores)
	}
	if len(decoded.GPUs) != len(info.GPUs) {
		t.Errorf("GPU count mismatch: %d vs %d", len(decoded.GPUs), len(info.GPUs))
	}
}

func TestCPUFeatures_Extraction(t *testing.T) {
	flagsLine := "fpu vme sse4_1 sse4_2 avx avx2 fma aes rdrand"
	features := extractCPUFeatures(flagsLine)

	expected := map[string]bool{
		"SSE4.1": true,
		"SSE4.2": true,
		"AVX":    true,
		"AVX2":   true,
		"FMA":    true,
		"AES-NI": true,
	}

	if len(features) != len(expected) {
		t.Errorf("expected %d features, got %d: %v", len(expected), len(features), features)
	}

	for _, f := range features {
		if !expected[f] {
			t.Errorf("unexpected feature: %s", f)
		}
	}
}

func TestParseMemValue(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"16384000 kB", 16384000},
		{"8192 kB", 8192},
		{"0 kB", 0},
	}

	for _, tt := range tests {
		got := parseMemValue(tt.input)
		if got != tt.want {
			t.Errorf("parseMemValue(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestNVMLDiscoverer_WithMockGPU(t *testing.T) {
	gpus := []GPUInfo{
		{
			Model:             "Test GPU",
			UUID:              "GPU-test-uuid",
			VRAMMB:            8192,
			DriverVersion:     "560.0",
			CUDAVersion:       "12.6",
			ComputeCapability: "8.0",
		},
	}

	d := NewTestableDiscoverer(nil, gpus, nil)
	info, err := d.Discover()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(info.GPUs) != 1 {
		t.Fatalf("expected 1 GPU, got %d", len(info.GPUs))
	}
	if info.GPUs[0].Model != "Test GPU" {
		t.Errorf("expected Test GPU, got %s", info.GPUs[0].Model)
	}
	if info.CPU.Threads < 1 {
		t.Error("expected at least 1 CPU thread")
	}
	if info.Memory.TotalMB < 1 {
		t.Error("expected memory > 0")
	}
}

func TestNVMLDiscoverer_GPUFailure(t *testing.T) {
	d := NewTestableDiscoverer(nil, nil, errors.New("nvml init failed"))
	_, err := d.Discover()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "nvml init failed") {
		t.Errorf("expected error to contain 'nvml init failed', got: %s", err.Error())
	}
}
