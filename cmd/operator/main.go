package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	v1alpha1 "github.com/bibhuti-kar/idealab/api/v1alpha1"
	"github.com/bibhuti-kar/idealab/internal/controller"
	"github.com/bibhuti-kar/idealab/internal/discovery"
	healthpkg "github.com/bibhuti-kar/idealab/internal/health"
)

func main() {
	logger := setupLogger()
	logger.Info("idealab operator starting")

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))

	healthPort := envInt("HEALTH_PORT", 8081)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		LeaderElection: false,
	})
	if err != nil {
		logger.Error("failed to create manager", "error", err)
		os.Exit(1)
	}

	discoverer := createDiscoverer(logger)

	reconciler := &controller.GPUClusterReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		Discoverer: discoverer,
		Logger:     logger,
		Recorder:   mgr.GetEventRecorderFor("idealab-operator"),
	}

	if err := reconciler.SetupWithManager(mgr); err != nil {
		logger.Error("failed to setup reconciler", "error", err)
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Error("failed to add healthz check", "error", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", func(_ *http.Request) error {
		if reconciler.IsReconciled() {
			return nil
		}
		return &healthz.NotReadyError{Reason: "not yet reconciled"}
	}); err != nil {
		logger.Error("failed to add readyz check", "error", err)
		os.Exit(1)
	}

	// Start standalone health server for external probes.
	healthServer := healthpkg.NewServer(healthPort, reconciler.IsReconciled, logger)
	go func() {
		if err := healthServer.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server failed", "error", err)
			os.Exit(1)
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		logger.Error("manager exited with error", "error", err)
		os.Exit(1)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5000000000)
	defer shutdownCancel()
	healthServer.Shutdown(shutdownCtx)

	logger.Info("idealab operator stopped")
}

func createDiscoverer(logger *slog.Logger) discovery.Discoverer {
	if envBool("MOCK_DISCOVERY", false) {
		logger.Info("using mock discoverer (MOCK_DISCOVERY=true)")
		return discovery.NewMockDiscoverer()
	}
	return discovery.NewNVMLDiscoverer(logger)
}

func setupLogger() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(envStr("LOG_LEVEL", "info")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	format := envStr("LOG_FORMAT", "json")
	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}

	return slog.New(handler)
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
