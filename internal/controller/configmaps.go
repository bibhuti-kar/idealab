package controller

import (
	"context"
	"fmt"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/bibhuti-kar/idealab/api/v1alpha1"
	"github.com/bibhuti-kar/idealab/internal/config"
	"github.com/bibhuti-kar/idealab/internal/discovery"
)

const (
	labelGPUCluster = "idealab.io/gpucluster"
	labelProfile    = "idealab.io/profile"
	labelManagedBy  = "app.kubernetes.io/managed-by"
	managedByValue  = "idealab-operator"
	valuesKey       = "values.yaml"
)

// reconcileConfigMaps generates or updates a ConfigMap per application profile.
func (r *GPUClusterReconciler) reconcileConfigMaps(ctx context.Context, logger *slog.Logger, gc *v1alpha1.GPUCluster, info discovery.DeviceInfo) error {
	hw := buildHardwareInfo(info)
	activeNames := make(map[string]bool)
	var statuses []v1alpha1.ProfileStatus

	for _, ap := range gc.Spec.ApplicationProfiles {
		cmName := configMapName(gc.Name, ap.Name)
		activeNames[cmName] = true

		pi := config.ProfileInput{
			Name:        ap.Name,
			HelmChart:   ap.HelmChart,
			HelmValues:  ap.HelmValues,
			GPUCount:    ap.Resources.GPUCount,
			GPUMemory:   ap.Resources.GPUMemory,
			CPULimit:    ap.Resources.CPULimit,
			MemoryLimit: ap.Resources.MemoryLimit,
		}

		values := config.GenerateValues(pi, hw)
		yamlData, err := config.RenderYAML(values)
		if err != nil {
			logger.Error("render values failed", "profile", ap.Name, "error", err)
			statuses = append(statuses, v1alpha1.ProfileStatus{Name: ap.Name, ConfigMapName: cmName})
			continue
		}

		if err := r.upsertConfigMap(ctx, logger, gc.Name, ap.Name, cmName, yamlData); err != nil {
			logger.Error("upsert configmap failed", "profile", ap.Name, "error", err)
			statuses = append(statuses, v1alpha1.ProfileStatus{Name: ap.Name, ConfigMapName: cmName})
			continue
		}

		statuses = append(statuses, v1alpha1.ProfileStatus{
			Name:          ap.Name,
			ConfigMapName: cmName,
			Ready:         true,
		})
	}

	gc.Status.ProfileStatuses = statuses

	if err := r.cleanupOrphanConfigMaps(ctx, logger, gc.Name, activeNames); err != nil {
		logger.Warn("orphan cleanup failed", "error", err)
	}

	return nil
}

func (r *GPUClusterReconciler) upsertConfigMap(ctx context.Context, logger *slog.Logger, gcName, profileName, cmName string, data []byte) error {
	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: r.Namespace,
			Labels: map[string]string{
				labelGPUCluster: gcName,
				labelProfile:    profileName,
				labelManagedBy:  managedByValue,
			},
		},
		Data: map[string]string{
			valuesKey: string(data),
		},
	}

	var existing corev1.ConfigMap
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), &existing)
	if errors.IsNotFound(err) {
		logger.Info("creating configmap", "name", cmName, "profile", profileName)
		return r.Create(ctx, desired)
	}
	if err != nil {
		return fmt.Errorf("get configmap %s: %w", cmName, err)
	}

	existing.Labels = desired.Labels
	existing.Data = desired.Data
	logger.Debug("updating configmap", "name", cmName, "profile", profileName)
	return r.Update(ctx, &existing)
}

func (r *GPUClusterReconciler) cleanupOrphanConfigMaps(ctx context.Context, logger *slog.Logger, gcName string, activeNames map[string]bool) error {
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
		if !activeNames[cm.Name] {
			logger.Info("deleting orphan configmap", "name", cm.Name)
			if err := r.Delete(ctx, cm); err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("delete configmap %s: %w", cm.Name, err)
			}
		}
	}
	return nil
}

func configMapName(gcName, profileName string) string {
	return fmt.Sprintf("%s-%s-values", gcName, profileName)
}

// checkResourceWarning sets a warning on the CR status if profiles overcommit VRAM.
func (r *GPUClusterReconciler) checkResourceWarning(logger *slog.Logger, gc *v1alpha1.GPUCluster, info discovery.DeviceInfo) {
	var profiles []config.ProfileInput
	for _, ap := range gc.Spec.ApplicationProfiles {
		profiles = append(profiles, config.ProfileInput{
			Name:      ap.Name,
			GPUMemory: ap.Resources.GPUMemory,
		})
	}

	vramMB := 0
	if gpu, err := info.PrimaryGPU(); err == nil {
		vramMB = gpu.VRAMMB
	}

	warning := config.CheckResourceOvercommit(profiles, vramMB)
	gc.Status.ResourceWarning = warning
	if warning != "" {
		logger.Warn(warning)
	}
}

func buildHardwareInfo(info discovery.DeviceInfo) config.HardwareInfo {
	hw := config.HardwareInfo{
		CPUCores:      info.CPU.Cores,
		MemoryTotalMB: info.Memory.TotalMB,
		GPUCount:      len(info.GPUs),
	}
	if gpu, err := info.PrimaryGPU(); err == nil {
		hw.GPUModel = gpu.Model
		hw.VRAMMB = gpu.VRAMMB
		hw.ComputeCapability = gpu.ComputeCapability
		hw.CUDAVersion = gpu.CUDAVersion
		hw.DriverVersion = gpu.DriverVersion
	}
	return hw
}
