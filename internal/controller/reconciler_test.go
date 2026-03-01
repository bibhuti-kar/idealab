package controller

import (
	"testing"

	"github.com/bibhuti-kar/idealab/internal/discovery"

	v1alpha1 "github.com/bibhuti-kar/idealab/api/v1alpha1"
)

func TestMapDeviceInfoToNodeInfo(t *testing.T) {
	info := discovery.DeviceInfo{
		Hostname: "test-host",
		CPU: discovery.CPUInfo{
			Model:    "Intel i5-9300H",
			Cores:    4,
			Threads:  8,
			Features: []string{"AVX2"},
		},
		GPUs: []discovery.GPUInfo{
			{
				Model:             "GTX 1660 Ti",
				VRAMMB:            6144,
				DriverVersion:     "560.35.03",
				CUDAVersion:       "12.6",
				ComputeCapability: "7.5",
			},
		},
		Memory: discovery.MemoryInfo{
			TotalMB:     16384,
			AvailableMB: 12288,
		},
	}

	node := mapDeviceInfoToNodeInfo(info)

	if node.Hostname != "test-host" {
		t.Errorf("hostname: got %s, want test-host", node.Hostname)
	}
	if node.CPU.Model != "Intel i5-9300H" {
		t.Errorf("cpu model: got %s", node.CPU.Model)
	}
	if node.CPU.Cores != 4 {
		t.Errorf("cpu cores: got %d, want 4", node.CPU.Cores)
	}
	if node.CPU.Threads != 8 {
		t.Errorf("cpu threads: got %d, want 8", node.CPU.Threads)
	}
	if node.GPU.Model != "GTX 1660 Ti" {
		t.Errorf("gpu model: got %s", node.GPU.Model)
	}
	if node.GPU.VRAMMB != 6144 {
		t.Errorf("gpu vram: got %d, want 6144", node.GPU.VRAMMB)
	}
	if node.GPU.DriverVersion != "560.35.03" {
		t.Errorf("driver version: got %s", node.GPU.DriverVersion)
	}
	if node.GPU.CUDAVersion != "12.6" {
		t.Errorf("cuda version: got %s", node.GPU.CUDAVersion)
	}
	if node.GPU.ComputeCapability != "7.5" {
		t.Errorf("compute capability: got %s", node.GPU.ComputeCapability)
	}
	if node.Memory.TotalMB != 16384 {
		t.Errorf("memory: got %d, want 16384", node.Memory.TotalMB)
	}
}

func TestMapDeviceInfoToNodeInfo_NoGPU(t *testing.T) {
	info := discovery.DeviceInfo{
		Hostname: "no-gpu-host",
		CPU: discovery.CPUInfo{
			Model: "Intel i5-9300H",
			Cores: 4,
		},
		GPUs:   nil,
		Memory: discovery.MemoryInfo{TotalMB: 8192},
	}

	node := mapDeviceInfoToNodeInfo(info)

	if node.GPU.Model != "" {
		t.Errorf("expected empty GPU model, got %s", node.GPU.Model)
	}
	if node.CPU.Model != "Intel i5-9300H" {
		t.Errorf("cpu should still be populated: %s", node.CPU.Model)
	}
}

func TestSetCondition_Add(t *testing.T) {
	gc := &v1alpha1.GPUCluster{}

	setCondition(gc, newCondition("Ready", "True", "TestReason", "test message"))

	if len(gc.Status.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(gc.Status.Conditions))
	}
	if gc.Status.Conditions[0].Type != "Ready" {
		t.Errorf("expected Ready, got %s", gc.Status.Conditions[0].Type)
	}
	if gc.Status.Conditions[0].Reason != "TestReason" {
		t.Errorf("expected TestReason, got %s", gc.Status.Conditions[0].Reason)
	}
}

func TestSetCondition_Update(t *testing.T) {
	gc := &v1alpha1.GPUCluster{}

	setCondition(gc, newCondition("Ready", "False", "NotReady", "waiting"))
	setCondition(gc, newCondition("Ready", "True", "IsReady", "done"))

	if len(gc.Status.Conditions) != 1 {
		t.Fatalf("expected 1 condition after update, got %d", len(gc.Status.Conditions))
	}
	if gc.Status.Conditions[0].Reason != "IsReady" {
		t.Errorf("expected IsReady, got %s", gc.Status.Conditions[0].Reason)
	}
}

func TestSetCondition_MultipleTypes(t *testing.T) {
	gc := &v1alpha1.GPUCluster{}

	setCondition(gc, newCondition("Ready", "True", "OK", ""))
	setCondition(gc, newCondition("Discovering", "False", "Complete", ""))

	if len(gc.Status.Conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(gc.Status.Conditions))
	}
}

func TestGetCondition_Found(t *testing.T) {
	gc := &v1alpha1.GPUCluster{}
	setCondition(gc, newCondition("Ready", "True", "OK", "ready"))

	cond := getCondition(gc, "Ready")
	if cond == nil {
		t.Fatal("expected to find Ready condition")
	}
	if cond.Reason != "OK" {
		t.Errorf("expected reason OK, got %s", cond.Reason)
	}
}

func TestGetCondition_NotFound(t *testing.T) {
	gc := &v1alpha1.GPUCluster{}
	cond := getCondition(gc, "Ready")
	if cond != nil {
		t.Error("expected nil for missing condition")
	}
}

func TestSanitizeLabel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"NVIDIA GeForce GTX 1660 Ti", "NVIDIA-GeForce-GTX-1660-Ti"},
		{"simple", "simple"},
		{"with spaces", "with-spaces"},
		{"special!@#chars", "specialchars"},
	}

	for _, tt := range tests {
		got := sanitizeLabel(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeLabel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeLabel_MaxLength(t *testing.T) {
	long := ""
	for i := 0; i < 100; i++ {
		long += "a"
	}
	got := sanitizeLabel(long)
	if len(got) > 63 {
		t.Errorf("label length %d exceeds 63", len(got))
	}
}

func TestGPULabels(t *testing.T) {
	gpu := discovery.GPUInfo{
		Model:             "NVIDIA GeForce GTX 1660 Ti",
		VRAMMB:            6144,
		DriverVersion:     "560.35.03",
		CUDAVersion:       "12.6",
		ComputeCapability: "7.5",
	}

	labels := gpuLabels(gpu)

	expected := map[string]string{
		"idealab.io/gpu-model":   "NVIDIA-GeForce-GTX-1660-Ti",
		"idealab.io/gpu-vram-mb": "6144",
		"idealab.io/gpu-driver":  "560.35.03",
		"idealab.io/gpu-cuda":    "12.6",
		"idealab.io/gpu-compute": "7.5",
	}

	for k, v := range expected {
		if labels[k] != v {
			t.Errorf("label %s: got %q, want %q", k, labels[k], v)
		}
	}
}

func TestPhaseConstants(t *testing.T) {
	phases := []v1alpha1.Phase{
		v1alpha1.PhasePending,
		v1alpha1.PhaseDiscovering,
		v1alpha1.PhaseReady,
		v1alpha1.PhaseError,
	}

	values := map[v1alpha1.Phase]bool{}
	for _, p := range phases {
		if values[p] {
			t.Errorf("duplicate phase: %s", p)
		}
		values[p] = true
	}
}

// newCondition creates a metav1.Condition for testing.
func newCondition(condType, status, reason, message string) metav1.Condition {
	s := metav1.ConditionTrue
	if status == "False" {
		s = metav1.ConditionFalse
	}
	return metav1.Condition{
		Type:    condType,
		Status:  s,
		Reason:  reason,
		Message: message,
	}
}
