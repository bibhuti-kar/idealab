package discovery

import (
	"bufio"
	"log/slog"
	"os"
	"strings"
)

// discoverMemory reads memory information from /proc/meminfo.
func discoverMemory(logger *slog.Logger) (MemoryInfo, error) {
	info := MemoryInfo{}

	f, err := os.Open("/proc/meminfo")
	if err != nil {
		logger.Warn("failed to open /proc/meminfo", "error", err)
		return info, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Values in /proc/meminfo are in kB
		switch key {
		case "MemTotal":
			kb := parseMemValue(value)
			info.TotalMB = kb / 1024
		case "MemAvailable":
			kb := parseMemValue(value)
			info.AvailableMB = kb / 1024
		}

		if info.TotalMB > 0 && info.AvailableMB > 0 {
			break
		}
	}

	logger.Debug("memory discovered",
		"totalMB", info.TotalMB,
		"availableMB", info.AvailableMB,
	)
	return info, nil
}

// parseMemValue extracts the numeric kB value from a /proc/meminfo line value.
// Format: "16384000 kB"
func parseMemValue(s string) int {
	s = strings.TrimSuffix(strings.TrimSpace(s), " kB")
	return parseProcInt(s)
}
