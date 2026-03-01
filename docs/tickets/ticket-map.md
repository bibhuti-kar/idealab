# Ticket Map -- idealab GPU Operator

**Project:** idealab
**JIRA Key:** IDEAL
**Created:** 2026-03-01
**Author:** Tech Manager

---

## Sprint Allocation

| Sprint | Epics | Ticket Range |
|--------|-------|-------------|
| Sprint 1 | E1 + E2 + E3 | IDEAL-010, IDEAL-101, IDEAL-201 -- IDEAL-306 |
| Sprint 2 | E4 | IDEAL-401 -- IDEAL-403 (backlog) |

---

## EPIC: Infrastructure

| ID | Type | Title | Priority | Size | Status | Deps | JIRA/GH |
|----|------|-------|----------|------|--------|------|---------|
| IDEAL-010 | Task | Initialize Go module with dependencies | P0 | S | TODO | -- | -- |

---

## EPIC: E1 -- Pre-Install Script

| ID | Type | Title | Priority | Size | Status | Deps | JIRA/GH |
|----|------|-------|----------|------|--------|------|---------|
| IDEAL-101 | Story | Update pre-install script | P0 | M | TODO | IDEAL-010 | -- |

---

## EPIC: E2 -- Device Discovery

| ID | Type | Title | Priority | Size | Status | Deps | JIRA/GH |
|----|------|-------|----------|------|--------|------|---------|
| IDEAL-201 | Story | Define discovery interfaces and types | P0 | S | TODO | IDEAL-010 | -- |
| IDEAL-202 | Task | Implement CPU discovery | P0 | S | TODO | IDEAL-201 | -- |
| IDEAL-203 | Task | Implement GPU discovery via NVML | P0 | M | TODO | IDEAL-201 | -- |
| IDEAL-204 | Task | Implement memory discovery | P0 | S | TODO | IDEAL-201 | -- |
| IDEAL-205 | Task | Integrate NVMLDiscoverer | P0 | M | TODO | IDEAL-202, IDEAL-203, IDEAL-204 | -- |

---

## EPIC: E3 -- Operator Core (CRD + Controller)

| ID | Type | Title | Priority | Size | Status | Deps | JIRA/GH |
|----|------|-------|----------|------|--------|------|---------|
| IDEAL-301 | Story | Define CRD Go types | P0 | M | TODO | IDEAL-010 | -- |
| IDEAL-302 | Task | Implement health server | P0 | S | TODO | IDEAL-010 | -- |
| IDEAL-303 | Story | Implement GPUCluster reconciler | P0 | L | TODO | IDEAL-301, IDEAL-205 | -- |
| IDEAL-304 | Task | Implement node labeling | P0 | S | TODO | IDEAL-303 | -- |
| IDEAL-305 | Task | Implement operator main entry point | P0 | M | TODO | IDEAL-301, IDEAL-302, IDEAL-303 | -- |
| IDEAL-306 | Task | Integration tests with envtest | P0 | M | TODO | IDEAL-303, IDEAL-304, IDEAL-305 | -- |

---

## EPIC: E4 -- Configuration Templates (Sprint 2 Backlog)

| ID | Type | Title | Priority | Size | Status | Deps | JIRA/GH |
|----|------|-------|----------|------|--------|------|---------|
| IDEAL-401 | Story | Application profiles in GPUCluster spec | P1 | M | BACKLOG | IDEAL-301 | -- |
| IDEAL-402 | Story | Helm values generation | P1 | L | BACKLOG | IDEAL-401, IDEAL-205 | -- |
| IDEAL-403 | Story | Generated ConfigMap output | P1 | M | BACKLOG | IDEAL-402 | -- |

---

## Ticket Details

---

### IDEAL-010: Initialize Go module with dependencies

**Type:** Task
**Epic:** Infrastructure
**Priority:** P0
**Size:** S (Small)
**Sprint:** 1
**Blocked By:** --
**Story Ref:** --

