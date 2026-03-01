package v1alpha1

import (
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGPUCluster_JSONRoundTrip(t *testing.T) {
	gc := &GPUCluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "idealab.io/v1alpha1",
			Kind:       "GPUCluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
		Spec: GPUClusterSpec{
			Driver: DriverSpec{
				Enabled: true,
				Version: "560",
			},
			DevicePlugin: DevicePluginSpec{
				Enabled: true,
			},
			GPUFeatureDiscovery: GPUFeatureDiscoverySpec{
				Enabled: true,
			},
		},
		Status: GPUClusterStatus{
			Phase: PhaseReady,
			Node: NodeInfo{
				Hostname: "gaming-pc",
				CPU: CPUNodeInfo{
					Model:    "Intel i5-9300H",
					Cores:    4,
					Threads:  8,
					Features: []string{"AVX2"},
				},
				GPU: GPUNodeInfo{
					Model:             "GTX 1660 Ti",
					VRAMMB:            6144,
					DriverVersion:     "560.35.03",
					CUDAVersion:       "12.6",
					ComputeCapability: "7.5",
				},
				Memory: MemoryNodeInfo{
					TotalMB: 16384,
				},
			},
		},
	}

	data, err := json.Marshal(gc)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded GPUCluster
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Name != "test-cluster" {
		t.Errorf("name: %s", decoded.Name)
	}
	if decoded.Spec.Driver.Version != "560" {
		t.Errorf("driver version: %s", decoded.Spec.Driver.Version)
	}
	if decoded.Status.Phase != PhaseReady {
		t.Errorf("phase: %s", decoded.Status.Phase)
	}
	if decoded.Status.Node.GPU.VRAMMB != 6144 {
		t.Errorf("vram: %d", decoded.Status.Node.GPU.VRAMMB)
	}
}

func TestGPUCluster_DeepCopy(t *testing.T) {
	gc := &GPUCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Status: GPUClusterStatus{
			Phase: PhaseReady,
			Node: NodeInfo{
				CPU: CPUNodeInfo{
					Features: []string{"AVX", "AVX2"},
				},
			},
			Conditions: []metav1.Condition{
				{
					Type:   "Ready",
					Status: metav1.ConditionTrue,
					Reason: "OK",
				},
			},
		},
	}

	copy := gc.DeepCopy()

	if copy.Name != gc.Name {
		t.Error("name mismatch after DeepCopy")
	}

	// Modify the copy — original should be unchanged.
	copy.Status.Phase = PhaseError
	if gc.Status.Phase != PhaseReady {
		t.Error("original phase mutated by copy modification")
	}

	copy.Status.Node.CPU.Features[0] = "CHANGED"
	if gc.Status.Node.CPU.Features[0] != "AVX" {
		t.Error("original features mutated by copy modification")
	}

	copy.Status.Conditions[0].Reason = "CHANGED"
	if gc.Status.Conditions[0].Reason != "OK" {
		t.Error("original conditions mutated by copy modification")
	}
}

func TestGPUClusterList_DeepCopy(t *testing.T) {
	list := &GPUClusterList{
		Items: []GPUCluster{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "item1"},
			},
		},
	}

	copy := list.DeepCopy()
	copy.Items[0].Name = "changed"

	if list.Items[0].Name != "item1" {
		t.Error("original list mutated by copy modification")
	}
}

func TestPhaseValues(t *testing.T) {
	if string(PhasePending) != "Pending" {
		t.Errorf("PhasePending: %s", PhasePending)
	}
	if string(PhaseDiscovering) != "Discovering" {
		t.Errorf("PhaseDiscovering: %s", PhaseDiscovering)
	}
	if string(PhaseReady) != "Ready" {
		t.Errorf("PhaseReady: %s", PhaseReady)
	}
	if string(PhaseError) != "Error" {
		t.Errorf("PhaseError: %s", PhaseError)
	}
}

func TestGroupVersion(t *testing.T) {
	if GroupVersion.Group != "idealab.io" {
		t.Errorf("group: %s", GroupVersion.Group)
	}
	if GroupVersion.Version != "v1alpha1" {
		t.Errorf("version: %s", GroupVersion.Version)
	}
}

func TestGPUCluster_StatusJSON_MatchesCRD(t *testing.T) {
	// Verify the JSON field names match what the CRD YAML expects.
	gc := GPUCluster{
		Status: GPUClusterStatus{
			Phase: PhaseReady,
			Node: NodeInfo{
				GPU: GPUNodeInfo{
					VRAMMB: 6144,
				},
			},
		},
	}

	data, _ := json.Marshal(gc.Status)
	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)

	// CRD expects "phase" field.
	if _, ok := raw["phase"]; !ok {
		t.Error("status JSON missing 'phase' field")
	}
	// CRD expects "node" field.
	if _, ok := raw["node"]; !ok {
		t.Error("status JSON missing 'node' field")
	}

	// Check nested GPU field name.
	var nodeRaw map[string]json.RawMessage
	json.Unmarshal(raw["node"], &nodeRaw)
	if _, ok := nodeRaw["gpu"]; !ok {
		t.Error("node JSON missing 'gpu' field")
	}

	var gpuRaw map[string]json.RawMessage
	json.Unmarshal(nodeRaw["gpu"], &gpuRaw)
	if _, ok := gpuRaw["vramMB"]; !ok {
		t.Error("gpu JSON missing 'vramMB' field")
	}
}

func TestApplicationProfile_DeepCopy(t *testing.T) {
	p := &ApplicationProfile{
		Name:      "test",
		HelmChart: "chart",
		HelmValues: map[string]any{
			"key": "value",
		},
		Resources: ProfileResources{
			GPUCount:    1,
			CPULimit:    "4000m",
			MemoryLimit: "8Gi",
		},
	}

	copy := p.DeepCopy()
	copy.HelmValues["key"] = "changed"

	if p.HelmValues["key"] != "value" {
		t.Error("original helm values mutated by copy")
	}
}
