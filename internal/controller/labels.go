package controller

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bibhuti-kar/idealab/internal/discovery"
)

const labelPrefix = "idealab.io/"

// labelNode applies GPU metadata labels to the k3s node.
func (r *GPUClusterReconciler) labelNode(ctx context.Context, logger *slog.Logger, info discovery.DeviceInfo) error {
	gpu, err := info.PrimaryGPU()
	if err != nil {
		return fmt.Errorf("no GPU to label: %w", err)
	}

	// Find the node by hostname.
	var nodeList corev1.NodeList
	if err := r.List(ctx, &nodeList); err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	if len(nodeList.Items) == 0 {
		return fmt.Errorf("no nodes found in cluster")
	}

	// Single-node k3s: use the first (and only) node.
	node := &nodeList.Items[0]
	original := node.DeepCopy()

	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}

	labels := gpuLabels(gpu)
	for k, v := range labels {
		node.Labels[k] = v
	}

	if err := r.Patch(ctx, node, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch node labels: %w", err)
	}

	logger.Info("node labeled with GPU metadata",
		"node", node.Name,
		"labels", labels,
	)
	return nil
}

// gpuLabels generates the label map for a GPU.
func gpuLabels(gpu discovery.GPUInfo) map[string]string {
	return map[string]string{
		labelPrefix + "gpu-model":   sanitizeLabel(gpu.Model),
		labelPrefix + "gpu-vram-mb": strconv.Itoa(gpu.VRAMMB),
		labelPrefix + "gpu-driver":  gpu.DriverVersion,
		labelPrefix + "gpu-cuda":    gpu.CUDAVersion,
		labelPrefix + "gpu-compute": gpu.ComputeCapability,
	}
}

// sanitizeLabel converts a GPU model name to a valid Kubernetes label value.
// Kubernetes label values must be <= 63 characters, alphanumeric or '-._'.
func sanitizeLabel(s string) string {
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return -1
	}, s)
	if len(s) > 63 {
		s = s[:63]
	}
	return s
}