**Description:**
Initialize the Go module and add all required dependencies so that subsequent
tickets can import packages and compile.

**Acceptance Criteria:**
1. `go.mod` contains module path `github.com/bibhuti-kar/idealab` with Go 1.22+.
2. All required dependencies are added:
   - `sigs.k8s.io/controller-runtime` v0.17.x
   - `k8s.io/client-go` v0.29.x
   - `k8s.io/apimachinery` v0.29.x
   - `k8s.io/api` v0.29.x
   - `github.com/NVIDIA/go-nvml` v0.12.x
   - `github.com/jaypipes/ghw` v0.12.x
   - `github.com/stretchr/testify` v1.9.x
   - `go.uber.org/goleak` v1.3.x
3. `go mod tidy` runs without errors.
4. A minimal `package main` in `cmd/operator/main.go` compiles with `go build`.

**Files to Create/Modify:**
- `go.mod` (update)
- `go.sum` (generated)
- `cmd/operator/main.go` (minimal placeholder if not present)

**Definition of Done:**
- `go build ./...` succeeds with zero errors.
- All dependencies resolve and download.

---

### IDEAL-101: Update pre-install script

**Type:** Story
**Epic:** E1 -- Pre-Install Script
**Priority:** P0
**Size:** M (Medium)
**Sprint:** 1
**Blocked By:** IDEAL-010
**Story Ref:** IDEAL-101 (S1.1), IDEAL-102 (S1.2), IDEAL-103 (S1.3)

**Description:**
Update the existing `scripts/preinstall.sh` to meet all acceptance criteria from
stories S1.1, S1.2, and S1.3. The existing script already covers most
functionality but requires three specific changes.

**Acceptance Criteria:**
1. Driver version updated: script installs `nvidia-driver-560` or later (was 550).
2. GPU test pod validation step added: after k3s and RuntimeClass are configured,
   the script deploys a test pod requesting `nvidia.com/gpu: 1`, runs `nvidia-smi`
   inside the container, verifies exit code 0, and cleans up the test pod.
3. containerd config path explicitly targets k3s path:
   `/var/lib/rancher/k3s/agent/etc/containerd/config.toml`.
4. Script is idempotent: running twice produces no errors or duplicate installs.
5. Script validates each step before proceeding to the next.
6. Script detects NVIDIA GPU by PCI vendor ID (0x10de) and prints model name.
7. Script skips driver install if compatible driver (>= 560) already exists.
8. After completion: k3s running, node Ready, nvidia RuntimeClass exists.

**Files to Create/Modify:**
- `scripts/preinstall.sh` (update)

**Definition of Done:**
- Script runs end-to-end on Ubuntu 22.04/24.04 with NVIDIA GPU.
- All 9 acceptance criteria from S1.1/S1.2/S1.3 are satisfied.
- E2E tests in test plan (section 4.1) can validate the script.

---

### IDEAL-201: Define discovery interfaces and types

**Type:** Story
**Epic:** E2 -- Device Discovery
**Priority:** P0
**Size:** S (Small)
**Sprint:** 1
**Blocked By:** IDEAL-010
**Story Ref:** IDEAL-204 (S2.4)

**Description:**
Create the `internal/discovery` package with the `Discoverer` interface,
`DeviceInfo`, `CPUInfo`, `GPUInfo`, and `MemoryInfo` structs, plus the
`MockDiscoverer` for testing. This establishes the contract that all subsequent
discovery tickets implement against.

**Acceptance Criteria:**
1. `Discoverer` interface defined with `Discover() (DeviceInfo, error)` method.
2. `DeviceInfo` struct contains: `Hostname` (string), `CPU` (CPUInfo),
   `GPUs` ([]GPUInfo), `Memory` (MemoryInfo).
3. `CPUInfo` struct contains: `Model`, `Cores`, `Threads`, `Architecture`,
   `Features` ([]string).
4. `GPUInfo` struct contains: `Model`, `UUID`, `VRAMMB`, `DriverVersion`,
   `CUDAVersion`, `ComputeCapability`, `Temperature`, `UtilizationPct`.
