# Test Plan: idealab GPU Operator -- Sprint 1

**Version:** 2.0
**Date:** 2026-03-01
**Author:** Engineer
**Status:** Updated
**JIRA Key:** IDEAL
**Sprint:** 1 (E1 + E2 + E3) + 2 (E4 + E5)

---

## 1. Overview

This test plan maps every acceptance criterion from Sprint 1 epics (E1, E2, E3)
to concrete test cases across three tiers: unit, integration, and E2E. Each
test case references the story and AC it verifies.

### 1.1 Testing Principles

- **TDD is non-negotiable.** Every test is written before the code it validates.
- **Order:** Pure functions (discovery structs) -> services (controller logic) ->
  integration (envtest) -> E2E (real or simulated cluster).
- **No GPU required for unit or integration tests.** MockDiscoverer substitutes
  for real NVML. E2E tests require GPU hardware or are skipped.

### 1.2 Coverage Targets

| Tier | Line Coverage | Branch Coverage | Scope |
|------|--------------|-----------------|-------|
| Unit | >= 80% | >= 70% | All new packages |
| Integration | All happy paths + primary error paths | -- | Controller + CRD |
| E2E | All P0 acceptance criteria | -- | Full stack |

### 1.3 Test File Locations

```
tests/
  unit/
    discovery_test.go
    controller_test.go
    health_test.go
    types_test.go
  integration/
    controller_integration_test.go
  e2e/
    preinstall_test.go
    operator_test.go
```

**Note:** Go convention allows test files co-located with source
(`internal/discovery/discovery_test.go`). The `tests/` directory is used here
for cross-package test organization. Individual packages may also contain
`_test.go` files for package-internal tests. Both patterns are acceptable; the
key requirement is that all AC-mapped tests exist.

---

## 2. Unit Tests

### 2.1 Discovery Module (`tests/unit/discovery_test.go`)

Tests use `MockDiscoverer` and direct struct construction. No NVML dependency.

#### 2.1.1 CPU Enumeration

| Test Case | AC | Description | Input | Expected Output |
|-----------|----|-------------|-------|-----------------|
| `TestCPUInfo_ModelAndTopology` | IDEAL-201 AC1 | Verify CPUInfo struct populates model, cores, threads, architecture | Mock CPU data: "Intel i5-9300H", 4 cores, 8 threads, "x86_64" | All fields match input |
| `TestCPUInfo_FeatureFlags` | IDEAL-201 AC2 | Verify CPU features list is populated from system data | Mock features: ["SSE4.2", "AVX", "AVX2"] | Features slice contains all entries |
| `TestCPUInfo_EmptyFeatures` | IDEAL-201 AC2 | Verify zero features does not cause error | Mock features: [] | Empty slice, no error |

#### 2.1.2 GPU Enumeration

| Test Case | AC | Description | Input | Expected Output |
|-----------|----|-------------|-------|-----------------|
| `TestGPUInfo_AllProperties` | IDEAL-202 AC1 | Verify all GPU fields populate correctly | Mock: "GTX 1660 Ti", UUID, 6144 MB, "560.35.03", "12.6", "7.5" | All fields match |
| `TestGPUInfo_MultipleGPUs` | IDEAL-202 AC2 | Verify multiple GPUs have unique UUIDs | Mock: 2 GPUs with different UUIDs | GPUs slice has 2 entries, UUIDs differ |
| `TestGPUInfo_SingleGPU` | IDEAL-202 AC2 | Verify single GPU case (primary target) | Mock: 1 GPU | GPUs slice has 1 entry |
| `TestGPUInfo_NotSupportedDegradation` | IDEAL-202 AC3 | Verify NOT_SUPPORTED fields get zero values | Mock: temperature=NOT_SUPPORTED, utilization=NOT_SUPPORTED | Temperature=0, Utilization=0, no error |
| `TestGPUInfo_NotSupportedLogging` | IDEAL-202 AC3 | Verify debug log emitted for NOT_SUPPORTED | Mock: ECC returns NOT_SUPPORTED | Log buffer contains debug message with function name |
| `TestGPUInfo_RequiredFieldFailure` | IDEAL-202 AC1 | Verify error when required field (name, UUID, VRAM) fails | Mock: GetName returns ERROR_UNKNOWN | Discover() returns error |

#### 2.1.3 Memory Enumeration

| Test Case | AC | Description | Input | Expected Output |
|-----------|----|-------------|-------|-----------------|
| `TestMemoryInfo_TotalAndAvailable` | IDEAL-203 AC1 | Verify total and available memory populate | Mock: total=16384 MB, available=12000 MB | Both fields match |
| `TestMemoryInfo_ConsistencyWithProcMeminfo` | IDEAL-203 AC2 | Verify values within 5% of /proc/meminfo | Read /proc/meminfo, compare with discovery output | Delta <= 5% of /proc/meminfo total |

#### 2.1.4 DeviceInfo Struct

| Test Case | AC | Description | Input | Expected Output |
|-----------|----|-------------|-------|-----------------|
| `TestDeviceInfo_MatchesCRDSchema` | IDEAL-204 AC1 | Verify DeviceInfo fields map to CRD status fields | Full mock DeviceInfo | JSON serialization matches expected CRD status field names |
| `TestDeviceInfo_Serialization` | IDEAL-204 AC1 | Verify DeviceInfo can round-trip through JSON | Full mock DeviceInfo | Marshal -> unmarshal produces identical struct |
| `TestDeviceInfo_CompletionTime` | IDEAL-204 AC2 | Verify mock discovery completes in < 5s | MockDiscoverer | Duration < 5 seconds |

#### 2.1.5 MockDiscoverer

