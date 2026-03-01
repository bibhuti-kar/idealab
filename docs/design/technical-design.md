# Technical Design Document: idealab GPU Operator

**Version:** 2.0
**Date:** 2026-03-01
**Author:** Engineer
**Status:** Updated
**JIRA Key:** IDEAL
**Sprint:** 1 (E1 + E2 + E3) + 2 (E4 + E5)

---

## 1. Overview

### 1.1 Project

idealab is a simplified GPU Operator for k3s single-node edge deployments. It
targets a solo developer running AI workloads on a gaming PC with a consumer
NVIDIA GPU (GTX 1660 Ti Mobile, Turing architecture, 6GB VRAM).

### 1.2 Scope

**Sprint 1** covers three P0 epics:

| Epic | Scope | Deliverables |
|------|-------|-------------|
| E1: Pre-Install Script | Host-level prerequisite installation | `scripts/preinstall.sh`, `cmd/preinstall` Go wrapper |
| E2: Device Discovery | CPU, GPU, and memory enumeration | `internal/discovery` package |
| E3: Operator Core | CRD, controller, health endpoints | `api/v1alpha1`, `internal/controller`, `internal/health`, `cmd/operator` |

**Sprint 2** covers two additional epics:

| Epic | Scope | Deliverables |
|------|-------|-------------|
| E4: Config Templates | Helm values generation from profiles + hardware | `internal/config` package, ConfigMap reconciliation, finalizer |
| E5: Monitoring | Prometheus metrics for GPU telemetry + reconciliation | `internal/metrics` package, controller metrics helpers |

### 1.3 Key Constraints

- Single node, single GPU, single operator replica (no leader election).
- Consumer GPU: NVML calls for ECC, MIG, NVLink return NOT_SUPPORTED. All NVML
  interactions must handle graceful degradation.
- Display GPU sharing: the GTX 1660 Ti drives the desktop compositor. 512MB VRAM
  headroom is reserved for display stability.
- k3s containerd config path: `/var/lib/rancher/k3s/agent/etc/containerd/config.toml`.
- CRD API group: `idealab.io`, version `v1alpha1`.

---

## 2. Architecture

### 2.1 System Components

```
Host (Ubuntu 22.04/24.04 + GTX 1660 Ti)
  |
  +-- NVIDIA Driver (host-installed, >= 560)
  +-- NVIDIA Container Toolkit (host-installed, configures containerd)
  +-- k3s (embedded containerd)
       |
       +-- idealab-system namespace
            |
            +-- idealab-operator Deployment (1 replica)
            |     |
            |     +-- cmd/operator binary
            |     +-- internal/controller (watches GPUCluster CRD)
            |     +-- internal/discovery (NVML + system APIs)
            |     +-- internal/health (/healthz, /readyz on :8081)
            |
            +-- GPUCluster CRD (api/v1alpha1)
            |
            +-- NVIDIA Device Plugin DaemonSet (installed by preinstall)
```

| Component | Location | Runs On | Description |
|-----------|----------|---------|-------------|
| Pre-install script | `scripts/preinstall.sh` | Host (bare metal) | Installs drivers, toolkit, k3s, RuntimeClass, device plugin |
| Pre-install Go wrapper | `cmd/preinstall/` | Host (bare metal) | Go binary that wraps the shell script for cross-platform consistency |
| Operator binary | `cmd/operator/` | k3s Pod (Deployment) | Main entry point: creates manager, registers controller, starts health server |
| Device Discovery | `internal/discovery/` | k3s Pod (via operator) | Enumerates CPU, GPU (NVML), and memory; returns `DeviceInfo` |
| Controller | `internal/controller/` | k3s Pod (via operator) | Watches GPUCluster CRD, reconciles state, updates status |
| Health Server | `internal/health/` | k3s Pod (via operator) | HTTP server on :8081 with /healthz and /readyz |
| CRD Types | `api/v1alpha1/` | Build-time + runtime | Go types matching `deploy/crds/gpucluster.yaml` |
| Config Engine | `internal/config/` | k3s Pod (via operator) | Generates Helm values from hardware + profiles, validates resource overcommit |
| Metrics | `internal/metrics/` | k3s Pod (via operator) | Prometheus gauges/counters for GPU telemetry and reconciliation stats |

### 2.2 Data Flow

```
                       kubectl apply gpucluster.yaml
                                    |
                                    v
                         K8s API Server (k3s)
                                    |
                           (watch event)
                                    v
                      +---------------------------+
                      |  GPUClusterReconciler     |
                      |  (internal/controller)    |
                      +---------------------------+
                         |                    |
                    1. Fetch CR          7. Update CR status
                    2. Handle deletion   8. Label node
                    3. Ensure finalizer  9. Set conditions
                         |
                         v
                      +---------------------------+
                      |  Discoverer               |
                      |  (internal/discovery)     |
                      +---------------------------+
                         |          |          |
                   CPU info    GPU info    Memory info
                  (ghw/proc)   (NVML)     (ghw/proc)
                         |          |          |
                         v          v          v
                      +---------------------------+
                      |  DeviceInfo struct        |
                      +---------------------------+
                         |                    |
                         v                    v
                      +----------------+  +------------------+
                      | Config Engine  |  | Prometheus       |
                      | (internal/     |  | Metrics          |
                      |  config)       |  | (internal/       |
                      +----------------+  |  metrics)        |
                         |                +------------------+
                         v                    |
                      ConfigMaps per          v
                      profile in           :8080/metrics
                      idealab-system       GPU gauges +
                                           reconcile counters
```