5. `MemoryInfo` struct contains: `TotalMB`, `AvailableMB`.
6. All structs have JSON tags matching CRD status field names.
7. `MockDiscoverer` returns a configurable `DeviceInfo` or error.
8. Unit tests pass: mock discoverer returns configured data, mock can simulate
   failure, DeviceInfo round-trips through JSON serialization.

**Files to Create:**
- `internal/discovery/discovery.go` (interface + structs)
- `internal/discovery/mock.go` (MockDiscoverer)
- `tests/unit/discovery_test.go` (or co-located `internal/discovery/discovery_test.go`)

**Definition of Done:**
- `go build ./internal/discovery/...` compiles.
- Unit tests for MockDiscoverer and type serialization pass.
- Test coverage >= 80% for this package.

---

### IDEAL-202: Implement CPU discovery

**Type:** Task
**Epic:** E2 -- Device Discovery
**Priority:** P0
**Size:** S (Small)
**Sprint:** 1
**Blocked By:** IDEAL-201
**Story Ref:** IDEAL-201 (S2.1)

**Description:**
Implement CPU enumeration in `internal/discovery/cpu.go`. Read CPU model,
physical cores, logical threads, architecture, and instruction set features
from `/proc/cpuinfo` or using the `ghw` library.

**Acceptance Criteria:**
1. Returns `CPUInfo` with model name (e.g., "Intel(R) Core(TM) i5-9300H CPU @ 2.40GHz").
2. Returns physical core count and logical thread count.
3. Returns CPU architecture (e.g., "x86_64").
4. Returns a list of CPU feature flags (e.g., SSE4.2, AVX, AVX2).
5. Feature list is derived from system APIs, not hardcoded.
6. Empty feature list does not cause an error.
7. Unit tests pass with mock `/proc/cpuinfo` data.

**Files to Create:**
- `internal/discovery/cpu.go`
- Test cases added to `tests/unit/discovery_test.go`

**Definition of Done:**
- Unit tests: `TestCPUInfo_ModelAndTopology`, `TestCPUInfo_FeatureFlags`,
  `TestCPUInfo_EmptyFeatures` all pass.
- Test coverage >= 80% for cpu.go.

---

### IDEAL-203: Implement GPU discovery via NVML

**Type:** Task
**Epic:** E2 -- Device Discovery
**Priority:** P0
**Size:** M (Medium)
**Sprint:** 1
**Blocked By:** IDEAL-201
**Story Ref:** IDEAL-202 (S2.2)

**Description:**
Implement GPU enumeration in `internal/discovery/nvml.go` using the go-nvml
library. Enumerate all GPUs, query all properties (model, UUID, VRAM, driver
version, CUDA version, compute capability, temperature, utilization), and handle
`NOT_SUPPORTED` gracefully for consumer GPU features.

**Acceptance Criteria:**
1. Calls `nvml.Init()` and `nvml.Shutdown()` correctly.
2. Enumerates all GPUs via `nvml.DeviceGetCount()`.
3. For each GPU, queries: name, UUID, memory info, driver version, CUDA version,
   compute capability, temperature, utilization.
4. Required fields (name, UUID, VRAM, driver version) that fail cause
   `Discover()` to return an error.
5. Optional fields (temperature, utilization) that return `NOT_SUPPORTED`
   are set to zero values without error.
6. `NOT_SUPPORTED` returns are logged at debug level with the function name.
7. Multiple GPUs each get unique UUIDs.
8. Unit tests pass with mocked NVML (build tag separation).

**Files to Create:**
- `internal/discovery/nvml.go`
- Test cases added to `tests/unit/discovery_test.go`

**Definition of Done:**
- Unit tests: `TestGPUInfo_AllProperties`, `TestGPUInfo_MultipleGPUs`,
  `TestGPUInfo_SingleGPU`, `TestGPUInfo_NotSupportedDegradation`,
  `TestGPUInfo_NotSupportedLogging`, `TestGPUInfo_RequiredFieldFailure` all pass.
