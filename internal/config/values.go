package config

import (
	"fmt"
	"strconv"
	"strings"
)

const vramReserveMB = 512

// HardwareInfo holds the discovered hardware context needed for value generation.
type HardwareInfo struct {
	GPUModel          string
	VRAMMB            int
	GPUCount          int
	ComputeCapability string
	CUDAVersion       string
	DriverVersion     string
	CPUCores          int
	MemoryTotalMB     int
}

// ProfileInput holds the application profile fields needed for value generation.
type ProfileInput struct {
	Name        string
	HelmChart   string
	HelmValues  map[string]any
	GPUCount    int
	GPUMemory   string
	CPULimit    string
	MemoryLimit string
}

// GenerateValues produces a Helm values map by combining hardware defaults
// with user-supplied overrides. User values win via deep merge.
func GenerateValues(profile ProfileInput, hw HardwareInfo) map[string]any {
	defaults := hardwareDefaults(profile, hw)
	return mergeMaps(defaults, profile.HelmValues)
}

func hardwareDefaults(profile ProfileInput, hw HardwareInfo) map[string]any {
	values := map[string]any{}

	if hw.GPUModel != "" {
		usableVRAM := hw.VRAMMB - vramReserveMB
		if usableVRAM < 0 {
			usableVRAM = 0
		}
		values["gpu"] = map[string]any{
			"model":             hw.GPUModel,
			"vramMB":            usableVRAM,
			"computeCapability": hw.ComputeCapability,
			"cudaVersion":       hw.CUDAVersion,
			"driverVersion":     hw.DriverVersion,
		}
	}

	gpuCount := profile.GPUCount
	if gpuCount == 0 && hw.GPUCount > 0 {
		gpuCount = 1
	}

	limits := map[string]any{}
	if gpuCount > 0 {
		limits["nvidia.com/gpu"] = strconv.Itoa(gpuCount)
	}
	if profile.CPULimit != "" {
		limits["cpu"] = profile.CPULimit
	} else if hw.CPUCores > 0 {
		limits["cpu"] = fmt.Sprintf("%d", hw.CPUCores)
	}
	if profile.MemoryLimit != "" {
		limits["memory"] = profile.MemoryLimit
	} else if hw.MemoryTotalMB > 0 {
		limits["memory"] = fmt.Sprintf("%dMi", hw.MemoryTotalMB/2)
	}

	if len(limits) > 0 {
		values["resources"] = map[string]any{
			"limits": limits,
		}
	}

	return values
}

// ParseGPUMemoryMB converts a GPU memory string (e.g. "4Gi", "2048Mi", "2048")
// to megabytes. Returns 0 if the string is empty or unparseable.
func ParseGPUMemoryMB(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if strings.HasSuffix(s, "Gi") {
		v, err := strconv.Atoi(strings.TrimSuffix(s, "Gi"))
		if err != nil {
			return 0
		}
		return v * 1024
	}
	if strings.HasSuffix(s, "Mi") {
		v, err := strconv.Atoi(strings.TrimSuffix(s, "Mi"))
		if err != nil {
			return 0
		}
		return v
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}
