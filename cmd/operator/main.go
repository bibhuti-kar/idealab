package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

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
	"github.com/bibhuti-kar/idealab/internal/metrics"
)

func main() {
	logger := setupLogger()
	logger.Info("idealab operator starting")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, logger); err != nil {
		logger.Error("operator failed", "error", err)
		os.Exit(1)
	}
	logger.Info("idealab operator stopped")
}

func run(ctx context.Context, logger *slog.Logger) error {
	scheme := setupScheme()
	healthPort := envInt("HEALTH_PORT", 8081)

	metrics.RegisterAll()

	mgr, err := setupManager(scheme)
	if err != nil {
		return fmt.Errorf("create manager: %w", err)
	}

	reconciler := setupReconciler(mgr, logger)
	if err := reconciler.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup reconciler: %w", err)
	}

	if err := addHealthChecks(mgr, reconciler); err != nil {
		return fmt.Errorf("add health checks: %w", err)
	}

	healthServer := healthpkg.NewServer(healthPort, reconciler.IsReconciled, logger)
	go func() {
		if err := healthServer.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server failed", "error", err)
			cancel := ctx.Done
			_ = cancel
		}
	}()

	logger.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("manager run: %w", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		logger.Warn("health server shutdown error", "error", err)
	}
	return nil
}

func setupScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(corev1.AddToScheme(s))
	utilruntime.Must(v1alpha1.AddToScheme(s))
	return s
}

func setupManager(scheme *runtime.Scheme) (ctrl.Manager, error) {
	metricsAddr := envStr("METRICS_BIND_ADDRESS", ":8080")
	return ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		LeaderElection: false,
	})
}

func setupReconciler(mgr ctrl.Manager, logger *slog.Logger) *controller.GPUClusterReconciler {
	ns := envStr("POD_NAMESPACE", "idealab-system")
	return &controller.GPUClusterReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		Discoverer: createDiscoverer(logger),
		Logger:     logger,
		Recorder:   mgr.GetEventRecorderFor("idealab-operator"),
		Namespace:  ns,
	}
}

func addHealthChecks(mgr ctrl.Manager, r *controller.GPUClusterReconciler) error {
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("healthz: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", func(_ *http.Request) error {
		if r.IsReconciled() {
			return nil
		}
		return fmt.Errorf("not yet reconciled")
	}); err != nil {
		return fmt.Errorf("readyz: %w", err)
	}
	return nil
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