### 2.3 Reconciliation State Machine

```
GPUCluster Created
        |
        v
    [Pending] ----> handle deletion? --yes--> cleanup ConfigMaps, remove finalizer
        |               |
        no              |
        v               v
    ensure finalizer (if profiles exist)
        |
        v
    set phase "Discovering"
        |
        v
    Run device discovery
        |
  +-----+------+
  |            |
Success     Failure
  |            |
  v            v
Label node  [Error]
Record GPU   phase="Error"
 metrics     Ready=False
  |          requeue with backoff
  v
If profiles:
  reconcileConfigMaps
  checkResourceWarning
  recordConfigMapCount
  |
  v
[Ready]
 phase="Ready"
 Ready=True
 Discovering=False
 record reconcile metrics
 requeue in 5 minutes
```

### 2.4 Module Dependency Graph

```
cmd/operator
  +-- internal/controller
  |     +-- internal/discovery
  |     +-- internal/config
  |     +-- internal/metrics
  |     +-- api/v1alpha1
  |     +-- sigs.k8s.io/controller-runtime
  +-- internal/metrics
  +-- internal/health
  +-- api/v1alpha1
        +-- k8s.io/apimachinery

internal/config
  +-- gopkg.in/yaml.v3

internal/metrics
  +-- github.com/prometheus/client_golang
  +-- sigs.k8s.io/controller-runtime/pkg/metrics

internal/discovery
  +-- github.com/NVIDIA/go-nvml/pkg/nvml
  +-- github.com/jaypipes/ghw (or /proc parsing)
  +-- log/slog (stdlib)

internal/health
  +-- net/http (stdlib)
  +-- log/slog (stdlib)

api/v1alpha1
  +-- k8s.io/apimachinery/pkg/apis/meta/v1
  +-- k8s.io/apimachinery/pkg/runtime
  +-- k8s.io/apimachinery/pkg/runtime/schema
  +-- sigs.k8s.io/controller-runtime/pkg/scheme
```

---

## 3. Detailed Module Design

### 3.1 api/v1alpha1 -- CRD Type Definitions

**Package:** `github.com/bibhuti-kar/idealab/api/v1alpha1`

**Files:**
- `types.go` -- Go struct definitions matching `deploy/crds/gpucluster.yaml`
- `groupversion_info.go` -- Scheme registration, GroupVersion constant
- `zz_generated.deepcopy.go` -- DeepCopy methods (generated by controller-gen)

**Types (aligned with CRD YAML):**

```
GPUCluster (metav1.TypeMeta, metav1.ObjectMeta)
  +-- Spec: GPUClusterSpec
  |     +-- Driver: DriverSpec
  |     |     +-- Enabled: bool (default true)
  |     |     +-- Version: string
  |     +-- DevicePlugin: DevicePluginSpec
  |     |     +-- Enabled: bool (default true)
  |     +-- GPUFeatureDiscovery: GPUFeatureDiscoverySpec
  |     |     +-- Enabled: bool (default true)
  |     +-- ApplicationProfiles: []ApplicationProfile  (Sprint 2 / E4)
  |           +-- Name: string
  |           +-- HelmChart: string
  |           +-- HelmValues: runtime.RawExtension
  |           +-- Resources: ProfileResources
  |                 +-- GPUCount: int (default 1)
  |                 +-- GPUMemory: string
  |                 +-- CPULimit: string
  |                 +-- MemoryLimit: string
  +-- Status: GPUClusterStatus
        +-- Phase: string (enum: Pending, Discovering, Ready, Error)
        +-- Node: NodeInfo
        |     +-- Hostname: string
        |     +-- CPU: CPUInfo
        |     |     +-- Model: string
        |     |     +-- Cores: int
        |     |     +-- Threads: int
        |     |     +-- Features: []string
        |     +-- GPU: GPUInfo
        |     |     +-- Model: string
        |     |     +-- VRAMMB: int
        |     |     +-- DriverVersion: string
        |     |     +-- CUDAVersion: string
        |     |     +-- ComputeCapability: string
        |     +-- Memory: MemoryInfo
        |           +-- TotalMB: int
        +-- Conditions: []metav1.Condition

GPUClusterList (metav1.TypeMeta, metav1.ListMeta)
  +-- Items: []GPUCluster
```

**Scheme registration:**
- GroupVersion: `idealab.io/v1alpha1`
- Kind: `GPUCluster`
- Register via `runtime.SchemeBuilder` for controller-runtime compatibility.

**CRD YAML note:** The existing `deploy/crds/gpucluster.yaml` defines the CRD
with status as a subresource and printer columns for Phase, GPU, VRAM, and Age.
The Go types must produce compatible JSON tags (`json:"phase,omitempty"` etc.).

### 3.2 internal/discovery -- Device Enumeration