| Test Case | AC | Description | Input | Expected Output |
|-----------|----|-------------|-------|-----------------|
| `TestMockDiscoverer_ReturnsConfiguredData` | -- | Verify mock returns pre-set DeviceInfo | Configured mock | Exact DeviceInfo returned |
| `TestMockDiscoverer_ReturnsError` | -- | Verify mock can simulate failure | Mock configured with error | Discover() returns error |

---

### 2.2 Controller Module (`tests/unit/controller_test.go`)

Tests use a fake Kubernetes client (controller-runtime `fake.NewClientBuilder`)
and `MockDiscoverer`. No real API server.

#### 2.2.1 Reconciliation Happy Path

| Test Case | AC | Description | Setup | Expected |
|-----------|----|-------------|-------|----------|
| `TestReconcile_NewResource_SetsDiscovering` | IDEAL-303 AC1 | New GPUCluster transitions to Discovering | Create GPUCluster with empty status | status.phase="Discovering", condition Discovering=True |
| `TestReconcile_Discovery_PopulatesStatus` | IDEAL-302 AC1 | Discovery data flows into status | GPUCluster + mock discovery returning full DeviceInfo | status.node.cpu, gpu, memory populated |
| `TestReconcile_Success_SetsReady` | IDEAL-303 AC2 | Successful reconcile sets Ready=True | GPUCluster + successful mock discovery | status.phase="Ready", condition Ready=True, reason="ReconcileSucceeded" |
| `TestReconcile_SetsLastTransitionTime` | IDEAL-303 AC2 | Ready condition has lastTransitionTime | GPUCluster + successful reconcile | condition.lastTransitionTime is non-zero and recent |
| `TestReconcile_DiscoveringSetToFalse` | IDEAL-303 AC1 | Discovering=False after discovery completes | GPUCluster after full reconcile | condition Discovering=False |

#### 2.2.2 Status Population

| Test Case | AC | Description | Setup | Expected |
|-----------|----|-------------|-------|----------|
| `TestReconcile_CPUInfoInStatus` | IDEAL-302 AC1 | CPU info maps to status correctly | Mock: model="i5-9300H", cores=4, threads=8 | status.node.cpu matches |
| `TestReconcile_GPUInfoInStatus` | IDEAL-302 AC2 | GPU info maps to status correctly | Mock: model="GTX 1660 Ti", vram=6144, driver="560.35.03" | status.node.gpu matches |
| `TestReconcile_MemoryInStatus` | IDEAL-302 AC1 | Memory maps to status | Mock: totalMB=16384 | status.node.memory.totalMB=16384 |
| `TestReconcile_HostnameInStatus` | IDEAL-302 AC1 | Hostname populated | Mock: hostname="gaming-pc" | status.node.hostname="gaming-pc" |

#### 2.2.3 Error Handling

| Test Case | AC | Description | Setup | Expected |
|-----------|----|-------------|-------|----------|
| `TestReconcile_DiscoveryError_SetsErrorPhase` | IDEAL-303 AC3 | Discovery failure sets Error phase | Mock returns error "NVML init failed" | status.phase="Error" |
| `TestReconcile_DiscoveryError_SetsReadyFalse` | IDEAL-303 AC3 | Discovery failure sets Ready=False | Mock returns error | condition Ready=False, reason="DiscoveryFailed" |
| `TestReconcile_DiscoveryError_IncludesMessage` | IDEAL-303 AC3 | Error message appears in condition | Mock returns error "NVML init failed" | condition.message contains "NVML init failed" |
| `TestReconcile_DiscoveryError_Requeues` | IDEAL-303 AC3 | Discovery failure requeues with backoff | Mock returns error | Result.RequeueAfter > 0 |
| `TestReconcile_ResourceDeleted_NoRequeue` | -- | Deleted resource does not requeue | No GPUCluster exists for the name | Result is empty (no requeue) |

#### 2.2.4 Node Labeling

| Test Case | AC | Description | Setup | Expected |
|-----------|----|-------------|-------|----------|
| `TestReconcile_LabelsNode` | IDEAL-302 AC1 | Node is labeled with GPU metadata | GPUCluster + mock discovery + fake Node | Node has labels: idealab.io/gpu-model, gpu-vram-mb, etc. |
| `TestReconcile_LabelsSanitized` | IDEAL-302 AC1 | Model name sanitized for label value | Model="NVIDIA GeForce GTX 1660 Ti" | Label value="NVIDIA-GeForce-GTX-1660-Ti" |
| `TestReconcile_LabelUpdate_NoOverwrite` | -- | Existing non-idealab labels preserved | Node with pre-existing labels | Pre-existing labels unchanged, idealab labels added |

#### 2.2.5 Condition Transitions

| Test Case | AC | Description | Setup | Expected |
|-----------|----|-------------|-------|----------|
| `TestConditions_PendingToDiscovering` | IDEAL-303 AC1 | Condition transition from no conditions to Discovering | New GPUCluster | Discovering=True added |
| `TestConditions_DiscoveringToReady` | IDEAL-303 AC2 | Condition transition after successful discovery | GPUCluster in Discovering phase | Discovering=False, Ready=True |
| `TestConditions_DiscoveringToError` | IDEAL-303 AC3 | Condition transition on failure | GPUCluster in Discovering + discovery error | Discovering=False, Ready=False |
| `TestConditions_ErrorToReady` | IDEAL-303 AC2 | Recovery from Error to Ready | GPUCluster in Error + successful rediscovery | Ready=True, phase="Ready" |

---

### 2.3 Health Module (`tests/unit/health_test.go`)

Tests use `net/http/httptest` for HTTP assertions. No external dependencies.

