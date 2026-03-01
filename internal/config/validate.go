package config

import (
	"fmt"
	"strings"
)

// CheckResourceOvercommit returns a warning message if the total GPU memory
// requested by all profiles exceeds availableVRAMMB. Returns empty string
// if within limits or if no profiles request GPU memory.
func CheckResourceOvercommit(profiles []ProfileInput, availableVRAMMB int) string {
	if availableVRAMMB <= 0 {
		return ""
	}

	totalRequested := 0
	var names []string
	for _, p := range profiles {
		mb := ParseGPUMemoryMB(p.GPUMemory)
		if mb > 0 {
			totalRequested += mb
			names = append(names, p.Name)
		}
	}

	if totalRequested == 0 {
		return ""
	}

	usable := availableVRAMMB - vramReserveMB
	if usable < 0 {
		usable = 0
	}

	if totalRequested > usable {
		return fmt.Sprintf("ResourceWarning: profiles [%s] request %dMB total GPU memory, but only %dMB usable (%dMB total - %dMB reserve)",
			strings.Join(names, ", "), totalRequested, usable, availableVRAMMB, vramReserveMB)
	}
	return ""
}
