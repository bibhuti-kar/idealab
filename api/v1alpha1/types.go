package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GPUCluster is the Schema for the gpuclusters API.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="GPU",type=string,JSONPath=`.status.node.gpu.model`
// +kubebuilder:printcolumn:name="VRAM",type=integer,JSONPath=`.status.node.gpu.vramMB`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type GPUCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GPUClusterSpec   `json:"spec,omitempty"`
	Status GPUClusterStatus `json:"status,omitempty"`
}

// GPUClusterSpec defines the desired state of GPUCluster.
type GPUClusterSpec struct {
	Driver              DriverSpec              `json:"driver,omitempty"`
	DevicePlugin        DevicePluginSpec        `json:"devicePlugin,omitempty"`
	GPUFeatureDiscovery GPUFeatureDiscoverySpec `json:"gpuFeatureDiscovery,omitempty"`
	ApplicationProfiles []ApplicationProfile    `json:"applicationProfiles,omitempty"`
}

// DriverSpec configures the GPU driver settings.
type DriverSpec struct {
	Enabled bool   `json:"enabled,omitempty"`
	Version string `json:"version,omitempty"`
}

// DevicePluginSpec configures the device plugin.
type DevicePluginSpec struct {
	Enabled bool `json:"enabled,omitempty"`
}

// GPUFeatureDiscoverySpec configures GPU feature discovery.
type GPUFeatureDiscoverySpec struct {
	Enabled bool `json:"enabled,omitempty"`
}

// ApplicationProfile defines a workload profile with resource requirements.
type ApplicationProfile struct {
	Name      string           `json:"name"`
	HelmChart string           `json:"helmChart,omitempty"`
	HelmValues map[string]any  `json:"helmValues,omitempty"`
	Resources ProfileResources `json:"resources,omitempty"`
}

// ProfileResources defines resource requirements for an application profile.
type ProfileResources struct {
	GPUCount    int    `json:"gpuCount,omitempty"`
	GPUMemory   string `json:"gpuMemory,omitempty"`
	CPULimit    string `json:"cpuLimit,omitempty"`
	MemoryLimit string `json:"memoryLimit,omitempty"`
}

// GPUClusterStatus defines the observed state of GPUCluster.
type GPUClusterStatus struct {
	Phase      Phase              `json:"phase,omitempty"`
	Node       NodeInfo           `json:"node,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Phase represents the lifecycle phase of a GPUCluster.
type Phase string

const (
	PhasePending     Phase = "Pending"
	PhaseDiscovering Phase = "Discovering"
	PhaseReady       Phase = "Ready"
	PhaseError       Phase = "Error"
)

// NodeInfo holds discovered hardware information for the node.
type NodeInfo struct {
	Hostname string         `json:"hostname,omitempty"`
	CPU      CPUNodeInfo    `json:"cpu,omitempty"`
	GPU      GPUNodeInfo    `json:"gpu,omitempty"`
	Memory   MemoryNodeInfo `json:"memory,omitempty"`
}

// CPUNodeInfo holds CPU details in the CRD status.
type CPUNodeInfo struct {
	Model   string   `json:"model,omitempty"`
	Cores   int      `json:"cores,omitempty"`
	Threads int      `json:"threads,omitempty"`
	Features []string `json:"features,omitempty"`
}

// GPUNodeInfo holds GPU details in the CRD status.
type GPUNodeInfo struct {
	Model             string `json:"model,omitempty"`
	VRAMMB            int    `json:"vramMB,omitempty"`
	DriverVersion     string `json:"driverVersion,omitempty"`
	CUDAVersion       string `json:"cudaVersion,omitempty"`
	ComputeCapability string `json:"computeCapability,omitempty"`
}

// MemoryNodeInfo holds system memory details in the CRD status.
type MemoryNodeInfo struct {
	TotalMB int `json:"totalMB,omitempty"`
}

// GPUClusterList contains a list of GPUCluster resources.
// +kubebuilder:object:root=true
type GPUClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GPUCluster `json:"items"`
}