- Build tag strategy documented: tests compile without NVML headers.
- Test coverage >= 80% for nvml.go.

---

### IDEAL-204: Implement memory discovery

**Type:** Task
**Epic:** E2 -- Device Discovery
**Priority:** P0
**Size:** S (Small)
**Sprint:** 1
**Blocked By:** IDEAL-201
**Story Ref:** IDEAL-203 (S2.3)

**Description:**
Implement memory enumeration in `internal/discovery/memory.go`. Read total
and available system memory from `/proc/meminfo` or using the `ghw` library.

**Acceptance Criteria:**
1. Returns `MemoryInfo` with `TotalMB` and `AvailableMB`.
2. Total memory is within 5% of `/proc/meminfo` reported value.
3. Unit tests pass with mock memory data.

**Files to Create:**
- `internal/discovery/memory.go`
- Test cases added to `tests/unit/discovery_test.go`

**Definition of Done:**
- Unit tests: `TestMemoryInfo_TotalAndAvailable`,
  `TestMemoryInfo_ConsistencyWithProcMeminfo` pass.
- Test coverage >= 80% for memory.go.

---

### IDEAL-205: Integrate NVMLDiscoverer

**Type:** Task
**Epic:** E2 -- Device Discovery
**Priority:** P0
**Size:** M (Medium)
**Sprint:** 1
**Blocked By:** IDEAL-202, IDEAL-203, IDEAL-204
**Story Ref:** IDEAL-204 (S2.4)

**Description:**
Wire CPU, GPU, and memory discovery into `NVMLDiscoverer.Discover()`. The
method initializes NVML, calls all three sub-discoverers, assembles the
complete `DeviceInfo`, and returns it. Also resolves hostname via `os.Hostname()`.

**Acceptance Criteria:**
1. `NVMLDiscoverer` implements `Discoverer` interface.
2. `Discover()` returns a complete `DeviceInfo` with hostname, CPU, GPUs, memory.
3. `DeviceInfo` fields map directly to CRD status fields (JSON serializable).
4. Total enumeration completes in under 5 seconds.
5. NVML is initialized before use and shut down after use.
6. Integration test with mock components verifies full assembly.

**Files to Create/Modify:**
- `internal/discovery/nvml.go` (add `Discover()` method wiring)
- Test cases: `TestDeviceInfo_MatchesCRDSchema`, `TestDeviceInfo_Serialization`,
  `TestDeviceInfo_CompletionTime`

**Definition of Done:**
- Integration test: full `Discover()` call returns complete `DeviceInfo`.
- JSON serialization of `DeviceInfo` matches CRD status schema.
- Completion time < 5 seconds with mock data.
- Test coverage >= 80% for discovery package.

---

### IDEAL-301: Define CRD Go types

**Type:** Story
**Epic:** E3 -- Operator Core
**Priority:** P0
**Size:** M (Medium)
**Sprint:** 1
**Blocked By:** IDEAL-010
**Story Ref:** IDEAL-301 (S3.1)

**Description:**
Create Go type definitions for the GPUCluster CRD in `api/v1alpha1/`. The types
must match the existing `deploy/crds/gpucluster.yaml` schema exactly. Register
the types with the controller-runtime scheme. Generate or write DeepCopy methods.

**Acceptance Criteria:**
1. `GPUCluster`, `GPUClusterSpec`, `GPUClusterStatus`, `GPUClusterList` types defined.
2. Spec fields: `Driver` (DriverSpec), `DevicePlugin` (DevicePluginSpec),
   `GPUFeatureDiscovery` (GPUFeatureDiscoverySpec), `ApplicationProfiles`
   ([]ApplicationProfile).
3. Status fields: `Phase` (string), `Node` (NodeInfo with CPU, GPU, Memory),
   `Conditions` ([]metav1.Condition).