| Test Case | AC | Description | Setup | Expected |
|-----------|----|-------------|-------|----------|
| `TestHealthz_Returns200` | IDEAL-304 AC1 | /healthz always returns 200 | Start health server | GET /healthz -> 200, body contains "ok" |
| `TestHealthz_ResponseBody` | IDEAL-304 AC1 | /healthz response body is valid JSON | Start health server | Response body parses as JSON with status field |
| `TestReadyz_Returns200_WhenReady` | IDEAL-304 AC2 | /readyz returns 200 when reconciled | Ready callback returns true | GET /readyz -> 200 |
| `TestReadyz_Returns503_WhenNotReady` | IDEAL-304 AC2 | /readyz returns 503 before first reconcile | Ready callback returns false | GET /readyz -> 503 |
| `TestReadyz_TransitionsToReady` | IDEAL-304 AC2 | /readyz transitions from 503 to 200 | Ready callback changes from false to true | First call -> 503, second call -> 200 |
| `TestHealthServer_ConfigurablePort` | -- | Port is configurable | Port=9999 | Server listens on 9999 |
| `TestHealthServer_InvalidPort_Error` | -- | Invalid port returns error | Port=0 | Error on start |

---

### 2.4 CRD Types (`tests/unit/types_test.go`)

Tests verify Go type definitions match the CRD YAML schema.

| Test Case | AC | Description | Setup | Expected |
|-----------|----|-------------|-------|----------|
| `TestGPUCluster_JSONTags` | IDEAL-301 AC2 | JSON tags produce correct field names | Marshal GPUCluster to JSON | JSON keys match CRD YAML property names |
| `TestGPUCluster_StatusFields` | IDEAL-301 AC2 | Status struct has all CRD status fields | Inspect GPUClusterStatus fields | Fields: phase, node (cpu, gpu, memory), conditions |
| `TestGPUCluster_SpecFields` | IDEAL-301 AC2 | Spec struct has all CRD spec fields | Inspect GPUClusterSpec fields | Fields: driver, devicePlugin, gpuFeatureDiscovery, applicationProfiles |
| `TestGPUCluster_PhaseEnum` | IDEAL-301 AC2 | Phase values match CRD enum | Set each phase value | Pending, Discovering, Ready, Error are valid |
| `TestGPUCluster_DeepCopy` | IDEAL-301 AC2 | DeepCopy produces independent copy | Create GPUCluster, deep copy, modify original | Copy is unaffected by modification |
| `TestGPUCluster_SchemeRegistration` | IDEAL-301 AC1 | Types register with runtime.Scheme | Register and look up GVK | GVK resolves to GPUCluster type |
| `TestGPUCluster_GroupVersion` | IDEAL-301 AC1 | GroupVersion is idealab.io/v1alpha1 | Read GroupVersion constant | Group="idealab.io", Version="v1alpha1" |

---

### 2.5 Config Module (`internal/config/*_test.go`)

Tests are pure functions — no K8s deps.

#### 2.5.1 Merge

| Test Case | AC | Description | Input | Expected |
|-----------|----|-------------|-------|----------|
| `TestMergeMaps_FlatOverride` | IDEAL-402 AC2 | Src values override dst | dst={a:old}, src={a:new} | a=new |
| `TestMergeMaps_DeepMerge` | IDEAL-402 AC2 | Nested maps merge recursively | Nested dst+src | Merged with all keys |
| `TestMergeMaps_NilDst` | -- | Nil dst creates new map | nil dst, src={k:v} | {k:v} |
| `TestMergeMaps_NilSrc` | -- | Nil src preserves dst | dst={k:v}, nil src | {k:v} |

#### 2.5.2 Values Generation

| Test Case | AC | Description | Input | Expected |
|-----------|----|-------------|-------|----------|
| `TestGenerateValues_HardwareDefaults` | IDEAL-402 AC1 | GPU/CPU/memory defaults populated | Profile + GTX 1660 Ti hardware | gpu.model, gpu.vramMB (minus 512 reserve), resources.limits set |
| `TestGenerateValues_UserOverrides` | IDEAL-402 AC2 | User HelmValues win over defaults | Profile with overrides | User values override hardware defaults |
| `TestGenerateValues_NoGPU` | IDEAL-402 AC1 | CPU-only hardware skips GPU section | No GPU in hardware | No gpu key, no nvidia.com/gpu limit |
| `TestGenerateValues_ProfileResourceLimits` | IDEAL-402 AC1 | Profile CPU/memory limits used | Profile with explicit limits | Limits match profile values |
| `TestGenerateValues_VRAMReserve` | IDEAL-402 AC1 | VRAM below reserve clamps to 0 | 256MB VRAM GPU | vramMB=0 |
| `TestParseGPUMemoryMB` | IDEAL-402 AC3 | Memory string parsing (Gi, Mi, raw) | "4Gi", "2048Mi", "" | 4096, 2048, 0 |

#### 2.5.3 Validation

| Test Case | AC | Description | Input | Expected |
|-----------|----|-------------|-------|----------|
| `TestCheckResourceOvercommit_UnderLimit` | IDEAL-402 AC3 | No warning when within limit | 3GB requested, 6GB available | Empty string |
| `TestCheckResourceOvercommit_OverLimit` | IDEAL-402 AC3 | Warning when exceeding limit | 7GB requested, 6GB available | Non-empty warning with profile names |
| `TestCheckResourceOvercommit_NoGPUMemorySet` | -- | No warning for CPU-only profiles | No gpuMemory set | Empty string |
| `TestCheckResourceOvercommit_ZeroAvailable` | -- | No warning with zero VRAM | 0 available | Empty string |

