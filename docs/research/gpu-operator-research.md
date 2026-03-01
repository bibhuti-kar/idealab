# GPU Operator Research: Simplified Replica for Single-Node k3s

**Date:** 2026-03-01
**Author:** Product Owner
**Status:** Research Complete
**Target Hardware:** NVIDIA GTX 1660 Ti Mobile (Turing, 6GB GDDR6)

---

## Table of Contents

1. [NVIDIA GPU Operator Architecture](#1-nvidia-gpu-operator-architecture)
2. [Simplified Scope for Our Project](#2-simplified-scope-for-our-project)
3. [k3s Specifics](#3-k3s-specifics)
4. [Key Go Libraries](#4-key-go-libraries)
5. [Reference Implementations](#5-reference-implementations)
6. [Risk Assessment](#6-risk-assessment)
7. [Appendix: Source References](#7-appendix-source-references)

---

## 1. NVIDIA GPU Operator Architecture

### 1.1 What Is the GPU Operator?

The NVIDIA GPU Operator is a Kubernetes operator that automates the management of all
NVIDIA software components needed to provision GPUs in a Kubernetes cluster. It uses the
[Operator Framework](https://operatorframework.io/) to manage the full lifecycle of GPU
software as containerized workloads.

The operator's primary value proposition: administrators can treat GPU nodes like CPU nodes.
Instead of provisioning special OS images for GPU nodes, they use a standard OS image and
let the GPU Operator handle all GPU-specific software provisioning.

Current version as of this research: **v25.10.1** (released February 2026).

### 1.2 Core Components (Full Operator)

The GPU Operator manages the following components, each deployed as a DaemonSet or Pod:

| Component | Image/Container | Purpose |
|-----------|----------------|---------|
| **NVIDIA GPU Driver** | `nvcr.io/nvidia/driver` | Deploys NVIDIA drivers as a container (compiles kernel modules on the node). Can be disabled if drivers are pre-installed on the host. |
| **NVIDIA Container Toolkit** | `nvidia-container-toolkit` | Configures the container runtime (containerd/CRI-O) to expose GPUs to containers. Installs the `nvidia-container-runtime` hook and libnvidia-container. |
| **NVIDIA Kubernetes Device Plugin** | `k8s-device-plugin` | Implements the Kubernetes Device Plugin API. Exposes `nvidia.com/gpu` as a schedulable resource. Monitors GPU health. |
| **GPU Feature Discovery (GFD)** | `gpu-feature-discovery` | Labels nodes with GPU properties (model, driver version, CUDA version, memory, MIG capabilities). Used by the scheduler for GPU-aware placement. |
| **DCGM Exporter** | `dcgm-exporter` | Exports GPU telemetry metrics (temperature, utilization, memory, ECC errors, power) in Prometheus format using NVIDIA DCGM. |
| **NVIDIA DCGM** | `dcgm` | Data Center GPU Manager -- the underlying engine for GPU health monitoring and diagnostics. |
| **MIG Manager** | `mig-manager` | Manages Multi-Instance GPU (MIG) configuration on supported GPUs (A100, H100). Watches for MIG geometry changes and reconfigures GPUs. |
| **NVIDIA Driver Manager** | `driver-manager` | Manages the lifecycle of containerized GPU drivers, including upgrades and pre-install detection. |
| **Validator** | `gpu-operator-validator` | Validates that all GPU Operator components are correctly deployed and functional. Runs validation checks on driver, toolkit, device plugin, and CUDA workloads. |
| **Node Feature Discovery (NFD)** | `nfd-master` + `nfd-worker` | Detects hardware features on nodes (PCI devices, CPU features). GPU nodes are identified by PCI vendor ID `0x10de` (NVIDIA). Deployed by the operator by default but can use an existing NFD installation. |
| **Confidential Computing Manager** | `cc-manager` | Manages confidential computing features for GPUs (optional, disabled by default). |
| **KubeVirt GPU Device Plugin** | `kubevirt-gpu-device-plugin` | Exposes GPUs for virtual machine workloads via KubeVirt (optional). |
| **vGPU Device Manager** | `vgpu-device-manager` | Manages NVIDIA vGPU devices (optional, for virtualized GPU sharing). |
| **GDS Driver** | `gds-driver` | GPU Direct Storage driver for direct data paths between storage and GPU memory (optional). |
| **GDRCopy Driver** | `gdrcopy-driver` | Low-latency GPU memory copy library (optional, runs as sidecar in driver pod). |
| **Kata Manager** | `kata-manager` | Manages Kata Containers integration for sandboxed GPU workloads (optional). |

### 1.3 CRDs and Configuration

The operator defines the following Custom Resource Definitions:

- **ClusterPolicy** -- The primary CRD. A cluster-wide singleton that defines the desired state for all GPU operator components. Specifies image versions, feature flags, and configuration for every managed component.
- **NVIDIADriver** -- A newer CRD (promoted toward GA) that allows per-node-group driver configuration, enabling different driver versions on different node groups.

### 1.4 Architecture Flow

```
ClusterPolicy CR
       |
       v
GPU Operator Controller (Deployment)
       |
       +---> Node Feature Discovery (DaemonSet)
       |         |
       |         v  labels nodes with feature.node.kubernetes.io/pci-10de.present=true
       |
       +---> Driver Container (DaemonSet) -- compiles/loads kernel modules
       |         |
       |         v  nvidia-smi available, /dev/nvidia* devices present
       |
       +---> Container Toolkit (DaemonSet) -- configures containerd/CRI-O
       |         |
       |         v  nvidia runtime available in container engine
       |
       +---> Device Plugin (DaemonSet) -- registers nvidia.com/gpu resource
       |         |
       |         v  kubelet can schedule GPU workloads
       |
       +---> GPU Feature Discovery (DaemonSet) -- labels nodes with GPU details
       |
       +---> DCGM Exporter (DaemonSet) -- GPU metrics for Prometheus
       |
       +---> MIG Manager (DaemonSet) -- MIG reconfiguration (if supported)
       |
       +---> Validator (DaemonSet) -- validates all components working
```

### 1.5 How the Device Plugin Works (Key for Our Project)

The NVIDIA k8s-device-plugin is the critical component that bridges NVIDIA GPUs to
Kubernetes scheduling. It implements the Kubernetes Device Plugin API:

1. **Discovery:** Uses NVML (NVIDIA Management Library) to enumerate all GPUs on the node.
2. **Registration:** Registers with the kubelet via a Unix socket at `/var/lib/kubelet/device-plugins/`.
3. **Resource Advertisement:** Advertises `nvidia.com/gpu` resources to the Kubernetes API. Each GPU is identified by UUID.
4. **Allocation:** When a pod requests `nvidia.com/gpu: 1`, the kubelet calls the device plugin's `Allocate()` RPC. The plugin returns the device spec (device files, environment variables) needed to give the container access.
5. **Health Monitoring:** Periodically checks GPU health via NVML and reports unhealthy devices to kubelet.

Configuration options for the device plugin:
- `MIG_STRATEGY`: none | single | mixed (for Multi-Instance GPU support)
- `FAIL_ON_INIT_ERROR`: true | false (whether to fail if no GPUs found)
- `DEVICE_LIST_STRATEGY`: envvar | volume-mounts | cdi-annotations | cdi-cri
- `DEVICE_ID_STRATEGY`: uuid | index
- `NVIDIA_DRIVER_ROOT`: path to driver installation root

### 1.6 GPU Sharing Mechanisms

The device plugin supports two GPU sharing methods:

**CUDA Time-Slicing:**
- Multiple workloads share a GPU via CUDA time-slicing
- No memory isolation -- workloads share fault domain
- Configured via `sharing.timeSlicing.resources[].replicas` in device plugin config
- Example: 1 physical GPU can be advertised as 10 `nvidia.com/gpu` resources

**CUDA MPS (Multi-Process Service):**
- Uses a control daemon for space partitioning
- Memory and compute resources can be explicitly partitioned
- Better isolation than time-slicing
- Mutually exclusive with time-slicing

### 1.7 Container Device Interface (CDI)

As of GPU Operator v25.10.0, CDI is enabled by default (`cdi.enabled=true`). CDI is a
standardized mechanism for exposing complex devices to containers. Instead of relying on
the `nvidia-container-runtime` hook, CDI uses native container runtime support in
containerd/CRI-O to inject GPU devices.

---

## 2. Simplified Scope for Our Project

### 2.1 What We Actually Need

For a single-node gaming PC (GTX 1660 Ti Mobile) running k3s, we can dramatically
simplify the architecture. Many GPU Operator components exist for multi-node,
data-center, or enterprise scenarios that do not apply to us.

### 2.2 Pre-Install Requirements (Host-Level, Before Our Operator)

These components must be installed directly on the host before our operator runs:

| Component | Why Host-Level | Installation |
|-----------|---------------|-------------|
| **NVIDIA GPU Drivers** | Kernel modules cannot be safely containerized on a gaming PC with an active display. The host drivers manage the display output. | `sudo apt install nvidia-driver-560` or equivalent |
| **NVIDIA Container Toolkit** | Configures containerd (k3s's runtime) to support GPU passthrough to containers. | `nvidia-ctk runtime configure --runtime=containerd` |
| **Go 1.22+** | Our operator is built in Go. | Standard Go installation |
| **k3s** | Lightweight Kubernetes distribution. | `curl -sfL https://get.k3s.io \| sh -` |
| **Helm** | For deploying our operator's chart. | Standard Helm 3 installation |

### 2.3 Our Operator Scope (What We Build)

| Component | Description | GPU Operator Equivalent |
|-----------|-------------|----------------------|
| **Device Enumeration** | Use NVML to discover GTX 1660 Ti, report UUID, memory, driver version, CUDA version | GPU Feature Discovery + Device Plugin discovery |
| **Resource Registration** | Implement Kubernetes Device Plugin API to register `nvidia.com/gpu` with kubelet | NVIDIA Device Plugin (core) |
| **Health Monitoring** | Periodic GPU health checks via NVML (temperature, utilization, errors) | Device Plugin health check + DCGM (simplified) |
| **Node Labeling** | Label the k3s node with GPU metadata (model, memory, driver version, CUDA compute capability) | GPU Feature Discovery |
| **Metrics Export** | Optional: export basic GPU metrics (temp, utilization, memory usage) for Prometheus | DCGM Exporter (simplified) |
| **Helm Chart** | Package the operator as a Helm chart for easy deployment | GPU Operator Helm chart |
| **CRD (Optional)** | A simplified GpuConfig CRD for single-node configuration | ClusterPolicy (simplified) |

### 2.4 What We Explicitly Skip

| Component | Reason for Skipping |
|-----------|-------------------|
| Driver Container | Drivers pre-installed on host (gaming PC needs display output) |
| Container Toolkit Container | Pre-installed on host |
| NFD (Node Feature Discovery) | Single node; we label it directly |
| MIG Manager | GTX 1660 Ti does not support Multi-Instance GPU |
| vGPU Manager | Not applicable for consumer GPUs |
| Confidential Computing | Enterprise/data-center feature |
| KubeVirt Plugin | Not running VMs |
| GDS Driver | No GPUDirect Storage on consumer hardware |
| GDRCopy | Data-center optimization |
| Kata Manager | No sandboxed VM workloads |
| Driver Manager | No containerized drivers to manage |
| Complex time-slicing/MPS | Can be a future enhancement, not MVP |

### 2.5 Architecture Diagram (Our Simplified Operator)

```
Host (Gaming PC - Ubuntu + GTX 1660 Ti)
  |
  +-- NVIDIA Driver (host-installed, e.g., 560.x)
  +-- NVIDIA Container Toolkit (host-installed, configures containerd)
  +-- k3s (containerd runtime)
       |
       +-- gpu-operator namespace
            |
            +-- GPU Operator Controller (Deployment, 1 replica)
            |     |
            |     +-- Watches GpuConfig CR
            |     +-- Reconciles desired state
            |
            +-- Device Plugin (DaemonSet, 1 pod on single node)
            |     |
            |     +-- NVML: enumerate GPUs
            |     +-- Register nvidia.com/gpu with kubelet
            |     +-- Allocate GPUs to requesting pods
            |     +-- Health monitoring loop
            |
            +-- GPU Labeler (Job or init container)
            |     |
            |     +-- NVML: read GPU properties
            |     +-- kubectl label node with GPU metadata
            |
            +-- Metrics Exporter (DaemonSet, optional)
                  |
                  +-- NVML: read temp, utilization, memory
                  +-- Expose /metrics endpoint for Prometheus
```

---

## 3. k3s Specifics

### 3.1 k3s vs Full Kubernetes for GPU Workloads

| Aspect | Full Kubernetes (k8s) | k3s |
|--------|----------------------|-----|
| **Container Runtime** | containerd or CRI-O (configurable) | containerd (embedded, default) |
| **Kubelet** | Standalone binary | Embedded in k3s agent |
| **Device Plugin Socket** | `/var/lib/kubelet/device-plugins/` | Same path (k3s is API-compatible) |
| **Helm** | Separate installation | k3s includes HelmChart CRD for auto-deploy |
| **NFD** | Commonly deployed | Not included; must deploy separately or handle in operator |
| **etcd** | External etcd cluster | Embedded SQLite (single node) or embedded etcd |
| **Resource Overhead** | ~2GB RAM minimum | ~512MB RAM minimum |
| **GPU Operator Support** | Officially supported | Works but not officially tested by NVIDIA for all scenarios |

### 3.2 k3s-Specific Configuration for GPU Support

**containerd configuration path in k3s:**
k3s uses its own containerd configuration at `/var/lib/rancher/k3s/agent/etc/containerd/config.toml`.
The NVIDIA Container Toolkit must be configured to modify this path specifically:

```bash
sudo nvidia-ctk runtime configure --runtime=containerd \
    --config=/var/lib/rancher/k3s/agent/etc/containerd/config.toml
sudo systemctl restart k3s
```

**RuntimeClass for k3s:**
After configuring containerd, a RuntimeClass must be created:

```yaml
apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: nvidia
handler: nvidia
```

Pods that need GPU access must reference this RuntimeClass unless the nvidia runtime is
set as the default handler.

**k3s HelmChart CRD:**
k3s can auto-deploy Helm charts placed in `/var/lib/rancher/k3s/server/manifests/`.
Our operator could leverage this for zero-touch deployment:

```yaml
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: gpu-operator
  namespace: kube-system
spec:
  chart: gpu-operator
  repo: https://our-helm-repo
  targetNamespace: gpu-operator
```

### 3.3 Device Plugin Path Compatibility

The device plugin socket path is identical in k3s and standard Kubernetes:
`/var/lib/kubelet/device-plugins/`. k3s symlinks this for compatibility.

The kubelet gRPC interface for device plugins is the same. Our device plugin implementation
will work identically on k3s and full Kubernetes.

### 3.4 k3s Networking and Pod Security

k3s uses Flannel as the default CNI. For GPU workloads, no special networking configuration
is needed (GPU communication is local to the node, not over the network).

k3s supports Pod Security Admission (PSA). The GPU operator namespace may need privileged
access for the device plugin pod (which needs access to `/dev/nvidia*` device files):

```bash
kubectl label ns gpu-operator pod-security.kubernetes.io/enforce=privileged
```

---

## 4. Key Go Libraries

### 4.1 Kubernetes Operator Development

| Library | Import Path | Purpose | Used By |
|---------|------------|---------|---------|
| **controller-runtime** | `sigs.k8s.io/controller-runtime` | High-level framework for building Kubernetes operators. Provides Manager, Controller, Reconciler abstractions. | NVIDIA GPU Operator itself |
| **client-go** | `k8s.io/client-go` | Low-level Kubernetes API client. For direct API calls, watches, informers. | All K8s tools |
| **apimachinery** | `k8s.io/apimachinery` | Kubernetes API object types, serialization, scheme registration. | All K8s tools |
| **api** | `k8s.io/api` | Core Kubernetes API types (Pod, Node, DaemonSet, etc.). | All K8s tools |
| **kubebuilder** | CLI tool (not a library) | Scaffolds operator project structure, generates CRD manifests, RBAC, webhooks. | Project setup |
| **operator-sdk** | CLI tool | Alternative to kubebuilder with additional lifecycle management features. | Alternative to kubebuilder |

**Recommendation:** Use **kubebuilder** to scaffold the project and **controller-runtime** for
the operator logic. This is the same approach used by the official NVIDIA GPU Operator.

### 4.2 NVIDIA GPU Interaction

| Library | Import Path | Purpose | Notes |
|---------|------------|---------|-------|
| **go-nvml** | `github.com/NVIDIA/go-nvml/pkg/nvml` | Go bindings for NVIDIA Management Library (NVML). Enumerate GPUs, query properties, monitor health/temperature/utilization. | Official NVIDIA library. This is the primary library for GPU interaction. Uses cgo to call libnvidia-ml.so. |
| **go-nvlib** | `github.com/NVIDIA/go-nvlib` | Higher-level NVIDIA library wrappers built on go-nvml. Provides device info, MIG support, and resource management abstractions. | Used internally by NVIDIA's k8s-device-plugin. |
| **nvidia-container-toolkit** (Go packages) | `github.com/NVIDIA/nvidia-container-toolkit/pkg/...` | CDI spec generation, container runtime configuration, device discovery. | Used if we need CDI integration. |

**Key go-nvml operations we will use:**

```go
// Initialize NVML
nvml.Init()
defer nvml.Shutdown()

// Get device count
count, ret := nvml.DeviceGetCount()

// Get device handle
device, ret := nvml.DeviceGetHandleByIndex(i)

// Get device properties
name, _ := device.GetName()                    // "NVIDIA GeForce GTX 1660 Ti"
uuid, _ := device.GetUUID()                    // "GPU-xxxxxxxx-xxxx-xxxx-xxxx"
memory, _ := device.GetMemoryInfo()            // Total: 6GB
temp, _ := device.GetTemperature(nvml.TEMPERATURE_GPU)
util, _ := device.GetUtilizationRates()        // GPU %, Memory %
power, _ := device.GetPowerUsage()             // milliwatts
driverVersion, _ := nvml.SystemGetDriverVersion()
cudaVersion, _ := nvml.SystemGetCudaDriverVersion()
```

### 4.3 Hardware Enumeration (Supplementary)

| Library | Import Path | Purpose |
|---------|------------|---------|
| **ghw** | `github.com/jaypipes/ghw` | Hardware discovery: PCI devices, GPU info, memory, CPU, topology. Can discover NVIDIA GPUs via PCI vendor/device IDs without NVML. |
| **gopsutil** | `github.com/shirou/gopsutil/v4` | System metrics: CPU, memory, disk, network, process info. Not GPU-specific but useful for host-level metrics alongside GPU metrics. |

**Note:** For our use case, **go-nvml is the primary library**. ghw and gopsutil are
supplementary for host-level context but not essential for the MVP.

### 4.4 Kubernetes Device Plugin API

The device plugin API is defined as a gRPC service. The key interface our plugin must
implement:

```protobuf
service DevicePlugin {
    rpc GetDevicePluginOptions(Empty) returns (DevicePluginOptions) {}
    rpc ListAndWatch(Empty) returns (stream ListAndWatchResponse) {}
    rpc GetPreferredAllocation(PreferredAllocationRequest) returns (PreferredAllocationResponse) {}
    rpc Allocate(AllocateRequest) returns (AllocateResponse) {}
    rpc PreStartContainer(PreStartContainerRequest) returns (PreStartContainerResponse) {}
}
```

The proto definitions are at:
`k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1`

Key implementation steps:
1. Create a gRPC server listening on a Unix socket in `/var/lib/kubelet/device-plugins/`
2. Register with kubelet at `/var/lib/kubelet/device-plugins/kubelet.sock`
3. Implement `ListAndWatch` to stream the list of available GPU devices
4. Implement `Allocate` to return device specs when a GPU is assigned to a pod
5. Handle kubelet restarts by re-registering

---

## 5. Reference Implementations

### 5.1 Primary References

| Repository | URL | Language | Relevance |
|-----------|-----|----------|-----------|
| **NVIDIA GPU Operator** | https://github.com/NVIDIA/gpu-operator | Go | Full operator source. Uses controller-runtime, kubebuilder. Our architectural model. |
| **NVIDIA k8s-device-plugin** | https://github.com/NVIDIA/k8s-device-plugin | Go | The actual device plugin implementation. This is the most important reference for our core functionality. |
| **NVIDIA go-nvml** | https://github.com/NVIDIA/go-nvml | Go | Official Go bindings for NVML. Our primary GPU interaction library. |
| **NVIDIA go-nvlib** | https://github.com/NVIDIA/go-nvlib | Go | Higher-level wrappers used by the device plugin. |
| **NVIDIA GPU Feature Discovery** | https://github.com/NVIDIA/gpu-feature-discovery | Go | Node labeling logic. Now merged into k8s-device-plugin repo (as of v0.15.0). |
| **NVIDIA Container Toolkit** | https://github.com/NVIDIA/nvidia-container-toolkit | Go | CDI spec generation, runtime configuration. |
| **NVIDIA DCGM Exporter** | https://github.com/NVIDIA/dcgm-exporter | Go | GPU metrics exporter for Prometheus. Reference for our simplified metrics. |

### 5.2 k8s-device-plugin Source Code Structure

The NVIDIA k8s-device-plugin repository (the most relevant reference) is structured as:

```
k8s-device-plugin/
  cmd/
    nvidia-device-plugin/      # Main entrypoint
  internal/
    plugin/                    # Core device plugin logic
    rm/                        # Resource manager
    lm/                        # Label manager (GFD)
    cdi/                       # CDI support
    flags/                     # CLI flag handling
    info/                      # Version info
  api/
    config/                    # Configuration types
  pkg/                         # Exported packages
  deployments/
    static/                    # Static K8s manifests
    helm/                      # Helm chart
      nvidia-device-plugin/
  docs/
    gpu-feature-discovery/     # GFD documentation
```

Key source files to study:
- Device plugin server: `internal/plugin/server.go`
- GPU device discovery via NVML: `internal/rm/nvml.go`
- GPU Feature Discovery labels: `internal/lm/nvml.go`
- Configuration parsing: `api/config/v1/config.go`
- Main entry point: `cmd/nvidia-device-plugin/main.go`

### 5.3 GPU Operator Source Code Structure

```
gpu-operator/
  api/
    nvidia.com/v1/
      clusterpolicy_types.go   # ClusterPolicy CRD type definitions
  controllers/
    clusterpolicy_controller.go  # Main reconciliation loop
    state_manager.go             # Manages component states
  internal/
  assets/                       # Embedded K8s manifests for operands
  deployments/
    gpu-operator/                # Helm chart
  hack/
  config/
    crd/                         # Generated CRD YAML
    rbac/                        # RBAC templates
```

### 5.4 Simplified Third-Party References

| Repository | URL | Relevance |
|-----------|-----|-----------|
| **k8s-device-plugin sample** | https://github.com/kubernetes/kubernetes/tree/master/pkg/kubelet/cm/devicemanager | Kubernetes source for the device manager (kubelet side). Useful for understanding the gRPC protocol. |
| **virtual-kubelet** | https://github.com/virtual-kubelet/virtual-kubelet | Demonstrates custom kubelet implementations (different use case but shows device API patterns). |
| **smarter-device-manager** | https://gitlab.com/arm-research/smarter/smarter-device-manager | Simple Go device plugin for generic Linux devices. Good minimal reference for the device plugin API pattern without NVIDIA-specific complexity. |

---

## 6. Risk Assessment

### 6.1 Consumer GPU (GTX 1660 Ti) vs Data Center GPUs

| Feature | GTX 1660 Ti Mobile | Data Center (A100/H100) | Impact on Our Project |
|---------|-------------------|------------------------|----------------------|
| **Architecture** | Turing (TU116) | Ampere/Hopper | Turing is well-supported by NVML and drivers. No issues. |
| **VRAM** | 6 GB GDDR6 | 40-80 GB HBM2e/HBM3 | Limits the size of models/datasets. Must document VRAM constraints. |
| **CUDA Compute Capability** | 7.5 | 8.0 / 9.0 | Sufficient for most CUDA workloads. Some newer features (BF16, Transformer Engine) unavailable. |
| **MIG Support** | Not supported | Supported (A100+) | We skip MIG entirely. Simplifies our operator. |
| **ECC Memory** | Not available | Available | Cannot report/monitor ECC errors. Some NVML health queries will return "not supported." |
| **NVLink** | Not available | Available | Not relevant for single-GPU, single-node. |
| **Power Management** | Mobile power profiles (80W TDP) | 300-700W TDP | NVML power queries work but mobile GPUs have aggressive thermal throttling. |
| **DCGM Support** | Limited | Full | DCGM (Data Center GPU Manager) has limited support for consumer GPUs. Some metrics may not be available. We should use NVML directly instead of DCGM. |
| **Driver Branch** | Game Ready / Production | Data Center (e.g., 535.x, 550.x, 570.x) | Consumer drivers work fine. The GPU Operator's containerized driver approach does not work well for consumer GPUs with active displays. Host-installed drivers are the correct approach. |
| **vGPU** | Not supported | Supported | Irrelevant; we skip this. |
| **Time-Slicing** | Supported (CUDA-level) | Supported | Works the same way, but with only 6GB VRAM, sharing is limited. |
| **Persistent Mode** | Available via nvidia-smi | Default in data center | Should be enabled for consistent device plugin behavior: `nvidia-smi -pm 1` |

### 6.2 NVML API Availability on Consumer GPUs

Not all NVML functions return data on consumer GPUs. Known limitations:

| NVML Function | Works on GTX 1660 Ti? | Notes |
|--------------|----------------------|-------|
| `DeviceGetCount` | Yes | Core enumeration |
| `DeviceGetName` | Yes | Returns "NVIDIA GeForce GTX 1660 Ti" |
| `DeviceGetUUID` | Yes | Returns GPU UUID |
| `DeviceGetMemoryInfo` | Yes | Returns total/free/used VRAM |
| `DeviceGetTemperature` | Yes | GPU core temperature |
| `DeviceGetUtilizationRates` | Yes | GPU and memory utilization % |
| `DeviceGetPowerUsage` | Yes | Current power draw in mW |
| `DeviceGetClockInfo` | Yes | Current/max clock speeds |
| `DeviceGetFanSpeed` | Varies | May not work on laptop (no user-accessible fan) |
| `DeviceGetEccMode` | Returns NOT_SUPPORTED | Consumer GPU, no ECC |
| `DeviceGetMigMode` | Returns NOT_SUPPORTED | Turing does not support MIG |
| `DeviceGetNvLinkState` | Returns NOT_SUPPORTED | No NVLink on consumer GPU |
| `DeviceGetPersistenceMode` | Yes | Requires nvidia-persistenced or `nvidia-smi -pm 1` |
| `DeviceGetComputeRunningProcesses` | Yes | Lists CUDA processes using the GPU |

**Mitigation:** Wrap all NVML calls with error handling that gracefully degrades when
a function returns `NVML_ERROR_NOT_SUPPORTED`. Report available metrics only.

### 6.3 Display GPU Sharing Risk

The GTX 1660 Ti is also the display GPU. When a container workload uses the GPU:
- Desktop compositing may slow down
- OOM conditions in GPU memory can crash the display
- Long-running CUDA kernels can cause display hangs (kernel timeout mechanism, `nvidia-smi --gpu-reset` may be needed)

**Mitigations:**
- Reserve some GPU memory headroom (e.g., 512MB) for display
- Set CUDA_VISIBLE_DEVICES in container to the same GPU (only 1 GPU available)
- Monitor GPU memory usage and alert before OOM
- Consider setting `nvidia-smi -e 0` to disable ECC (already off on consumer) and `nvidia-smi -pm 1` for persistence mode

### 6.4 Technical Risks

| Risk | Severity | Likelihood | Mitigation |
|------|----------|------------|------------|
| NVML cgo linking fails on build | Medium | Low | Use the official NVIDIA go-nvml package which handles linking. Build in Docker with CUDA base image. |
| k3s containerd config path changes between versions | Low | Low | Pin k3s version, document the config path. |
| Device plugin registration fails after k3s restart | Medium | Medium | Implement kubelet restart detection and re-registration (standard pattern in reference implementation). |
| GPU thermal throttling causes performance variability | Low | High | Expected on mobile GPU. Document as known behavior. Not a software issue. |
| Host driver version mismatch with NVML library version | Medium | Low | Build against the NVML stub library, load at runtime. go-nvml handles this via dlopen. |
| Container workload crashes display | High | Medium | Document as known limitation. Future: explore headless GPU passthrough or separate display GPU. |
| Only 6GB VRAM limits useful workloads | Medium | High | Document VRAM requirements for common ML frameworks. This is a hardware constraint, not a software issue. |

### 6.5 Scope Confidence Assessment

| Aspect | Confidence | Notes |
|--------|-----------|-------|
| Device Plugin API implementation | High | Well-documented Kubernetes API, multiple reference implementations, stable since K8s 1.10 |
| NVML Go bindings | High | Official NVIDIA library, actively maintained, used in production |
| k3s compatibility | High | k3s is fully Kubernetes API compatible for device plugins |
| Consumer GPU NVML support | Medium-High | Core functions work, but must handle NOT_SUPPORTED gracefully |
| Helm chart packaging | High | Standard practice, well-documented |
| Operator pattern (controller-runtime) | High | Mature framework, used by GPU Operator itself |

---

## 7. Appendix: Source References

### Documentation
- NVIDIA GPU Operator Documentation: https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/overview.html
- NVIDIA GPU Operator Installation Guide: https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/getting-started.html
- NVIDIA Container Toolkit Installation: https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html
- Kubernetes Device Plugin Framework: https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/

### Source Code Repositories
- NVIDIA GPU Operator: https://github.com/NVIDIA/gpu-operator (Apache 2.0)
- NVIDIA k8s-device-plugin: https://github.com/NVIDIA/k8s-device-plugin (Apache 2.0)
- NVIDIA go-nvml: https://github.com/NVIDIA/go-nvml (Apache 2.0)
- NVIDIA go-nvlib: https://github.com/NVIDIA/go-nvlib (Apache 2.0)
- NVIDIA Container Toolkit: https://github.com/NVIDIA/nvidia-container-toolkit (Apache 2.0)
- NVIDIA DCGM Exporter: https://github.com/NVIDIA/dcgm-exporter (Apache 2.0)
- NVIDIA GPU Feature Discovery: https://github.com/NVIDIA/gpu-feature-discovery (merged into k8s-device-plugin)

### Go Libraries
- controller-runtime: https://github.com/kubernetes-sigs/controller-runtime
- client-go: https://github.com/kubernetes/client-go
- kubebuilder: https://github.com/kubernetes-sigs/kubebuilder
- ghw: https://github.com/jaypipes/ghw
- gopsutil: https://github.com/shirou/gopsutil

### k3s
- k3s Documentation: https://docs.k3s.io/
- k3s GPU Support: https://docs.k3s.io/advanced#nvidia-container-runtime-support

### GTX 1660 Ti Specifications
- Architecture: Turing (TU116)
- CUDA Cores: 1536
- VRAM: 6GB GDDR6 (192-bit bus)
- CUDA Compute Capability: 7.5
- TDP: 80W (Mobile)
- Driver support: R450+ (consumer branch)
- NVML support: Full (excluding ECC, MIG, NVLink)