**Package:** `github.com/bibhuti-kar/idealab/internal/discovery`

**Files:**
- `discovery.go` -- `Discoverer` interface and `DeviceInfo` struct
- `nvml.go` -- `NVMLDiscoverer` implementation (real hardware)
- `cpu.go` -- CPU enumeration helpers (ghw or /proc/cpuinfo parsing)
- `memory.go` -- Memory enumeration helpers (ghw or /proc/meminfo parsing)
- `mock.go` -- `MockDiscoverer` for testing

#### 3.2.1 Interface

```
type Discoverer interface {
    Discover() (DeviceInfo, error)
}
```

#### 3.2.2 Data Structures

```
DeviceInfo
  +-- Hostname: string
  +-- CPU: CPUInfo
  |     +-- Model: string           // "Intel(R) Core(TM) i5-9300H CPU @ 2.40GHz"
  |     +-- Cores: int              // 4 (physical)
  |     +-- Threads: int            // 8 (logical)
  |     +-- Architecture: string    // "x86_64"
  |     +-- Features: []string      // ["SSE4.2", "AVX", "AVX2"]
  +-- GPUs: []GPUInfo
  |     +-- Model: string           // "NVIDIA GeForce GTX 1660 Ti"
  |     +-- UUID: string            // "GPU-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
  |     +-- VRAMMB: int             // 6144
  |     +-- DriverVersion: string   // "560.35.03"
  |     +-- CUDAVersion: string     // "12.6"
  |     +-- ComputeCapability: string // "7.5"
  |     +-- Temperature: int        // degrees C (optional, 0 if unavailable)
  |     +-- UtilizationPct: int     // 0-100 (optional, 0 if unavailable)
  +-- Memory: MemoryInfo
        +-- TotalMB: int            // total system RAM in MB
        +-- AvailableMB: int        // available RAM at discovery time in MB
```

**Note on CRD mapping:** The CRD status has a single `gpu` object (not an
array), reflecting the single-GPU target hardware. The `DeviceInfo.GPUs` field
is a slice to handle the general case, but the controller will populate
`status.node.gpu` from `GPUs[0]` for the single-GPU scenario.

#### 3.2.3 NVMLDiscoverer

Initialization sequence:
1. Call `nvml.Init()` -- if this fails, return error (no GPU driver).
2. Defer `nvml.Shutdown()`.
3. Call `nvml.DeviceGetCount()` for GPU count.
4. For each GPU index, call `nvml.DeviceGetHandleByIndex(i)` and query:
   - `device.GetName()` -- model name
   - `device.GetUUID()` -- unique identifier
   - `device.GetMemoryInfo()` -- total VRAM (convert bytes to MB)
   - `nvml.SystemGetDriverVersion()` -- host driver version string
   - `nvml.SystemGetCudaDriverVersion()` -- CUDA driver version (integer, convert to major.minor)
   - `device.GetCudaComputeCapability()` -- major, minor ints, format as "major.minor"
   - `device.GetTemperature(nvml.TEMPERATURE_GPU)` -- degrees C (optional)
   - `device.GetUtilizationRates()` -- GPU utilization percent (optional)
5. For CPU: read `/proc/cpuinfo` or use `ghw.CPU()`.
6. For memory: read `/proc/meminfo` or use `ghw.Memory()`.
7. For hostname: use `os.Hostname()`.

**NVML error handling pattern:**

Every NVML call returns a `nvml.Return` value. The pattern for optional fields:

```
value, ret := device.GetTemperature(nvml.TEMPERATURE_GPU)
if ret == nvml.ERROR_NOT_SUPPORTED {
    // Log at debug level, set field to zero value
} else if ret != nvml.SUCCESS {
    // Log at warn level, set field to zero value
} else {
    // Use value
}
```

Required fields (name, UUID, memory, driver version) treat non-SUCCESS returns
as errors that fail the entire discovery.

#### 3.2.4 MockDiscoverer

Returns a pre-configured `DeviceInfo` for unit testing. Allows tests to:
- Set specific GPU properties (model, VRAM, compute capability).
- Simulate discovery failure by returning an error.
- Simulate partial data (e.g., temperature unavailable).

### 3.3 internal/controller -- GPUCluster Reconciler

**Package:** `github.com/bibhuti-kar/idealab/internal/controller`

**Files:**
- `reconciler.go` -- `GPUClusterReconciler` struct and `Reconcile()` method
- `conditions.go` -- Condition helper functions
- `labels.go` -- Node labeling logic

#### 3.3.1 Reconciler Struct

```
GPUClusterReconciler
  +-- Client: client.Client              // controller-runtime k8s client
  +-- Scheme: *runtime.Scheme            // for GVK resolution
  +-- Discoverer: discovery.Discoverer   // injected (real or mock)
  +-- Logger: *slog.Logger               // structured logger
  +-- Recorder: record.EventRecorder     // k8s event recorder
  +-- Namespace: string                  // from POD_NAMESPACE env, default "idealab-system"
  +-- reconciled: atomic.Bool            // tracks if at least one reconcile succeeded
```

#### 3.3.2 Reconcile Loop