#### 2.5.4 Render

| Test Case | AC | Description | Input | Expected |
|-----------|----|-------------|-------|----------|
| `TestRenderYAML_Simple` | IDEAL-403 AC1 | Simple map to YAML | {key: value, num: 42} | Contains "key: value" |
| `TestRenderYAML_Nested` | IDEAL-403 AC1 | Nested map to YAML | {parent: {child: val}} | Valid nested YAML |
| `TestRenderYAML_Empty` | -- | Empty map renders as {} | {} | "{}\n" |

### 2.6 ConfigMap Reconciliation (`internal/controller/configmaps_test.go`)

Tests use `fake.NewClientBuilder()` with scheme.

| Test Case | AC | Description | Setup | Expected |
|-----------|----|-------------|-------|----------|
| `TestReconcileConfigMaps_Create` | IDEAL-403 AC1 | ConfigMap created per profile | 1 profile, no existing CMs | CM created with correct name, labels, data |
| `TestReconcileConfigMaps_Update` | IDEAL-403 AC2 | ConfigMap updated on change | Existing CM with old data | CM data updated |
| `TestReconcileConfigMaps_MultiProfile` | IDEAL-403 AC1 | Multiple profiles create multiple CMs | 2 profiles | 2 CMs created, 2 profile statuses |
| `TestReconcileConfigMaps_OrphanCleanup` | IDEAL-403 AC2 | Removed profiles delete orphan CMs | 1 profile + 1 orphan CM | Orphan deleted |
| `TestCheckResourceWarning` | IDEAL-402 AC3 | ResourceWarning set on overcommit | 2 profiles exceeding 6GB | status.resourceWarning non-empty |
| `TestConfigMapName` | -- | Naming convention | gc=my-cluster, profile=ollama | "my-cluster-ollama-values" |
| `TestBuildHardwareInfo` | -- | Discovery to HardwareInfo mapping | Mock DeviceInfo | All fields mapped |
| `TestBuildHardwareInfo_NoGPU` | -- | No GPU case | No GPUs | Empty GPU fields |

### 2.7 Finalizer (`internal/controller/finalizer_test.go`)

| Test Case | AC | Description | Setup | Expected |
|-----------|----|-------------|-------|----------|
| `TestEnsureFinalizer_Add` | IDEAL-403 AC1 | Finalizer added on first reconcile | GPUCluster without finalizer | Finalizer present |
| `TestEnsureFinalizer_AlreadyPresent` | -- | Nop when already present | GPUCluster with finalizer | No error, no change |
| `TestHandleDeletion_NotDeleting` | -- | Returns false for non-deleted resource | No DeletionTimestamp | deleting=false |
| `TestHandleDeletion_CleansUpConfigMaps` | IDEAL-403 AC1 | Deletes CMs and removes finalizer | Deleted GPUCluster + CM | CM deleted, finalizer removed |

### 2.8 Metrics (`internal/metrics/metrics_test.go`, `internal/controller/metrics_test.go`)

| Test Case | AC | Description | Setup | Expected |
|-----------|----|-------------|-------|----------|
| `TestMetricsRegistration` | E5 | All metrics register without error | Fresh prometheus.Registry | No registration errors |
| `TestGPUGaugesSetValues` | E5 | GPU gauges accept label values | Set temp, util, VRAM | No panic |
| `TestCounterIncrement` | E5 | Reconcile counter increments | Inc success/error | No panic |
| `TestConfigMapsGauge` | E5 | ConfigMaps gauge accepts value | Set(3) | No panic |
| `TestRecordGPUMetrics` | E5 | Controller helper records GPU data | Mock DeviceInfo | No panic |
| `TestRecordGPUMetrics_NoGPU` | E5 | No panic with empty GPUs | No GPUs | No panic |
| `TestRecordConfigMapCount` | E5 | ConfigMap count set | Set(3), Set(0) | No panic |

---

## 3. Integration Tests

### 3.1 Controller + envtest (`tests/integration/controller_integration_test.go`)

Integration tests use `sigs.k8s.io/controller-runtime/pkg/envtest` which starts
a real API server and etcd. The controller runs against this real API server with
`MockDiscoverer` injected.

#### 3.1.1 Setup/Teardown

```
TestMain (or suite setup):
  1. Start envtest Environment with CRD paths = ["deploy/crds/"]
  2. Register v1alpha1 scheme
  3. Create Manager with envtest config
  4. Register GPUClusterReconciler with MockDiscoverer
  5. Start Manager in background goroutine
  6. Create k8s client for test assertions

After all tests:
  1. Stop Manager
  2. Stop envtest Environment
```

#### 3.1.2 CRD CRUD Operations

| Test Case | AC | Description | Steps | Expected |
|-----------|----|-------------|-------|----------|
| `TestIntegration_CRDInstalled` | IDEAL-301 AC1 | CRD is registered in envtest API server | List CRDs | gpuclusters.idealab.io exists, status=Established |
| `TestIntegration_CreateGPUCluster` | IDEAL-301 AC1 | GPUCluster can be created | kubectl apply equivalent | Resource created, no error |
| `TestIntegration_CreateWithInvalidSpec` | IDEAL-301 AC3 | Invalid spec is rejected | Create GPUCluster with invalid phase | API server returns validation error |
| `TestIntegration_StatusIsSubresource` | IDEAL-301 AC2 | Status updates do not require spec changes | Create GPUCluster, update only status | Status updated, spec unchanged |

#### 3.1.3 End-to-End Reconciliation (with mock)

