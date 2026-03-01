package controller

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/bibhuti-kar/idealab/api/v1alpha1"
	"github.com/bibhuti-kar/idealab/internal/discovery"
	"github.com/bibhuti-kar/idealab/internal/metrics"
)

const (
	reconcileInterval = 5 * time.Minute
	errorRequeueBase  = 5 * time.Second
)

// GPUClusterReconciler reconciles GPUCluster resources.
type GPUClusterReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Discoverer discovery.Discoverer
	Logger     *slog.Logger
	Recorder   record.EventRecorder
	Namespace  string
	reconciled atomic.Bool
}

// IsReconciled returns true after at least one successful reconciliation.
func (r *GPUClusterReconciler) IsReconciled() bool {
	return r.reconciled.Load()
}

// Reconcile handles a single reconciliation cycle for a GPUCluster.
func (r *GPUClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	logger := r.Logger.With("gpucluster", req.Name)

	var gc v1alpha1.GPUCluster
	if err := r.Get(ctx, req.NamespacedName, &gc); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetch GPUCluster: %w", err)
	}

	// Handle deletion with finalizer cleanup.
	deleting, err := r.handleDeletion(ctx, logger, &gc)
	if deleting || err != nil {
		return ctrl.Result{}, err
	}

	// Add finalizer if profiles are defined.
	if len(gc.Spec.ApplicationProfiles) > 0 {
		if err := r.ensureFinalizer(ctx, &gc); err != nil {
			return ctrl.Result{}, fmt.Errorf("ensure finalizer: %w", err)
		}
	}

	if err := r.ensureDiscovering(ctx, logger, &gc); err != nil {
		return ctrl.Result{}, err
	}

	deviceInfo, err := r.Discoverer.Discover()
	if err != nil {
		return r.handleDiscoveryError(ctx, logger, &gc, err)
	}

	gc.Status.Node = mapDeviceInfoToNodeInfo(deviceInfo)
	r.applyNodeLabels(ctx, logger, &gc, deviceInfo)
	recordGPUMetrics(deviceInfo)

	// Reconcile ConfigMaps for application profiles.
	if len(gc.Spec.ApplicationProfiles) > 0 {
		if err := r.reconcileConfigMaps(ctx, logger, &gc, deviceInfo); err != nil {
			logger.Error("configmap reconciliation failed", "error", err)
		}
		r.checkResourceWarning(logger, &gc, deviceInfo)
		recordConfigMapCount(len(gc.Status.ProfileStatuses))
	}

	result, err := r.markReady(ctx, logger, &gc)
	duration := time.Since(start).Seconds()
	metrics.ReconcileDuration.Observe(duration)
	if err != nil {
		metrics.ReconcileTotal.WithLabelValues("error").Inc()
	} else {
		metrics.ReconcileTotal.WithLabelValues("success").Inc()
	}
	return result, err
}

// SetupWithManager registers the reconciler with the manager.
func (r *GPUClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.GPUCluster{}).
		Complete(r)
}

func (r *GPUClusterReconciler) ensureDiscovering(ctx context.Context, logger *slog.Logger, gc *v1alpha1.GPUCluster) error {
	if gc.Status.Phase != "" && gc.Status.Phase != v1alpha1.PhasePending {
		return nil
	}
	gc.Status.Phase = v1alpha1.PhaseDiscovering
	setCondition(gc, metav1.Condition{
		Type:    "Discovering",
		Status:  metav1.ConditionTrue,
		Reason:  "DiscoveryInProgress",
		Message: "Hardware discovery is running",
	})
	if err := r.Status().Update(ctx, gc); err != nil {
		return fmt.Errorf("update discovering status: %w", err)
	}
	r.Recorder.Event(gc, corev1.EventTypeNormal, "DiscoveryStarted", "Hardware discovery initiated")
	logger.Info("discovery started", "phase", gc.Status.Phase)
	return nil
}

func (r *GPUClusterReconciler) handleDiscoveryError(ctx context.Context, logger *slog.Logger, gc *v1alpha1.GPUCluster, err error) (ctrl.Result, error) {
	logger.Error("device discovery failed", "error", err)
	gc.Status.Phase = v1alpha1.PhaseError
	setCondition(gc, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionFalse,
		Reason:  "DiscoveryFailed",
		Message: err.Error(),
	})
	setCondition(gc, metav1.Condition{
		Type:    "Discovering",
		Status:  metav1.ConditionFalse,
		Reason:  "DiscoveryFailed",
		Message: err.Error(),
	})
	if updateErr := r.Status().Update(ctx, gc); updateErr != nil {
		logger.Error("failed to update error status", "error", updateErr)
	}
	r.Recorder.Event(gc, corev1.EventTypeWarning, "DiscoveryFailed", err.Error())
	return ctrl.Result{RequeueAfter: errorRequeueBase}, nil
}

func (r *GPUClusterReconciler) applyNodeLabels(ctx context.Context, logger *slog.Logger, gc *v1alpha1.GPUCluster, info discovery.DeviceInfo) {
	if err := r.labelNode(ctx, logger, info); err != nil {
		logger.Warn("node labeling failed, continuing", "error", err)
		r.Recorder.Event(gc, corev1.EventTypeWarning, "NodeLabelFailed", err.Error())
	}
}

func (r *GPUClusterReconciler) markReady(ctx context.Context, logger *slog.Logger, gc *v1alpha1.GPUCluster) (ctrl.Result, error) {
	gc.Status.Phase = v1alpha1.PhaseReady
	setCondition(gc, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "ReconcileSucceeded",
		Message: "Device discovery completed successfully",
	})
	setCondition(gc, metav1.Condition{
		Type:    "Discovering",
		Status:  metav1.ConditionFalse,
		Reason:  "DiscoveryComplete",
		Message: "",
	})
	if err := r.Status().Update(ctx, gc); err != nil {
		return ctrl.Result{}, fmt.Errorf("update ready status: %w", err)
	}
	r.Recorder.Event(gc, corev1.EventTypeNormal, "ReconcileSucceeded", "GPUCluster is ready")
	r.reconciled.Store(true)
	logger.Info("reconciliation complete",
		"phase", gc.Status.Phase,
		"gpu", gc.Status.Node.GPU.Model,
	)
	return ctrl.Result{RequeueAfter: reconcileInterval}, nil
}

// mapDeviceInfoToNodeInfo converts discovery output to CRD status.
func mapDeviceInfoToNodeInfo(info discovery.DeviceInfo) v1alpha1.NodeInfo {
	node := v1alpha1.NodeInfo{
		Hostname: info.Hostname,
		CPU: v1alpha1.CPUNodeInfo{
			Model:    info.CPU.Model,
			Cores:    info.CPU.Cores,
			Threads:  info.CPU.Threads,
			Features: info.CPU.Features,
		},
		Memory: v1alpha1.MemoryNodeInfo{
			TotalMB: info.Memory.TotalMB,
		},
	}
	if gpu, err := info.PrimaryGPU(); err == nil {
		node.GPU = v1alpha1.GPUNodeInfo{
			Model:             gpu.Model,
			VRAMMB:            gpu.VRAMMB,
			DriverVersion:     gpu.DriverVersion,
			CUDAVersion:       gpu.CUDAVersion,
			ComputeCapability: gpu.ComputeCapability,
		}
	}
	return node
}