```
func (r *GPUClusterReconciler) Reconcile(ctx, req) (ctrl.Result, error):

  1. Fetch GPUCluster by req.NamespacedName
     - If NotFound: return (no requeue) -- resource deleted
     - If error: return error (requeue with backoff)

  2. Handle deletion: if DeletionTimestamp set, cleanup ConfigMaps,
     remove finalizer, return.

  3. If profiles defined, ensure finalizer "idealab.io/configmap-cleanup".

  4. If status.phase is empty or "Pending":
     - Set status.phase = "Discovering"
     - Set condition Discovering=True, reason="DiscoveryInProgress"
     - Update status subresource
     - Record event "DiscoveryStarted"

  5. Run r.Discoverer.Discover()
     - If error:
       - Set status.phase = "Error"
       - Set condition Ready=False, reason="DiscoveryFailed", message=err.Error()
       - Set condition Discovering=False
       - Update status subresource
       - Record event "DiscoveryFailed"
       - Return ctrl.Result{RequeueAfter: backoff}, nil

  6. Map DeviceInfo to GPUCluster status fields.

  7. Label the k3s node with GPU metadata.

  8. Record GPU Prometheus metrics (temperature, utilization, VRAM, power).

  9. If profiles defined:
     - reconcileConfigMaps: generate values, create/update ConfigMaps, cleanup orphans.
     - checkResourceWarning: set status.resourceWarning if overcommit.
     - recordConfigMapCount: update Prometheus gauge.

  10. Set status.phase = "Ready", record reconcile duration + success counter.

  11. Return ctrl.Result{RequeueAfter: 5 * time.Minute}
```

#### 3.3.3 Backoff Strategy

On transient errors (NVML init failure, API server unavailable), the controller
uses controller-runtime's built-in exponential backoff:

- Return `ctrl.Result{RequeueAfter: duration}` with increasing intervals.
- Base: 5 seconds, max: 5 minutes, multiplier: 2.
- The controller-runtime rate limiter handles this if we return an error
  from `Reconcile()`.

#### 3.3.4 Node Labels

Labels follow the Kubernetes label convention (`<prefix>/<key>=<value>`):

| Label | Example Value | Source |
|-------|---------------|--------|
| `idealab.io/gpu-model` | `NVIDIA-GeForce-GTX-1660-Ti` | `GPUInfo.Model` (sanitized: spaces to dashes) |
| `idealab.io/gpu-vram-mb` | `6144` | `GPUInfo.VRAMMB` |
| `idealab.io/gpu-driver` | `560.35.03` | `GPUInfo.DriverVersion` |
| `idealab.io/gpu-cuda` | `12.6` | `GPUInfo.CUDAVersion` |
| `idealab.io/gpu-compute` | `7.5` | `GPUInfo.ComputeCapability` |

The reconciler patches the node object using `client.MergeFrom` to avoid
overwriting other labels.

### 3.4 internal/health -- Health Server

**Package:** `github.com/bibhuti-kar/idealab/internal/health`

**Files:**
- `server.go` -- HTTP server with /healthz and /readyz

#### 3.4.1 Design

```
Server
  +-- Port: int                     // from HEALTH_PORT env var, default 8081
  +-- Ready: func() bool            // callback that checks reconciler state
  +-- Logger: *slog.Logger

Endpoints:
  GET /healthz -> 200 {"status": "ok"}      (always, if process is alive)
  GET /readyz  -> 200 {"status": "ready"}    (if Ready() returns true)
                  503 {"status": "not ready"} (if Ready() returns false)
```

The `Ready` callback is wired to `GPUClusterReconciler.reconciled.Load()` so
that /readyz returns 503 until the first successful reconciliation.

The health server runs in a separate goroutine, started before the
controller-runtime Manager. If the health server fails to bind, the operator
exits with a non-zero code.

### 3.5 cmd/operator -- Main Entry Point

**Package:** `github.com/bibhuti-kar/idealab/cmd/operator`

**Files:**
- `main.go` -- Operator bootstrap

#### 3.5.1 Startup Sequence

```
1. Parse configuration from environment variables:
   - LOG_LEVEL (default: "info")
   - LOG_FORMAT (default: "json")
   - HEALTH_PORT (default: "8081")

2. Initialize slog logger with JSON handler and configured level.

3. Create controller-runtime Manager:
   - Scheme: register api/v1alpha1 types + core v1 types
   - MetricsBindAddress: ":8080" (controller-runtime default, or "0" to disable)
   - HealthProbeBindAddress: disabled (we use our own health server)
   - LeaderElection: false (single replica)

4. Create NVMLDiscoverer (or MockDiscoverer if MOCK_DISCOVERY=true).

5. Create GPUClusterReconciler with Client, Scheme, Discoverer, Logger.

6. Register reconciler with Manager via ctrl.NewControllerManagedBy(mgr):
   - For(&v1alpha1.GPUCluster{})
   - Owns(&corev1.ConfigMap{})  // for Sprint 2 config templates

7. Start health server in background goroutine.

8. Start Manager (blocking) -- runs controller loop.
   - On error: log.Fatal and exit 1.
```

