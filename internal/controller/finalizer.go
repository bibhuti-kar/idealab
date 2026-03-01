package controller

import (
	"context"
	"fmt"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha1 "github.com/bibhuti-kar/idealab/api/v1alpha1"
)

const finalizerName = "idealab.io/configmap-cleanup"

// ensureFinalizer adds the cleanup finalizer if not already present.
func (r *GPUClusterReconciler) ensureFinalizer(ctx context.Context, gc *v1alpha1.GPUCluster) error {
	if controllerutil.ContainsFinalizer(gc, finalizerName) {
		return nil
	}
	controllerutil.AddFinalizer(gc, finalizerName)
	return r.Update(ctx, gc)
}

// handleDeletion cleans up ConfigMaps and removes the finalizer.
// Returns true if the object is being deleted and the caller should return.
func (r *GPUClusterReconciler) handleDeletion(ctx context.Context, logger *slog.Logger, gc *v1alpha1.GPUCluster) (bool, error) {
	if gc.DeletionTimestamp.IsZero() {
		return false, nil
	}

	if !controllerutil.ContainsFinalizer(gc, finalizerName) {
		return true, nil
	}

	logger.Info("cleaning up configmaps for deleted GPUCluster", "name", gc.Name)
	if err := r.deleteAllConfigMaps(ctx, logger, gc.Name); err != nil {
		return true, fmt.Errorf("cleanup configmaps: %w", err)
	}

	controllerutil.RemoveFinalizer(gc, finalizerName)
	if err := r.Update(ctx, gc); err != nil {
		return true, fmt.Errorf("remove finalizer: %w", err)
	}

	logger.Info("finalizer removed, deletion complete")
	return true, nil
}

func (r *GPUClusterReconciler) deleteAllConfigMaps(ctx context.Context, logger *slog.Logger, gcName string) error {
	var cmList corev1.ConfigMapList
	if err := r.List(ctx, &cmList,
		client.InNamespace(r.Namespace),
		client.MatchingLabels{
			labelGPUCluster: gcName,
			labelManagedBy:  managedByValue,
		},
	); err != nil {
		return fmt.Errorf("list configmaps: %w", err)
	}

	for i := range cmList.Items {
		cm := &cmList.Items[i]
		logger.Info("deleting configmap", "name", cm.Name)
		if err := r.Delete(ctx, cm); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("delete configmap %s: %w", cm.Name, err)
		}
	}
	return nil
}