4. `GroupVersion` is `idealab.io/v1alpha1`.
5. Types register with `runtime.SchemeBuilder`.
6. DeepCopy methods exist and produce independent copies.
7. JSON tags produce field names matching the CRD YAML properties.
8. Phase enum values: Pending, Discovering, Ready, Error.

**Files to Create:**
- `api/v1alpha1/types.go`
- `api/v1alpha1/groupversion_info.go`
- `api/v1alpha1/zz_generated.deepcopy.go`
- `tests/unit/types_test.go`

**Definition of Done:**
- Types compile and register with scheme.
- Unit tests: `TestGPUCluster_JSONTags`, `TestGPUCluster_StatusFields`,
  `TestGPUCluster_SpecFields`, `TestGPUCluster_PhaseEnum`,
  `TestGPUCluster_DeepCopy`, `TestGPUCluster_SchemeRegistration`,
  `TestGPUCluster_GroupVersion` all pass.
- Test coverage >= 80% for api/v1alpha1 package.

---

### IDEAL-302: Implement health server

**Type:** Task
**Epic:** E3 -- Operator Core
**Priority:** P0
**Size:** S (Small)
**Sprint:** 1
**Blocked By:** IDEAL-010
**Story Ref:** IDEAL-304 (S3.4)

**Description:**
Create the HTTP health server in `internal/health/server.go` with `/healthz`
and `/readyz` endpoints. The server accepts a configurable port and a `Ready`
callback function that reflects whether the reconciler has completed at least
one successful reconciliation.

**Acceptance Criteria:**
1. `GET /healthz` returns HTTP 200 with JSON body `{"status": "ok"}`.
2. `GET /readyz` returns HTTP 200 with `{"status": "ready"}` when `Ready()` returns true.
3. `GET /readyz` returns HTTP 503 with `{"status": "not ready"}` when `Ready()` returns false.
4. Port is configurable (default 8081).
5. Server starts in a goroutine and does not block.
6. Server logs startup and errors via slog.

**Files to Create:**
- `internal/health/server.go`
- `tests/unit/health_test.go`

**Definition of Done:**
- Unit tests: `TestHealthz_Returns200`, `TestHealthz_ResponseBody`,
  `TestReadyz_Returns200_WhenReady`, `TestReadyz_Returns503_WhenNotReady`,
  `TestReadyz_TransitionsToReady`, `TestHealthServer_ConfigurablePort`,
  `TestHealthServer_InvalidPort_Error` all pass.
- Test coverage >= 80% for health package.

---

### IDEAL-303: Implement GPUCluster reconciler

**Type:** Story
**Epic:** E3 -- Operator Core
**Priority:** P0
**Size:** L (Large)
**Sprint:** 1
**Blocked By:** IDEAL-301, IDEAL-205
**Story Ref:** IDEAL-302 (S3.2), IDEAL-303 (S3.3)

**Description:**
Implement the 7-step reconciliation loop in `internal/controller/reconciler.go`
per the technical design document section 3.3.2. The reconciler watches
GPUCluster resources, triggers device discovery, populates status, sets
conditions (Discovering, Ready, Error), records events, and sets the
`reconciled` flag for the health server's readiness callback.

Also create condition helper functions in `internal/controller/conditions.go`.

**Acceptance Criteria:**
1. New GPUCluster transitions from empty/Pending to Discovering phase.
2. Discovering condition set to True with reason "DiscoveryInProgress".
3. After discovery: status populated with CPU, GPU, memory, hostname.
4. On success: phase set to Ready, condition Ready=True with reason
   "ReconcileSucceeded", Discovering=False.
5. On discovery failure: phase set to Error, Ready=False with reason
   "DiscoveryFailed" and error message in condition message.
6. On failure: requeue with exponential backoff (5s base, 5m max).
7. Kubernetes events recorded: DiscoveryStarted, ReconcileSucceeded,
   DiscoveryFailed.
