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
)

const (
	reconcileInterval = 5 * time.Minute
	errorRequeueBase  = 5 * time.Second
	errorRequeueMax   = 5 * time.Minute
)

// GPUClusterReconciler reconciles GPUCluster resources.
type GPUClusterReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Discoverer discovery.Discoverer
	Logger     *slog.Logger
	Recorder   record.EventRecorder
	reconciled atomic.Bool
}

// IsReconciled returns true if at least one successful reconciliation has occurred.
func (r *GPUClusterReconciler) IsReconciled() bool {
	return r.reconciled.Load()
}

// Reconcile handles a single reconciliation cycle for a GPUCluster resource.
func (r *GPUClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger.With("gpucluster", req.Name)

	// Step 1: Fetch the GPUCluster resource.
	var gc v1alpha1.GPUCluster
	if err := r.Get(ctx, req.NamespacedName, &gc); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("GPUCluster deleted, nothing to reconcile")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetch GPUCluster: %w", err)
	}

	// Step 2: Set phase to Discovering if new or pending.
	if gc.Status.Phase == "" || gc.Status.Phase == v1alpha1.PhasePending {
		gc.Status.Phase = v1alpha1.PhaseDiscovering
		setCondition(&gc, metav1.Condition{
			Type:               "Discovering",
			Status:             metav1.ConditionTrue,
			Reason:             "DiscoveryInProgress",
			Message:            "Hardware discovery is running",
			LastTransitionTime: metav1.Now(),
		})
		if err := r.Status().Update(ctx, &gc); err != nil {
			return ctrl.Result{}, fmt.Errorf("update discovering status: %w", err)
		}
		r.Recorder.Event(&gc, corev1.EventTypeNormal, "DiscoveryStarted", "Hardware discovery initiated")
		logger.Info("discovery started", "phase", gc.Status.Phase)
	}

	// Step 3: Run device discovery.
	deviceInfo, err := r.Discoverer.Discover()
	if err != nil {
		logger.Error("device discovery failed", "error", err)
		gc.Status.Phase = v1alpha1.PhaseError
		setCondition(&gc, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "DiscoveryFailed",
			Message:            err.Error(),
			LastTransitionTime: metav1.Now(),
		})
		setCondition(&gc, metav1.Condition{
			Type:               "Discovering",
			Status:             metav1.ConditionFalse,
			Reason:             "DiscoveryFailed",
			Message:            err.Error(),
			LastTransitionTime: metav1.Now(),
		})
		if updateErr := r.Status().Update(ctx, &gc); updateErr != nil {
			logger.Error("failed to update error status", "error", updateErr)
		}
		r.Recorder.Event(&gc, corev1.EventTypeWarning, "DiscoveryFailed", err.Error())
		return ctrl.Result{RequeueAfter: errorRequeueBase}, nil
	}

	// Step 4: Map DeviceInfo to GPUCluster status.
	gc.Status.Node = mapDeviceInfoToNodeInfo(deviceInfo)

	// Step 5: Label the node with GPU metadata.
	if err := r.labelNode(ctx, logger, deviceInfo); err != nil {
		logger.Warn("node labeling failed, continuing", "error", err)
		r.Recorder.Event(&gc, corev1.EventTypeWarning, "NodeLabelFailed", err.Error())
	}

	// Step 6: Set Ready status.
	gc.Status.Phase = v1alpha1.PhaseReady
	setCondition(&gc, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "ReconcileSucceeded",
		Message:            "Device discovery completed successfully",
		LastTransitionTime: metav1.Now(),
	})
	setCondition(&gc, metav1.Condition{
		Type:               "Discovering",
		Status:             metav1.ConditionFalse,
		Reason:             "DiscoveryComplete",
		Message:            "",
		LastTransitionTime: metav1.Now(),
	})
	if err := r.Status().Update(ctx, &gc); err != nil {
		return ctrl.Result{}, fmt.Errorf("update ready status: %w", err)
	}

	r.Recorder.Event(&gc, corev1.EventTypeNormal, "ReconcileSucceeded", "GPUCluster is ready")
	r.reconciled.Store(true)

	logger.Info("reconciliation complete",
		"phase", gc.Status.Phase,
		"gpu", gc.Status.Node.GPU.Model,
		"vramMB", gc.Status.Node.GPU.VRAMMB,
	)

	// Step 7: Requeue for periodic refresh.
	return ctrl.Result{RequeueAfter: reconcileInterval}, nil
}

// SetupWithManager registers the reconciler with the controller-runtime manager.
func (r *GPUClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.GPUCluster{}).
		Complete(r)
}

// mapDeviceInfoToNodeInfo converts discovery output to CRD status format.
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
