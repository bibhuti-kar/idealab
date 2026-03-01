package controller

import (
	"context"
	"log/slog"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/bibhuti-kar/idealab/api/v1alpha1"
	"github.com/bibhuti-kar/idealab/internal/discovery"
)

func testScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = v1alpha1.AddToScheme(s)
	return s
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func testDeviceInfo() discovery.DeviceInfo {
	return discovery.NewMockDiscoverer().Info
}

func TestReconcileConfigMaps_Create(t *testing.T) {
	scheme := testScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	logger := testLogger()

	r := &GPUClusterReconciler{
		Client:    client,
		Scheme:    scheme,
		Namespace: "test-ns",
		Logger:    logger,
	}

	gc := &v1alpha1.GPUCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-gc"},
		Spec: v1alpha1.GPUClusterSpec{
			ApplicationProfiles: []v1alpha1.ApplicationProfile{
				{
					Name:      "ollama",
					HelmChart: "ollama/ollama",
					Resources: v1alpha1.ProfileResources{GPUCount: 1},
				},
			},
		},
	}

	ctx := context.Background()
	if err := r.reconcileConfigMaps(ctx, logger, gc, testDeviceInfo()); err != nil {
		t.Fatalf("reconcileConfigMaps: %v", err)
	}

	// Verify ConfigMap was created.
	var cm corev1.ConfigMap
	key := types.NamespacedName{Name: "test-gc-ollama-values", Namespace: "test-ns"}
	if err := client.Get(ctx, key, &cm); err != nil {
		t.Fatalf("expected configmap to exist: %v", err)
	}

	if cm.Labels[labelGPUCluster] != "test-gc" {
		t.Errorf("expected gpucluster label, got %v", cm.Labels)
	}
	if cm.Labels[labelProfile] != "ollama" {
		t.Errorf("expected profile label, got %v", cm.Labels)
	}
	if cm.Labels[labelManagedBy] != managedByValue {
		t.Errorf("expected managed-by label, got %v", cm.Labels)
	}
	if _, ok := cm.Data[valuesKey]; !ok {
		t.Error("expected values.yaml key in configmap data")
	}

	// Verify profile status was set.
	if len(gc.Status.ProfileStatuses) != 1 {
		t.Fatalf("expected 1 profile status, got %d", len(gc.Status.ProfileStatuses))
	}
	if !gc.Status.ProfileStatuses[0].Ready {
		t.Error("expected profile status Ready=true")
	}
}

func TestReconcileConfigMaps_Update(t *testing.T) {
	scheme := testScheme()

	existing := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-gc-ollama-values",
			Namespace: "test-ns",
			Labels: map[string]string{
				labelGPUCluster: "test-gc",
				labelProfile:    "ollama",
				labelManagedBy:  managedByValue,
			},
		},
		Data: map[string]string{valuesKey: "old: data\n"},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	logger := testLogger()

	r := &GPUClusterReconciler{
		Client:    client,
		Scheme:    scheme,
		Namespace: "test-ns",
		Logger:    logger,
	}

	gc := &v1alpha1.GPUCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-gc"},
		Spec: v1alpha1.GPUClusterSpec{
			ApplicationProfiles: []v1alpha1.ApplicationProfile{
				{
					Name:      "ollama",
					HelmChart: "ollama/ollama",
					Resources: v1alpha1.ProfileResources{GPUCount: 1},
				},
			},
		},
	}

	ctx := context.Background()
	if err := r.reconcileConfigMaps(ctx, logger, gc, testDeviceInfo()); err != nil {
		t.Fatalf("reconcileConfigMaps: %v", err)
	}

	var cm corev1.ConfigMap
	key := types.NamespacedName{Name: "test-gc-ollama-values", Namespace: "test-ns"}
	if err := client.Get(ctx, key, &cm); err != nil {
		t.Fatalf("get configmap: %v", err)
	}

	if cm.Data[valuesKey] == "old: data\n" {
		t.Error("expected configmap data to be updated")
	}
}