8. `reconciled` atomic flag set to true after first success.
9. Deleted resource returns empty result (no requeue).
10. Periodic re-reconciliation every 5 minutes for hardware refresh.
11. Recovery from Error to Ready on successful rediscovery.

**Files to Create:**
- `internal/controller/reconciler.go`
- `internal/controller/conditions.go`
- `tests/unit/controller_test.go`

**Definition of Done:**
- Unit tests (from test plan section 2.2) all pass:
  - Happy path: 5 tests (SetsDiscovering, PopulatesStatus, SetsReady,
    SetsLastTransitionTime, DiscoveringSetToFalse)
  - Status population: 4 tests (CPUInfo, GPUInfo, Memory, Hostname)
  - Error handling: 5 tests (ErrorPhase, ReadyFalse, IncludesMessage,
    Requeues, ResourceDeleted)
  - Condition transitions: 4 tests (PendingToDiscovering, DiscoveringToReady,
    DiscoveringToError, ErrorToReady)
- Test coverage >= 80% for controller package.

---

### IDEAL-304: Implement node labeling

**Type:** Task
**Epic:** E3 -- Operator Core
**Priority:** P0
**Size:** S (Small)
**Sprint:** 1
**Blocked By:** IDEAL-303
**Story Ref:** IDEAL-302 (S3.2)

**Description:**
Implement node labeling logic in `internal/controller/labels.go`. After
successful device discovery, the reconciler patches the k3s node with GPU
metadata labels using the `idealab.io/` prefix. Labels use `client.MergeFrom`
to avoid overwriting existing non-idealab labels.

**Acceptance Criteria:**
1. Node labeled with: `idealab.io/gpu-model`, `idealab.io/gpu-vram-mb`,
   `idealab.io/gpu-driver`, `idealab.io/gpu-cuda`, `idealab.io/gpu-compute`.
2. GPU model name sanitized: spaces replaced with dashes.
3. Labels applied via `client.MergeFrom` patch (no overwrite of other labels).
4. Pre-existing non-idealab labels on the node are preserved.
5. Node labeling failure logs a warning but does not prevent Ready status
   (if discovery itself succeeded).

**Files to Create:**
- `internal/controller/labels.go`
- Test cases added to `tests/unit/controller_test.go`

**Definition of Done:**
- Unit tests: `TestReconcile_LabelsNode`, `TestReconcile_LabelsSanitized`,
  `TestReconcile_LabelUpdate_NoOverwrite` all pass.
- Test coverage >= 80% for labels.go.

---

### IDEAL-305: Implement operator main entry point

**Type:** Task
**Epic:** E3 -- Operator Core
**Priority:** P0
**Size:** M (Medium)
**Sprint:** 1
**Blocked By:** IDEAL-301, IDEAL-302, IDEAL-303
**Story Ref:** IDEAL-302 (S3.2), IDEAL-304 (S3.4)

**Description:**
Implement the operator bootstrap sequence in `cmd/operator/main.go` per
technical design section 3.5. Parse environment variables, initialize logging,
create the controller-runtime Manager, wire the reconciler and health server,
and start the manager.

**Acceptance Criteria:**
1. Parses `LOG_LEVEL`, `LOG_FORMAT`, `HEALTH_PORT`, `MOCK_DISCOVERY` from
   environment variables with documented defaults.
2. Initializes `slog` logger with JSON handler and configured level.
3. Creates controller-runtime Manager with:
   - Scheme registering `api/v1alpha1` and core v1 types.
   - Leader election disabled.
   - Health probe bind address disabled (uses custom health server).
4. Creates `NVMLDiscoverer` (or `MockDiscoverer` if `MOCK_DISCOVERY=true`).
5. Registers `GPUClusterReconciler` with Manager.
6. Starts health server in background goroutine.
7. Starts Manager (blocking).
8. Exits with non-zero code on fatal error.

**Files to Create/Modify:**
- `cmd/operator/main.go`

**Definition of Done:**
- `go build ./cmd/operator/` compiles.
- Integration test with envtest validates startup sequence.
- Environment variable defaults are correct.
- `MOCK_DISCOVERY=true` uses MockDiscoverer.

