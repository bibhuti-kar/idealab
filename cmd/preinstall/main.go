package main

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("idealab preinstall starting")

	scriptPath := findScript()
	if scriptPath == "" {
		logger.Error("preinstall.sh not found")
		os.Exit(1)
	}

	logger.Info("running preinstall script", "path", scriptPath)

	cmd := exec.Command("bash", scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Error("preinstall script failed", "error", err)
		os.Exit(1)
	}

	logger.Info("preinstall complete")
}

func findScript() string {
	candidates := []string{
		"scripts/preinstall.sh",
		"../scripts/preinstall.sh",
		"/app/scripts/preinstall.sh",
	}

	execPath, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(execPath)
		candidates = append(candidates, filepath.Join(dir, "..", "..", "scripts", "preinstall.sh"))
	}

	for _, path := range candidates {
		abs, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
	}
	return ""
}