func TestReconcileConfigMaps_MultiProfile(t *testing.T) {
	scheme := testScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	logger := testLogger()

	r := &GPUClusterReconciler{
		Client:    client,
		Scheme:    scheme,
		Namespace: "test-ns",
		Logger:    logger,
	}

	gc := &v1alpha1.GPUCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-gc"},
		Spec: v1alpha1.GPUClusterSpec{
			ApplicationProfiles: []v1alpha1.ApplicationProfile{
				{Name: "ollama", HelmChart: "ollama/ollama"},
				{Name: "vllm", HelmChart: "vllm/vllm"},
			},
		},
	}

	ctx := context.Background()
	if err := r.reconcileConfigMaps(ctx, logger, gc, testDeviceInfo()); err != nil {
		t.Fatalf("reconcileConfigMaps: %v", err)
	}

	if len(gc.Status.ProfileStatuses) != 2 {
		t.Fatalf("expected 2 profile statuses, got %d", len(gc.Status.ProfileStatuses))
	}

	for _, name := range []string{"test-gc-ollama-values", "test-gc-vllm-values"} {
		var cm corev1.ConfigMap
		key := types.NamespacedName{Name: name, Namespace: "test-ns"}
		if err := client.Get(ctx, key, &cm); err != nil {
			t.Errorf("expected configmap %s to exist: %v", name, err)
		}
	}
}

func TestReconcileConfigMaps_OrphanCleanup(t *testing.T) {
	scheme := testScheme()

	orphan := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-gc-removed-values",
			Namespace: "test-ns",
			Labels: map[string]string{
				labelGPUCluster: "test-gc",
				labelProfile:    "removed",
				labelManagedBy:  managedByValue,
			},
		},
		Data: map[string]string{valuesKey: "orphan\n"},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(orphan).Build()
	logger := testLogger()

	r := &GPUClusterReconciler{
		Client:    client,
		Scheme:    scheme,
		Namespace: "test-ns",
		Logger:    logger,
	}

	gc := &v1alpha1.GPUCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-gc"},
		Spec: v1alpha1.GPUClusterSpec{
			ApplicationProfiles: []v1alpha1.ApplicationProfile{
				{Name: "ollama", HelmChart: "ollama/ollama"},
			},
		},
	}

	ctx := context.Background()
	if err := r.reconcileConfigMaps(ctx, logger, gc, testDeviceInfo()); err != nil {
		t.Fatalf("reconcileConfigMaps: %v", err)
	}

	// Orphan should be deleted.
	var cm corev1.ConfigMap
	key := types.NamespacedName{Name: "test-gc-removed-values", Namespace: "test-ns"}
	err := client.Get(ctx, key, &cm)
	if err == nil {
		t.Error("expected orphan configmap to be deleted")
	}
}

func TestCheckResourceWarning(t *testing.T) {
	logger := testLogger()

	r := &GPUClusterReconciler{Logger: logger}

	gc := &v1alpha1.GPUCluster{
		Spec: v1alpha1.GPUClusterSpec{
			ApplicationProfiles: []v1alpha1.ApplicationProfile{
				{Name: "big", Resources: v1alpha1.ProfileResources{GPUMemory: "4Gi"}},
				{Name: "also-big", Resources: v1alpha1.ProfileResources{GPUMemory: "3Gi"}},
			},
		},
	}

	info := testDeviceInfo() // 6144 MB VRAM
	r.checkResourceWarning(logger, gc, info)

	if gc.Status.ResourceWarning == "" {
		t.Error("expected resource warning for overcommit")
	}
}

func TestConfigMapName(t *testing.T) {
	got := configMapName("my-cluster", "ollama-inference")
	want := "my-cluster-ollama-inference-values"
	if got != want {
		t.Errorf("configMapName = %q, want %q", got, want)
	}
}

func TestBuildHardwareInfo(t *testing.T) {
	info := testDeviceInfo()
	hw := buildHardwareInfo(info)

	if hw.GPUModel != "NVIDIA GeForce GTX 1660 Ti" {
		t.Errorf("GPUModel = %q", hw.GPUModel)
	}
	if hw.VRAMMB != 6144 {
		t.Errorf("VRAMMB = %d, want 6144", hw.VRAMMB)
	}
	if hw.GPUCount != 1 {
		t.Errorf("GPUCount = %d, want 1", hw.GPUCount)
	}
	if hw.CPUCores != 4 {
		t.Errorf("CPUCores = %d, want 4", hw.CPUCores)
	}
}

func TestBuildHardwareInfo_NoGPU(t *testing.T) {
	info := discovery.DeviceInfo{
		CPU:    discovery.CPUInfo{Cores: 8},
		Memory: discovery.MemoryInfo{TotalMB: 32768},
	}
	hw := buildHardwareInfo(info)

	if hw.GPUModel != "" {
		t.Errorf("expected empty GPU model, got %q", hw.GPUModel)
	}
	if hw.GPUCount != 0 {
		t.Errorf("GPUCount = %d, want 0", hw.GPUCount)
	}
}