---

### IDEAL-306: Integration tests with envtest

**Type:** Task
**Epic:** E3 -- Operator Core
**Priority:** P0
**Size:** M (Medium)
**Sprint:** 1
**Blocked By:** IDEAL-303, IDEAL-304, IDEAL-305
**Story Ref:** IDEAL-301 (S3.1), IDEAL-302 (S3.2), IDEAL-303 (S3.3)

**Description:**
Create integration tests using `controller-runtime/pkg/envtest` that start a
real API server and etcd, install the CRD, register the reconciler with
MockDiscoverer, and verify end-to-end reconciliation behavior.

**Acceptance Criteria:**
1. CRD CRUD: CRD installed and Established, GPUCluster can be created,
   invalid spec rejected, status is a subresource.
2. Reconciliation: Creating GPUCluster triggers discovery, status populated
   within 30 seconds, conditions transition correctly (Discovering -> Ready).
3. Error recovery: MockDiscoverer fails first, succeeds second -- Error phase
   then Ready phase.
4. Node labeling: Node gets `idealab.io/*` labels after reconciliation.
5. Events: DiscoveryStarted, ReconcileSucceeded, DiscoveryFailed events recorded.
6. Setup/teardown: envtest Environment starts and stops cleanly, no goroutine
   leaks (goleak).

**Files to Create:**
- `tests/integration/controller_integration_test.go`

**Definition of Done:**
- All 13 integration test cases from test plan section 3 pass.
- Tests compile and run with `go test -tags=integration ./tests/integration/`.
- No goroutine leaks detected by goleak.
- envtest binaries documented in Makefile setup target.

---

## Dependency Graph

```
IDEAL-010 (Go module init)
  |
  +---> IDEAL-101 (Pre-install script update)
  |
  +---> IDEAL-201 (Discovery interfaces + types)
  |       |
  |       +---> IDEAL-202 (CPU discovery)
  |       +---> IDEAL-203 (GPU discovery via NVML)
  |       +---> IDEAL-204 (Memory discovery)
  |               |
  |               +---> IDEAL-205 (Integrate NVMLDiscoverer)
  |                       |
  |                       +---> IDEAL-303 (GPUCluster reconciler) ----+
  |                                |                                  |
  +---> IDEAL-301 (CRD Go types) --+                                  |
  |                                |                                  |
  |                                +---> IDEAL-304 (Node labeling)    |
  |                                |                                  |
  +---> IDEAL-302 (Health server) --+---> IDEAL-305 (Operator main)   |
                                   |                                  |
                                   +---> IDEAL-306 (Integration tests)|
                                          ^                           |
                                          +---------------------------+
```

---

## Implementation Order (Recommended)

The following sequence respects all dependency edges and groups work for
efficient TDD sessions:

```
Phase 1 -- Foundation (no deps):
  1. IDEAL-010  Initialize Go module

Phase 2 -- Parallel tracks (depend only on IDEAL-010):
  Track A: Pre-install
    2. IDEAL-101  Update pre-install script

  Track B: Discovery types
    3. IDEAL-201  Discovery interfaces and types

  Track C: Operator types + health
    4. IDEAL-301  CRD Go types
    5. IDEAL-302  Health server

Phase 3 -- Discovery implementations (depend on IDEAL-201):
  6. IDEAL-202  CPU discovery
  7. IDEAL-203  GPU discovery via NVML
  8. IDEAL-204  Memory discovery

Phase 4 -- Assembly (depends on phases 2-3):
  9. IDEAL-205  Integrate NVMLDiscoverer

Phase 5 -- Controller (depends on IDEAL-301, IDEAL-205):
  10. IDEAL-303  GPUCluster reconciler
  11. IDEAL-304  Node labeling

Phase 6 -- Wiring + integration (depends on phase 5):
  12. IDEAL-305  Operator main entry point
  13. IDEAL-306  Integration tests with envtest
```
