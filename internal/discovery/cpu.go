package discovery

import (
	"bufio"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// discoverCPU reads CPU information from /proc/cpuinfo.
func discoverCPU(logger *slog.Logger) (CPUInfo, error) {
	info := CPUInfo{
		Architecture: runtime.GOARCH,
		Threads:      runtime.NumCPU(),
	}

	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		logger.Warn("failed to open /proc/cpuinfo, using runtime defaults", "error", err)
		return info, nil
	}
	defer f.Close()

	var physicalIDs = make(map[string]bool)
	var coreIDs = make(map[string]bool)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "model name":
			if info.Model == "" {
				info.Model = value
			}
		case "physical id":
			physicalIDs[value] = true
		case "core id":
			coreIDs[value] = true
		case "flags":
			if len(info.Features) == 0 {
				info.Features = extractCPUFeatures(value)
			}
		}
	}

	if len(coreIDs) > 0 {
		info.Cores = len(coreIDs)
	} else {
		info.Cores = info.Threads
	}

	logger.Debug("CPU discovered",
		"model", info.Model,
		"cores", info.Cores,
		"threads", info.Threads,
	)
	return info, nil
}

// extractCPUFeatures filters CPU flags to a useful subset.
func extractCPUFeatures(flagsLine string) []string {
	relevant := map[string]string{
		"sse4_1": "SSE4.1",
		"sse4_2": "SSE4.2",
		"avx":    "AVX",
		"avx2":   "AVX2",
		"avx512f": "AVX-512",
		"fma":    "FMA",
		"aes":    "AES-NI",
		"vnni":   "VNNI",
		"amx_tile": "AMX",
	}

	flags := strings.Fields(flagsLine)
	var features []string
	for _, flag := range flags {
		if name, ok := relevant[flag]; ok {
			features = append(features, name)
		}
	}
	return features
}

// parseProcInt parses an integer from a /proc file value.
func parseProcInt(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