#### 3.5.2 Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | `info` | Logging level: debug, info, warn, error |
| `LOG_FORMAT` | `json` | Log format: json, text |
| `HEALTH_PORT` | `8081` | Port for /healthz and /readyz |
| `MOCK_DISCOVERY` | `false` | Use MockDiscoverer instead of NVML (for dev/test) |

### 3.6 internal/config -- Helm Values Generation (E4)

**Package:** `github.com/bibhuti-kar/idealab/internal/config`

**Files:**
- `merge.go` -- Deep map merge (src wins)
- `values.go` -- `GenerateValues(profile, hardware)` combines hardware defaults with user overrides
- `validate.go` -- `CheckResourceOvercommit(profiles, availableVRAMMB)` advisory warning
- `render.go` -- `RenderYAML(values)` marshals to YAML bytes for ConfigMap

#### 3.6.1 GenerateValues Logic

1. Build hardware defaults: `gpu.model`, `gpu.vramMB` (minus 512MB reserve),
   `gpu.computeCapability`, `gpu.cudaVersion`, `gpu.driverVersion`.
2. Set `resources.limits.nvidia.com/gpu` from profile GPUCount (default 1 if GPU present).
3. Set CPU/memory limits from profile or fall back to hardware info.
4. Deep-merge user `HelmValues` on top — user values win.

#### 3.6.2 ConfigMap Reconciliation

In `internal/controller/configmaps.go`:

- Per profile: generate values, render YAML, create/update ConfigMap.
- ConfigMap name: `{gpucluster-name}-{profile-name}-values`.
- Labels: `idealab.io/gpucluster`, `idealab.io/profile`, `app.kubernetes.io/managed-by: idealab-operator`.
- Data key: `values.yaml`.
- Orphan cleanup: list ConfigMaps by label, delete any not matching active profiles.

#### 3.6.3 Finalizer

In `internal/controller/finalizer.go`:

- Finalizer name: `idealab.io/configmap-cleanup`.
- Added when profiles exist; on GPUCluster deletion, deletes all managed ConfigMaps
  then removes the finalizer.
- No owner references (GPUCluster is cluster-scoped, ConfigMaps are namespaced).

#### 3.6.4 CRD Status Extensions

- `ProfileStatuses []ProfileStatus` -- per-profile reconciliation state (name, configMapName, ready).
- `ResourceWarning string` -- set when total requested GPU memory exceeds available VRAM.

### 3.7 internal/metrics -- Prometheus Metrics (E5)

**Package:** `github.com/bibhuti-kar/idealab/internal/metrics`

**Files:**
- `metrics.go` -- Prometheus gauge/counter/histogram definitions + `RegisterAll()`

#### 3.7.1 Custom Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `idealab_gpu_temperature_celsius` | Gauge | gpu, uuid | GPU temperature |
| `idealab_gpu_utilization_percent` | Gauge | gpu, uuid | GPU core utilization |
| `idealab_gpu_vram_total_mb` | Gauge | gpu, uuid | Total VRAM |
| `idealab_gpu_vram_used_mb` | Gauge | gpu, uuid | Used VRAM |
| `idealab_gpu_power_watts` | Gauge | gpu, uuid | Power consumption |
| `idealab_reconcile_total` | Counter | result | Reconciliation count (success/error) |
| `idealab_reconcile_duration_seconds` | Histogram | -- | Reconciliation duration |
| `idealab_configmaps_generated` | Gauge | -- | Number of managed ConfigMaps |

#### 3.7.2 Controller Metrics Helpers

In `internal/controller/metrics.go`:

- `recordGPUMetrics(info)` -- updates all GPU gauges from discovery data.
- `recordConfigMapCount(n)` -- sets the configmaps_generated gauge.
- Reconcile timing and counters recorded directly in `Reconcile()`.

#### 3.7.3 Metrics Endpoint

Controller-runtime serves metrics on `:8080/metrics`. Configurable via
`METRICS_BIND_ADDRESS` env var (default `:8080`).

### 3.8 cmd/preinstall -- Pre-Install Go Wrapper

**Package:** `github.com/bibhuti-kar/idealab/cmd/preinstall`

**Files:**
- `main.go` -- Wraps `scripts/preinstall.sh` execution

The Go binary provides a structured entry point for the pre-install script.
For Sprint 1, it delegates to `scripts/preinstall.sh` via `os/exec`. The
existing shell script already handles:

1. Root check
2. OS detection (Ubuntu/Debian)
3. Base package installation
4. GPU detection via `lspci`
5. NVIDIA driver installation (or skip if >= 560 exists)
6. NVIDIA Container Toolkit installation
7. containerd configuration for NVIDIA runtime
8. Go installation
9. k3s installation
10. k3s NVIDIA RuntimeClass creation
11. NVIDIA device plugin deployment
12. Validation of all components

**Script improvement needed for AC compliance:**
- The existing script installs `nvidia-driver-550`. This must be updated to
  install `nvidia-driver-560` or later per AC for IDEAL-102 (driver >= 560).
- The existing script does not run a GPU test pod (IDEAL-103 AC3). A validation
  step that deploys a test pod requesting `nvidia.com/gpu: 1`, runs `nvidia-smi`
  inside it, and cleans up is needed.

---

## 4. Go Dependencies

### 4.1 Required Modules