| Test Case | AC | Description | Steps | Expected |
|-----------|----|-------------|-------|----------|
| `TestIntegration_Reconcile_PopulatesStatus` | IDEAL-302 AC1 | Creating GPUCluster triggers discovery and status population | Create GPUCluster, wait up to 30s | Status has cpu, gpu, memory data from mock |
| `TestIntegration_Reconcile_Within30Seconds` | IDEAL-302 AC1 | Reconciliation completes within 30 seconds | Create GPUCluster, poll status | status.phase="Ready" within 30s |
| `TestIntegration_Reconcile_ConditionTransitions` | IDEAL-303 AC1, AC2 | Conditions transition correctly | Create GPUCluster, observe conditions | Discovering=True then Ready=True |
| `TestIntegration_Reconcile_ErrorRecovery` | IDEAL-303 AC3 | Error condition set, then recovery on retry | MockDiscoverer: fail first, succeed second | Error phase then Ready phase |
| `TestIntegration_Reconcile_NodeLabeled` | IDEAL-302 AC1 | Node gets GPU labels after reconciliation | Create GPUCluster + fake Node | Node has idealab.io/* labels |

#### 3.1.4 Event Recording

| Test Case | AC | Description | Steps | Expected |
|-----------|----|-------------|-------|----------|
| `TestIntegration_Event_DiscoveryStarted` | IDEAL-303 AC1 | Event recorded when discovery starts | Create GPUCluster | Event with reason "DiscoveryStarted" |
| `TestIntegration_Event_ReconcileSucceeded` | IDEAL-303 AC2 | Event recorded on success | Create GPUCluster, wait for Ready | Event with reason "ReconcileSucceeded" |
| `TestIntegration_Event_DiscoveryFailed` | IDEAL-303 AC3 | Event recorded on failure | MockDiscoverer returns error | Event with reason "DiscoveryFailed" |

---

## 4. E2E Tests

### 4.1 Pre-Install Script (`tests/e2e/preinstall_test.go`)

E2E tests for the pre-install script run in a Docker container simulating a
fresh Ubuntu installation. These tests validate the script's behavior without
requiring a real GPU (some tests verify error handling for missing GPU).

**Test environment:** Docker container based on `ubuntu:22.04` or `ubuntu:24.04`
with `lspci` output simulated via a mock PCI database.

| Test Case | AC | Description | Setup | Expected |
|-----------|----|-------------|-------|----------|
| `TestPreinstall_AllComponentsInstalled` | IDEAL-101 AC1 | Script installs all components in order | Fresh Ubuntu container with mock GPU PCI entry | Each component installed in correct order: drivers, toolkit, k3s, RuntimeClass |
| `TestPreinstall_Idempotent` | IDEAL-101 AC2 | Script runs twice without error | Run script twice in same container | Second run completes without errors, no duplicates |
| `TestPreinstall_ValidatesEachStep` | IDEAL-101 AC3 | Script validates before proceeding | Container with simulated failure at toolkit step | Script exits with non-zero code and clear error message |
| `TestPreinstall_DetectsNvidiaGPU` | IDEAL-102 AC1 | Script detects NVIDIA GPU by PCI ID | Container with NVIDIA PCI entry in lspci | stdout contains "Detected: NVIDIA GeForce GTX 1660 Ti" |
| `TestPreinstall_NoGPU_Fails` | IDEAL-102 AC1 | Script fails if no NVIDIA GPU detected | Container without NVIDIA PCI entry | Exit 1 with "No NVIDIA GPU detected" |
| `TestPreinstall_SkipsExistingDriver` | IDEAL-102 AC3 | Script skips driver if compatible exists | Container with mock nvidia-smi returning "560.35.03" | stdout contains "driver already installed" |
| `TestPreinstall_InstallsCorrectDriver` | IDEAL-102 AC2 | Script installs driver >= 560 | Container without existing driver | apt-get install called for nvidia-driver-560 or later |
| `TestPreinstall_K3sRunning` | IDEAL-103 AC1 | k3s is installed and node Ready | Full script execution in privileged container | k3s kubectl get nodes shows Ready |
| `TestPreinstall_ContainerdConfigured` | IDEAL-103 AC2 | containerd configured for NVIDIA runtime | Full script execution | config.toml at k3s containerd path contains nvidia handler, RuntimeClass exists |
| `TestPreinstall_GPUTestPod` | IDEAL-103 AC3 | Test pod runs nvidia-smi successfully | Full script execution on GPU hardware | Test pod exits 0, cleaned up after validation |

**Note:** Tests marked "Full script execution" require either real GPU hardware
or are tagged `//go:build e2e && gpu` for conditional execution. Tests that
only verify script logic (idempotency, detection) can run without a GPU.

### 4.2 Full Operator (`tests/e2e/operator_test.go`)

E2E tests deploy the operator to a real or simulated k3s cluster and verify
end-to-end behavior. Uses `kind` (Kubernetes in Docker) with k3s image, or
a real k3s node.

**Test environment options:**
1. **kind cluster** with NVIDIA GPU support (requires host GPU).
2. **Real k3s node** (the target gaming PC).
3. **kind cluster without GPU** + `MOCK_DISCOVERY=true` for CI.

| Test Case | AC | Description | Steps | Expected |
|-----------|----|-------------|-------|----------|
| `TestE2E_CRDDeployed` | IDEAL-301 AC1 | CRD is installed in cluster | Apply CRD YAML, verify | kubectl get crd gpuclusters.idealab.io shows Established |
| `TestE2E_OperatorRunning` | IDEAL-304 AC1 | Operator pod is running and healthy | Deploy operator, check pod status | Pod is Running, liveness probe passes |
| `TestE2E_HealthzEndpoint` | IDEAL-304 AC1 | /healthz returns 200 | Port-forward to operator pod | GET /healthz -> 200 |
| `TestE2E_ReadyzEndpoint_BeforeReconcile` | IDEAL-304 AC2 | /readyz returns 503 before any GPUCluster | Port-forward before creating GPUCluster | GET /readyz -> 503 |
| `TestE2E_CreateGPUCluster_StatusPopulated` | IDEAL-302 AC1 | Creating GPUCluster populates status | Apply GPUCluster CR, wait 30s | status.phase="Ready", status.node populated |
| `TestE2E_GPUInfoMatchesNvidiaSmi` | IDEAL-302 AC2 | GPU status matches nvidia-smi output | Compare status.node.gpu with nvidia-smi output | Model, driver version, CUDA version match |
| `TestE2E_ReadyzEndpoint_AfterReconcile` | IDEAL-304 AC2 | /readyz returns 200 after reconciliation | Port-forward after GPUCluster reconciled | GET /readyz -> 200 |
| `TestE2E_ConditionsSet` | IDEAL-303 AC2 | Ready condition is True after reconciliation | Get GPUCluster, inspect conditions | Ready=True, Discovering=False |
| `TestE2E_ReconcileWithin30Seconds` | IDEAL-302 AC1 | Status populated within 30 seconds | Create GPUCluster, poll every 2s for 30s | phase="Ready" within 30s |
| `TestE2E_ErrorCondition_NoGPU` | IDEAL-303 AC3 | Error condition when GPU unavailable | Deploy operator on node without GPU (no mock) | phase="Error", Ready=False, reason="DiscoveryFailed" |

---

## 5. Test Matrix: Acceptance Criteria Coverage

This matrix maps every AC from Sprint 1 to the test cases that verify it.
Every AC must have at least one test case. P0 ACs should have both unit and
integration (or E2E) coverage.

### 5.1 E1: Pre-Install Script

| Story | AC | Unit | Integration | E2E |
|-------|----|------|-------------|-----|
| IDEAL-101 S1.1 | AC1: Installs all components in order | -- | -- | `TestPreinstall_AllComponentsInstalled` |
| IDEAL-101 S1.1 | AC2: Idempotent execution | -- | -- | `TestPreinstall_Idempotent` |
| IDEAL-101 S1.1 | AC3: Validates each step | -- | -- | `TestPreinstall_ValidatesEachStep` |
| IDEAL-102 S1.2 | AC1: Detects NVIDIA GPU | -- | -- | `TestPreinstall_DetectsNvidiaGPU`, `TestPreinstall_NoGPU_Fails` |
| IDEAL-102 S1.2 | AC2: Installs correct driver | -- | -- | `TestPreinstall_InstallsCorrectDriver` |
| IDEAL-102 S1.2 | AC3: Skips existing driver | -- | -- | `TestPreinstall_SkipsExistingDriver` |
| IDEAL-103 S1.3 | AC1: k3s installed and Ready | -- | -- | `TestPreinstall_K3sRunning` |
| IDEAL-103 S1.3 | AC2: containerd configured | -- | -- | `TestPreinstall_ContainerdConfigured` |
| IDEAL-103 S1.3 | AC3: GPU test pod succeeds | -- | -- | `TestPreinstall_GPUTestPod` |

### 5.2 E2: Device Discovery

| Story | AC | Unit | Integration | E2E |
|-------|----|------|-------------|-----|
| IDEAL-201 S2.1 | AC1: CPU model, cores, threads, arch | `TestCPUInfo_ModelAndTopology` | `TestIntegration_Reconcile_PopulatesStatus` | `TestE2E_CreateGPUCluster_StatusPopulated` |
| IDEAL-201 S2.1 | AC2: CPU feature flags | `TestCPUInfo_FeatureFlags`, `TestCPUInfo_EmptyFeatures` | -- | -- |
| IDEAL-202 S2.2 | AC1: GPU properties via NVML | `TestGPUInfo_AllProperties`, `TestGPUInfo_RequiredFieldFailure` | `TestIntegration_Reconcile_PopulatesStatus` | `TestE2E_GPUInfoMatchesNvidiaSmi` |
| IDEAL-202 S2.2 | AC2: Multiple GPUs enumerated | `TestGPUInfo_MultipleGPUs`, `TestGPUInfo_SingleGPU` | -- | -- |
| IDEAL-202 S2.2 | AC3: NOT_SUPPORTED degradation | `TestGPUInfo_NotSupportedDegradation`, `TestGPUInfo_NotSupportedLogging` | -- | -- |
| IDEAL-203 S2.3 | AC1: Total and available memory | `TestMemoryInfo_TotalAndAvailable` | `TestIntegration_Reconcile_PopulatesStatus` | `TestE2E_CreateGPUCluster_StatusPopulated` |
| IDEAL-203 S2.3 | AC2: Memory consistent with /proc/meminfo | `TestMemoryInfo_ConsistencyWithProcMeminfo` | -- | -- |
| IDEAL-204 S2.4 | AC1: DeviceInfo matches CRD schema | `TestDeviceInfo_MatchesCRDSchema`, `TestDeviceInfo_Serialization` | -- | -- |
| IDEAL-204 S2.4 | AC2: Enumeration under 5 seconds | `TestDeviceInfo_CompletionTime` | `TestIntegration_Reconcile_Within30Seconds` | `TestE2E_ReconcileWithin30Seconds` |

### 5.3 E3: Operator Core

| Story | AC | Unit | Integration | E2E |
|-------|----|------|-------------|-----|
| IDEAL-301 S3.1 | AC1: CRD installable and Established | `TestGPUCluster_SchemeRegistration`, `TestGPUCluster_GroupVersion` | `TestIntegration_CRDInstalled`, `TestIntegration_CreateGPUCluster` | `TestE2E_CRDDeployed` |
| IDEAL-301 S3.1 | AC2: Spec and status schema complete | `TestGPUCluster_StatusFields`, `TestGPUCluster_SpecFields`, `TestGPUCluster_JSONTags`, `TestGPUCluster_PhaseEnum`, `TestGPUCluster_DeepCopy` | `TestIntegration_StatusIsSubresource` | -- |
| IDEAL-301 S3.1 | AC3: CRD enforces validation | -- | `TestIntegration_CreateWithInvalidSpec` | -- |
| IDEAL-302 S3.2 | AC1: Status populated within 30s | `TestReconcile_Discovery_PopulatesStatus`, `TestReconcile_CPUInfoInStatus`, `TestReconcile_MemoryInStatus`, `TestReconcile_HostnameInStatus` | `TestIntegration_Reconcile_PopulatesStatus`, `TestIntegration_Reconcile_Within30Seconds`, `TestIntegration_Reconcile_NodeLabeled` | `TestE2E_CreateGPUCluster_StatusPopulated`, `TestE2E_ReconcileWithin30Seconds` |
| IDEAL-302 S3.2 | AC2: GPU info matches nvidia-smi | `TestReconcile_GPUInfoInStatus` | -- | `TestE2E_GPUInfoMatchesNvidiaSmi` |
| IDEAL-303 S3.3 | AC1: Discovering condition set | `TestReconcile_NewResource_SetsDiscovering`, `TestReconcile_DiscoveringSetToFalse`, `TestConditions_PendingToDiscovering` | `TestIntegration_Reconcile_ConditionTransitions`, `TestIntegration_Event_DiscoveryStarted` | `TestE2E_ConditionsSet` |
| IDEAL-303 S3.3 | AC2: Ready condition set | `TestReconcile_Success_SetsReady`, `TestReconcile_SetsLastTransitionTime`, `TestConditions_DiscoveringToReady`, `TestConditions_ErrorToReady` | `TestIntegration_Reconcile_ConditionTransitions`, `TestIntegration_Event_ReconcileSucceeded` | `TestE2E_ConditionsSet` |
| IDEAL-303 S3.3 | AC3: Error condition on failure | `TestReconcile_DiscoveryError_SetsErrorPhase`, `TestReconcile_DiscoveryError_SetsReadyFalse`, `TestReconcile_DiscoveryError_IncludesMessage`, `TestReconcile_DiscoveryError_Requeues`, `TestConditions_DiscoveringToError` | `TestIntegration_Reconcile_ErrorRecovery`, `TestIntegration_Event_DiscoveryFailed` | `TestE2E_ErrorCondition_NoGPU` |
| IDEAL-304 S3.4 | AC1: /healthz returns 200 | `TestHealthz_Returns200`, `TestHealthz_ResponseBody` | -- | `TestE2E_HealthzEndpoint`, `TestE2E_OperatorRunning` |
| IDEAL-304 S3.4 | AC2: /readyz reflects state | `TestReadyz_Returns200_WhenReady`, `TestReadyz_Returns503_WhenNotReady`, `TestReadyz_TransitionsToReady` | -- | `TestE2E_ReadyzEndpoint_BeforeReconcile`, `TestE2E_ReadyzEndpoint_AfterReconcile` |

---

## 6. Test Infrastructure

### 6.1 Build Tags

| Tag | Purpose | Used By |
|-----|---------|---------|
| `unit` | Standard unit tests (default, no tag needed) | `go test ./...` |
| `integration` | Requires envtest (API server + etcd binaries) | `go test -tags=integration ./tests/integration/` |
| `e2e` | Requires running k3s cluster | `go test -tags=e2e ./tests/e2e/` |
| `gpu` | Requires NVIDIA GPU hardware | `go test -tags=e2e,gpu ./tests/e2e/` |

### 6.2 CI Pipeline Test Stages

```
Stage 1: Unit Tests (no dependencies)
  go test ./... -v -race -coverprofile=coverage.txt
  - Runs on any CI runner (no GPU, no k8s)
  - Uses MockDiscoverer
  - Target: < 2 minutes

Stage 2: Integration Tests (envtest)
  go test -tags=integration ./tests/integration/ -v
  - Downloads envtest binaries (API server + etcd)
  - Uses MockDiscoverer
  - Target: < 5 minutes

Stage 3: E2E Tests (requires k3s + GPU)
  go test -tags=e2e,gpu ./tests/e2e/ -v -timeout=10m
  - Runs on self-hosted runner with GPU hardware
  - OR: skipped in CI, run manually on target hardware
  - Target: < 10 minutes
```

### 6.3 Test Dependencies

| Dependency | Purpose | Install |
|-----------|---------|---------|
| `github.com/stretchr/testify` | Assertions (require, assert) | `go get` |
| `sigs.k8s.io/controller-runtime/pkg/envtest` | Integration test API server | `go get` (+ envtest binary setup) |
| `go.uber.org/goleak` | Goroutine leak detection | `go get` |
| `net/http/httptest` | HTTP test server (stdlib) | Built-in |

### 6.4 Makefile Targets

```makefile
test:           # All unit tests
test-short:     # Quick unit tests (skip slow)
test-integration: # envtest-based integration tests
test-e2e:       # E2E tests (requires cluster)
test-all:       # Unit + integration + E2E
coverage:       # Unit tests with coverage report
```

---

## 7. Test Data

### 7.1 Mock DeviceInfo (used across all unit and integration tests)

```
DeviceInfo{
  Hostname: "test-node",
  CPU: CPUInfo{
    Model:        "Intel(R) Core(TM) i5-9300H CPU @ 2.40GHz",
    Cores:        4,
    Threads:      8,
    Architecture: "x86_64",
    Features:     []string{"SSE4.2", "AVX", "AVX2"},
  },
  GPUs: []GPUInfo{{
    Model:             "NVIDIA GeForce GTX 1660 Ti",
    UUID:              "GPU-12345678-1234-1234-1234-123456789abc",
    VRAMMB:            6144,
    DriverVersion:     "560.35.03",
    CUDAVersion:       "12.6",
    ComputeCapability: "7.5",
    Temperature:       45,
    UtilizationPct:    12,
  }},
  Memory: MemoryInfo{
    TotalMB:     16384,
    AvailableMB: 12000,
  },
}
```

### 7.2 Mock GPUCluster CR (used in controller tests)

```yaml
apiVersion: idealab.io/v1alpha1
kind: GPUCluster
metadata:
  name: test-cluster
spec:
  driver:
    enabled: true
    version: "560"
  devicePlugin:
    enabled: true
  gpuFeatureDiscovery:
    enabled: true
```

### 7.3 Error Scenarios

| Scenario | MockDiscoverer Config | Expected Phase |
|----------|----------------------|---------------|
| NVML init failure | Return `errors.New("NVML initialization failed: driver not loaded")` | Error |
| No GPU found | Return `errors.New("no NVIDIA GPU devices found")` | Error |
| Partial data (temp unavailable) | Return DeviceInfo with Temperature=0 | Ready (temperature is optional) |
| API server unreachable | (controller-runtime handles) | Requeue with backoff |

---

## 8. Acceptance Criteria Summary

### 8.1 Sprint 1 Totals

| Epic | Stories | Acceptance Criteria | Unit Tests | Integration Tests | E2E Tests |
|------|---------|---------------------|-----------|-------------------|-----------|
| E1 Pre-Install | 3 | 9 | 0 | 0 | 10 |
| E2 Discovery | 4 | 9 | 13 | 1 (via reconcile) | 2 |
| E3 Operator Core | 4 | 10 | 25 | 9 | 10 |
| E4 Config Templates | 3 | 7 | 21 | 8 | 0 |
| E5 Monitoring | -- | -- | 7 | 0 | 0 |
| **Total** | **14** | **35** | **66** | **18** | **22** |

**Grand total: 106 test cases covering 35 acceptance criteria.**
**Currently passing: 53 unit tests across 6 packages.**

### 8.2 Coverage Gap Analysis

All 28 Sprint 1 acceptance criteria have at least one mapped test case. The
following criteria have extra depth:

- IDEAL-302 AC1 (status populated within 30s): Covered at all three tiers.
- IDEAL-303 AC3 (error condition): 5 unit tests + 2 integration + 1 E2E.
- IDEAL-304 AC2 (readyz reflects state): 3 unit tests + 2 E2E tests.

No gaps identified. All P0 acceptance criteria have multi-tier coverage.

### 5.4 E4: Configuration Templates

| Story | AC | Unit | Integration | E2E |
|-------|----|------|-------------|-----|
| IDEAL-401 | AC1: Profiles accepted in spec | `TestGenerateValues_HardwareDefaults` | `TestReconcileConfigMaps_Create` | -- |
| IDEAL-401 | AC2: Empty name/chart rejected | (CRD YAML `required` + `minLength` validation) | -- | -- |
| IDEAL-402 | AC1: Hardware-derived settings | `TestGenerateValues_HardwareDefaults`, `TestGenerateValues_NoGPU`, `TestGenerateValues_VRAMReserve` | `TestReconcileConfigMaps_Create` | -- |
| IDEAL-402 | AC2: User overrides win | `TestGenerateValues_UserOverrides`, `TestMergeMaps_FlatOverride`, `TestMergeMaps_DeepMerge` | -- | -- |
| IDEAL-402 | AC3: ResourceWarning on overcommit | `TestCheckResourceOvercommit_OverLimit`, `TestCheckResourceOvercommit_UnderLimit` | `TestCheckResourceWarning` | -- |
| IDEAL-403 | AC1: ConfigMap per profile | `TestRenderYAML_Simple`, `TestRenderYAML_Nested` | `TestReconcileConfigMaps_Create`, `TestReconcileConfigMaps_MultiProfile`, `TestEnsureFinalizer_Add`, `TestHandleDeletion_CleansUpConfigMaps` | -- |
| IDEAL-403 | AC2: ConfigMap updated on change | -- | `TestReconcileConfigMaps_Update`, `TestReconcileConfigMaps_OrphanCleanup` | -- |

### 5.5 E5: Monitoring

| Feature | Unit | Integration |
|---------|------|-------------|
| GPU telemetry gauges | `TestGPUGaugesSetValues`, `TestRecordGPUMetrics`, `TestRecordGPUMetrics_NoGPU` | -- |
| Reconcile counters | `TestCounterIncrement` | -- |
| ConfigMaps gauge | `TestConfigMapsGauge`, `TestRecordConfigMapCount` | -- |
| Metrics registration | `TestMetricsRegistration` | -- |

---

## References

- Technical Design: `/home/bibs/work/idealab/docs/design/technical-design.md`
- PRD: `/home/bibs/work/idealab/docs/prd.md`
- Stories: `/home/bibs/work/idealab/docs/stories/`
- Acceptance Matrix: `/home/bibs/work/idealab/docs/stories/acceptance-matrix.md`
- CRD YAML: `/home/bibs/work/idealab/deploy/crds/gpucluster.yaml`
