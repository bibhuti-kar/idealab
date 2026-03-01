package controller

import (
	"context"
	"log/slog"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha1 "github.com/bibhuti-kar/idealab/api/v1alpha1"
)

func TestEnsureFinalizer_Add(t *testing.T) {
	scheme := testScheme()
	gc := &v1alpha1.GPUCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-gc"},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(gc).Build()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	r := &GPUClusterReconciler{
		Client:    client,
		Scheme:    scheme,
		Namespace: "test-ns",
		Logger:    logger,
	}

	ctx := context.Background()
	if err := r.ensureFinalizer(ctx, gc); err != nil {
		t.Fatalf("ensureFinalizer: %v", err)
	}

	if !controllerutil.ContainsFinalizer(gc, finalizerName) {
		t.Error("expected finalizer to be added")
	}
}

func TestEnsureFinalizer_AlreadyPresent(t *testing.T) {
	scheme := testScheme()
	gc := &v1alpha1.GPUCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-gc",
			Finalizers: []string{finalizerName},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(gc).Build()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	r := &GPUClusterReconciler{
		Client:    client,
		Scheme:    scheme,
		Namespace: "test-ns",
		Logger:    logger,
	}

	ctx := context.Background()
	if err := r.ensureFinalizer(ctx, gc); err != nil {
		t.Fatalf("ensureFinalizer should be nop: %v", err)
	}
}

func TestHandleDeletion_NotDeleting(t *testing.T) {
	scheme := testScheme()
	gc := &v1alpha1.GPUCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-gc"},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	r := &GPUClusterReconciler{
		Client:    client,
		Scheme:    scheme,
		Namespace: "test-ns",
		Logger:    logger,
	}

	ctx := context.Background()
	deleting, err := r.handleDeletion(ctx, logger, gc)
	if err != nil {
		t.Fatalf("handleDeletion: %v", err)
	}
	if deleting {
		t.Error("expected deleting=false for non-deleted resource")
	}
}

func TestHandleDeletion_CleansUpConfigMaps(t *testing.T) {
	scheme := testScheme()
	now := metav1.Now()

	gc := &v1alpha1.GPUCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-gc",
			DeletionTimestamp: &now,
			Finalizers:        []string{finalizerName},
		},
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-gc-ollama-values",
			Namespace: "test-ns",
			Labels: map[string]string{
				labelGPUCluster: "test-gc",
				labelManagedBy:  managedByValue,
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(gc, cm).Build()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	r := &GPUClusterReconciler{
		Client:    client,
		Scheme:    scheme,
		Namespace: "test-ns",
		Logger:    logger,
	}

	ctx := context.Background()
	deleting, err := r.handleDeletion(ctx, logger, gc)
	if err != nil {
		t.Fatalf("handleDeletion: %v", err)
	}
	if !deleting {
		t.Error("expected deleting=true")
	}

	// ConfigMap should be deleted.
	var checkCM corev1.ConfigMap
	key := types.NamespacedName{Name: "test-gc-ollama-values", Namespace: "test-ns"}
	if err := client.Get(ctx, key, &checkCM); err == nil {
		t.Error("expected configmap to be deleted")
	}

	// Finalizer should be removed.
	if controllerutil.ContainsFinalizer(gc, finalizerName) {
		t.Error("expected finalizer to be removed")
	}
}