| Module | Version | Purpose |
|--------|---------|---------|
| `sigs.k8s.io/controller-runtime` | v0.17.x | Operator framework: Manager, Controller, Reconciler, Client |
| `k8s.io/client-go` | v0.29.x | Kubernetes API client (transitive via controller-runtime) |
| `k8s.io/apimachinery` | v0.29.x | API object types, scheme, serialization |
| `k8s.io/api` | v0.29.x | Core Kubernetes API types (Node, ConfigMap, etc.) |
| `github.com/NVIDIA/go-nvml` | v0.13.x | NVML Go bindings for GPU enumeration |
| `github.com/jaypipes/ghw` | v0.12.x | CPU and memory enumeration |
| `github.com/prometheus/client_golang` | v1.18.x | Prometheus metrics SDK |
| `gopkg.in/yaml.v3` | v3.0.x | YAML rendering for Helm values ConfigMaps |

### 4.2 Standard Library (no go.mod entry needed)

| Package | Purpose |
|---------|---------|
| `log/slog` | Structured logging (Go 1.21+) |
| `net/http` | Health server |
| `os` | Environment variables, hostname |
| `os/exec` | Pre-install script execution |
| `sync/atomic` | Thread-safe ready flag |
| `context` | Context propagation |
| `fmt` | String formatting |
| `strings` | Label sanitization |
| `strconv` | Numeric conversions |
| `time` | Requeue durations |

### 4.3 Test Dependencies

| Module | Version | Purpose |
|--------|---------|---------|
| `sigs.k8s.io/controller-runtime/pkg/envtest` | (same as controller-runtime) | Integration testing with real API server |
| `github.com/stretchr/testify` | v1.9.x | Test assertions |
| `go.uber.org/goleak` | v1.3.x | Goroutine leak detection in tests |

### 4.4 Build Note: CGO and NVML

`go-nvml` uses cgo to link against `libnvidia-ml.so`. This means:
- **Build requires:** `CGO_ENABLED=1` and NVML headers/stubs available.
- **Runtime requires:** `libnvidia-ml.so` on the host (provided by NVIDIA driver).
- **Docker build:** Use `nvidia/cuda:12.6.0-devel-ubuntu22.04` as the builder
  base image (provides NVML headers and stub library).
- **Test without GPU:** Use build tags (`//go:build !nvml`) to compile with
  `MockDiscoverer` when NVML is unavailable. Alternatively, use the
  `MOCK_DISCOVERY=true` env var at runtime.

The existing Dockerfile uses `golang:1.22-alpine` with `CGO_ENABLED=0`. This
will need to change for the NVML integration. The updated builder stage should
use a CUDA-capable base image.

---

## 5. Error Handling Strategy

### 5.1 NVML Errors

| NVML Return | Handling | Impact |
|-------------|----------|--------|
| `SUCCESS` | Use returned value | Normal path |
| `ERROR_NOT_SUPPORTED` | Log at debug level, set field to zero value | Expected on consumer GPUs for ECC, MIG, NVLink, fan speed |
| `ERROR_UNINITIALIZED` | Return error from `Discover()` | Fatal: driver not loaded |
| `ERROR_NO_PERMISSION` | Return error from `Discover()` | Fatal: container lacks GPU access |
| `ERROR_NOT_FOUND` | Return error from `Discover()` | Fatal: no GPU device |
| `ERROR_INSUFFICIENT_SIZE` | Retry with larger buffer (if applicable) | Transient |
| `ERROR_UNKNOWN` | Log at error level, set field to zero value | Degraded but not fatal |

### 5.2 Controller Errors

| Error Scenario | Handling |
|---------------|----------|
| GPUCluster CR not found | Return `ctrl.Result{}` (no requeue) -- resource was deleted |
| API server unreachable | Return error (controller-runtime auto-requeues with backoff) |
| Status update conflict | Return error (controller-runtime retries) |
| Discovery failure | Set Error phase, requeue with explicit backoff (5s base, 5m max) |
| Node labeling failure | Log warning, set condition, still mark Ready if discovery succeeded |

### 5.3 Pre-Install Script Errors

Each function in `scripts/preinstall.sh` validates its result before the next
step proceeds. The script uses `set -euo pipefail` for strict error handling.

| Step | Validation | On Failure |
|------|-----------|------------|
| GPU detection | `lspci` grep for NVIDIA PCI vendor ID | Exit 1 with "No NVIDIA GPU detected" |
| Driver install | `nvidia-smi` runs successfully | Exit 1 with driver error |
| Toolkit install | `nvidia-ctk` command exists | Exit 1 with toolkit error |
| k3s install | `k3s kubectl get nodes` succeeds | Exit 1 with k3s error |
| RuntimeClass | `k3s kubectl get runtimeclass nvidia` succeeds | Exit 1 |
| GPU validation | Test pod completes with exit 0 | Exit 1 with validation error |

---

## 6. Deployment Architecture

### 6.1 Kubernetes Resources

