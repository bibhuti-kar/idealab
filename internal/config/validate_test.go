package config

import (
	"strings"
	"testing"
)

func TestCheckResourceOvercommit_UnderLimit(t *testing.T) {
	profiles := []ProfileInput{
		{Name: "a", GPUMemory: "2Gi"},
		{Name: "b", GPUMemory: "1Gi"},
	}
	// 6144 - 512 reserve = 5632 usable; 2048 + 1024 = 3072 requested
	warn := CheckResourceOvercommit(profiles, 6144)
	if warn != "" {
		t.Errorf("expected no warning, got: %s", warn)
	}
}

func TestCheckResourceOvercommit_OverLimit(t *testing.T) {
	profiles := []ProfileInput{
		{Name: "big", GPUMemory: "4Gi"},
		{Name: "also-big", GPUMemory: "3Gi"},
	}
	// 6144 - 512 = 5632 usable; 4096 + 3072 = 7168 requested
	warn := CheckResourceOvercommit(profiles, 6144)
	if warn == "" {
		t.Fatal("expected warning for overcommit")
	}
	if !strings.Contains(warn, "ResourceWarning") {
		t.Errorf("expected ResourceWarning prefix, got: %s", warn)
	}
	if !strings.Contains(warn, "big") {
		t.Errorf("expected profile name in warning, got: %s", warn)
	}
}

func TestCheckResourceOvercommit_NoGPUMemorySet(t *testing.T) {
	profiles := []ProfileInput{
		{Name: "cpu-only"},
	}
	warn := CheckResourceOvercommit(profiles, 6144)
	if warn != "" {
		t.Errorf("expected no warning for CPU-only profile, got: %s", warn)
	}
}

func TestCheckResourceOvercommit_ZeroAvailable(t *testing.T) {
	profiles := []ProfileInput{
		{Name: "any", GPUMemory: "1Gi"},
	}
	warn := CheckResourceOvercommit(profiles, 0)
	if warn != "" {
		t.Errorf("expected no warning with zero available, got: %s", warn)
	}
}