| Resource | Name | Namespace | Purpose |
|----------|------|-----------|---------|
| Namespace | `idealab-system` | -- | Operator namespace |
| ServiceAccount | `idealab-operator` | `idealab-system` | Operator identity |
| ClusterRole | `idealab-operator` | -- | RBAC permissions |
| ClusterRoleBinding | `idealab-operator` | -- | Binds SA to ClusterRole |
| Deployment | `idealab-operator` | `idealab-system` | Operator pod (1 replica) |
| CRD | `gpuclusters.idealab.io` | -- | GPUCluster custom resource |

### 6.2 RBAC Permissions (from deploy/operator/rbac.yaml)

| API Group | Resource | Verbs | Reason |
|-----------|----------|-------|--------|
| `idealab.io` | `gpuclusters` | get, list, watch, create, update, patch, delete | Manage GPUCluster CRs |
| `idealab.io` | `gpuclusters/status` | get, update, patch | Update status subresource |
| `""` (core) | `nodes` | get, list, watch, patch | Label nodes with GPU metadata |
| `""` (core) | `pods` | get, list, watch | Observe pod state |
| `""` (core) | `configmaps` | get, list, watch, create, update, patch | Sprint 2: generated Helm values |
| `apps` | `deployments`, `daemonsets` | get, list, watch, create, update, patch, delete | Future: manage operand workloads |
| `""` (core) | `events` | create, patch | Record reconciliation events |

### 6.3 Operator Pod Spec (from deploy/operator/deployment.yaml)

- Image: `idealab-operator:latest` (will be pinned for production)
- Port: 8080 (metrics), 8081 (health)
- Liveness: `GET /healthz` every 30s, initial delay 15s
- Readiness: `GET /readyz` every 10s, initial delay 5s
- Resources: 100m-200m CPU, 128Mi-256Mi memory
- Security: non-root, read-only root filesystem, drop all capabilities, no privilege escalation

### 6.4 Docker Image

The Dockerfile needs updating for NVML support. The target design:

```
Builder stage:
  - Base: nvidia/cuda:12.6.0-devel-ubuntu22.04 (provides NVML headers)
  - Install Go 1.22
  - CGO_ENABLED=1 (required for go-nvml cgo bindings)
  - Build operator binary and preinstall binary

Runtime stage:
  - Base: nvidia/cuda:12.6.0-base-ubuntu22.04 (provides libnvidia-ml.so stub)
  - Copy binaries from builder
  - Non-root user (nonroot:nonroot or appuser)
  - EXPOSE 8081
  - HEALTHCHECK on /healthz
  - ENTRYPOINT ["/operator"]
```

Using `gcr.io/distroless/static-debian12` (current Dockerfile) will not work
because `go-nvml` requires `libnvidia-ml.so` at runtime. The CUDA base image
provides the stub library; the actual driver library is bind-mounted from the
host by the NVIDIA Container Toolkit at container start.

---

## 7. Configuration

### 7.1 Operator Configuration (Environment Variables)

All configuration follows the 12-factor principle: environment variables,
validated at startup, never hardcoded.

| Variable | Type | Default | Validation |
|----------|------|---------|------------|
| `LOG_LEVEL` | string | `info` | Must be one of: debug, info, warn, error |
| `LOG_FORMAT` | string | `json` | Must be one of: json, text |
| `HEALTH_PORT` | int | `8081` | Must be 1024-65535 |
| `MOCK_DISCOVERY` | bool | `false` | Must be true or false |
| `POD_NAMESPACE` | string | `idealab-system` | Set via Downward API in deployment |
| `METRICS_BIND_ADDRESS` | string | `:8080` | Address for Prometheus metrics endpoint |

### 7.2 GPUCluster CR Example

```yaml
apiVersion: idealab.io/v1alpha1
kind: GPUCluster
metadata:
  name: my-gpu-cluster
spec:
  driver:
    enabled: true
    version: "560"
  devicePlugin:
    enabled: true
  gpuFeatureDiscovery:
    enabled: true
  applicationProfiles:
    - name: ollama-inference
      helmChart: ollama/ollama
      helmValues:
        ollama:
          gpu:
            enabled: true
          models:
            - phi3:mini
      resources:
        gpuCount: 1
        gpuMemory: "4Gi"
        cpuLimit: "4"
        memoryLimit: "8Gi"
```

After reconciliation, the status is auto-populated and ConfigMaps are generated:

```yaml
status:
  phase: Ready
  node:
    hostname: gaming-pc
    cpu:
      model: "Intel(R) Core(TM) i5-9300H CPU @ 2.40GHz"
      cores: 4
      threads: 8
      features: ["SSE4.2", "AVX", "AVX2"]
    gpu:
      model: "NVIDIA GeForce GTX 1660 Ti"
      vramMB: 6144
      driverVersion: "560.35.03"
      cudaVersion: "12.6"
      computeCapability: "7.5"
    memory:
      totalMB: 16384
  profileStatuses:
    - name: ollama-inference
      configMapName: my-gpu-cluster-ollama-inference-values
      ready: true
  conditions:
    - type: Ready
      status: "True"
      reason: ReconcileSucceeded
      message: "Device discovery completed successfully"
    - type: Discovering
      status: "False"
      reason: DiscoveryComplete
```

---

## 8. Logging

All logging uses `log/slog` (Go stdlib) with JSON output by default.

### 8.1 Log Levels

| Level | Usage |
|-------|-------|
| `DEBUG` | NVML NOT_SUPPORTED returns, detailed reconciliation steps, individual field discovery |
| `INFO` | Reconciliation start/complete, phase transitions, node label updates, startup/shutdown |
| `WARN` | Non-fatal errors (node label failure, optional field unavailable), degraded operation |
| `ERROR` | Fatal discovery failure, API server errors, health server bind failure |

### 8.2 Structured Fields

Every log line includes:
- `ts` -- ISO 8601 timestamp
- `level` -- log level
- `msg` -- human-readable message
- `controller` -- "GPUCluster" (when in reconciler context)
- `gpucluster` -- name of the resource being reconciled
- `phase` -- current phase (when applicable)
- Additional context fields per message

---

## 9. File Layout

```
idealab/
  api/
    v1alpha1/
      types.go                  # GPUCluster, GPUClusterSpec, GPUClusterStatus
      groupversion_info.go      # SchemeBuilder, GroupVersion
      zz_generated.deepcopy.go  # DeepCopy (generated)
  cmd/
    operator/
      main.go                   # Operator entry point
    preinstall/
      main.go                   # Pre-install Go wrapper
  internal/
    controller/
      reconciler.go             # GPUClusterReconciler
      conditions.go             # Condition helper functions
      labels.go                 # Node labeling logic
      configmaps.go             # ConfigMap reconciliation per profile (E4)
      finalizer.go              # Finalizer for ConfigMap cleanup (E4)
      metrics.go                # GPU metrics recording helper (E5)
    config/
      merge.go                  # Deep map merge
      values.go                 # Helm values generation from hardware + profiles
      validate.go               # Resource overcommit check
      render.go                 # YAML rendering
    metrics/
      metrics.go                # Prometheus gauge/counter/histogram definitions
    discovery/
      discovery.go              # Discoverer interface, DeviceInfo struct
      nvml.go                   # NVMLDiscoverer implementation
      cpu.go                    # CPU enumeration
      memory.go                 # Memory enumeration
      mock.go                   # MockDiscoverer for tests
    health/
      server.go                 # HTTP health server
  deploy/
    crds/
      gpucluster.yaml           # CRD manifest (existing)
    operator/
      namespace.yaml            # Namespace (existing)
      deployment.yaml           # Operator Deployment (existing)
      rbac.yaml                 # RBAC (existing)
    templates/
      gpu-workload.yaml         # Workload template (existing)
  scripts/
    preinstall.sh               # Pre-install bash script (existing)
  tests/
    unit/
      discovery_test.go         # Discovery unit tests
      controller_test.go        # Controller unit tests
      health_test.go            # Health server unit tests
      types_test.go             # CRD type tests
    integration/
      controller_integration_test.go  # envtest-based integration tests
    e2e/
      preinstall_test.go        # Pre-install E2E tests
      operator_test.go          # Full operator E2E tests
  docs/
    design/
      technical-design.md       # This document
      test-plan.md              # Test plan
    prd.md                      # Product requirements (existing)
    research/                   # Research docs (existing)
    stories/                    # User stories (existing)
  Dockerfile                    # Multi-stage build (existing, needs NVML update)
  docker-compose.yml            # Local dev (existing)
  Makefile                      # Build targets (existing)
  go.mod                        # Go module (existing)
  go.sum                        # Dependency checksums (to be generated)
```

---

## 10. Open Questions

| ID | Question | Impact | Decision |
|----|----------|--------|----------|
| Q1 | Should discovery re-run periodically or only on CR create/update? | Stale hardware data if thermal state changes | **Decision:** Re-reconcile every 5 minutes to refresh temperature/utilization. Phase stays "Ready" on refresh. |
| Q2 | Should the operator manage the NVIDIA device plugin DaemonSet? | Scope creep vs. convenience | **Deferred to Sprint 2.** Pre-install script handles device plugin installation. |
| Q3 | Should the CRD status store a GPU array or single GPU object? | CRD YAML currently has single `gpu` object | **Decision:** Keep single `gpu` in CRD status (matches single-GPU target). Discovery module returns a slice internally for forward compatibility. |
| Q4 | How to handle NVML unavailability in CI (no GPU hardware)? | Cannot run real NVML tests in CI | **Decision:** Use `MockDiscoverer` in unit tests. Integration tests use envtest with mock. E2E tests require GPU hardware or are skipped. |
| Q5 | Should the Dockerfile use CUDA base image or distroless? | NVML runtime dependency vs. image size | **Decision:** Use CUDA base image for runtime (provides libnvidia-ml.so stub). Accept the larger image size as a necessary tradeoff. |

---

## References

- PRD: `/home/bibs/work/idealab/docs/prd.md`
- Research: `/home/bibs/work/idealab/docs/research/gpu-operator-research.md`
- Stories: `/home/bibs/work/idealab/docs/stories/`
- CRD YAML: `/home/bibs/work/idealab/deploy/crds/gpucluster.yaml`
- NVIDIA go-nvml: https://github.com/NVIDIA/go-nvml
- controller-runtime: https://github.com/kubernetes-sigs/controller-runtime
- k3s GPU support: https://docs.k3s.io/advanced#nvidia-container-runtime-support
