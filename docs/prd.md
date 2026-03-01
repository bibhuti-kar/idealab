# Product Requirements Document: idealab GPU Operator

**Version:** 1.0
**Date:** 2026-03-01
**Author:** Product Manager
**Status:** Draft
**JIRA Key:** IDEAL

---

## 1. Problem Statement

Running GPU-accelerated AI workloads on a single-node edge device (a gaming PC with
an NVIDIA GTX 1660 Ti) using Kubernetes requires assembling multiple NVIDIA
components: host drivers, container toolkit, device plugin, feature discovery, and
monitoring. The official NVIDIA GPU Operator is designed for multi-node data center
clusters and carries significant overhead -- 14+ containerized components, MIG
support, vGPU management, driver containers, and enterprise features that are
irrelevant to a single-node consumer GPU deployment.

There is no turnkey solution for a solo developer who wants to:
1. Set up k3s on a gaming PC with full GPU support.
2. Have an operator that discovers hardware, exposes GPU resources to Kubernetes,
   and generates configuration for AI workloads.
3. Deploy multiple AI services (inference, training, pipelines) with correct GPU
   resource allocation derived from actual hardware capabilities.

The gap is clear: the full GPU Operator is too heavy, and manual setup is too
error-prone and undocumented for reproducibility.

---

## 2. Target User

**Primary Persona: Solo Edge Developer**

- A developer or ML engineer who owns a single NVIDIA GPU machine (gaming PC or
  workstation) and wants to run AI workloads on Kubernetes locally.
- Comfortable with the terminal but does not want to manually wire up device
  plugins, containerd configs, and RuntimeClasses.
- Needs a reproducible setup that works after a reboot without re-doing manual
  steps.
- Runs Ubuntu Linux with a consumer-grade NVIDIA GPU (Turing or newer).

**Characteristics:**
- Single machine, single GPU, single node.
- Uses k3s (not full Kubernetes) for low overhead.
- Deploys 1-3 AI services that share the GPU via time-slicing or sequential access.
- Values simplicity and debuggability over enterprise features.

---

## 3. Solution Overview

A two-phase approach:

### Phase 1: Pre-Install Script
A single bash script that installs and validates all host-level prerequisites:
NVIDIA drivers, NVIDIA Container Toolkit (configured for k3s containerd), k3s
itself, and the nvidia RuntimeClass. The script is idempotent and validates each
step before proceeding.

### Phase 2: Kubernetes Operator
A Go-based Kubernetes operator that:
- Discovers CPU and GPU hardware via NVML and system APIs.
- Defines a `GPUCluster` CRD that captures the cluster's hardware profile and
  desired workload configuration.
- Reconciles the CRD, populating status with discovered hardware and reporting
  conditions (Ready, Error, Discovering).
- Generates Helm values files (as ConfigMaps) from the combination of hardware
  capabilities and user-defined application profiles.
- Exposes health and readiness endpoints.

---

## 4. Milestones

### M1: Pre-Install + k3s Running with GPU Support
**Goal:** A developer runs one script and ends up with k3s running, GPU schedulable,
and all prerequisites validated.

**Deliverables:**
- Pre-install bash script covering drivers, toolkit, k3s, RuntimeClass.
- Validation checks for each component.
- Idempotent execution (safe to re-run).

### M2: Operator with Device Discovery + GPUCluster CRD
**Goal:** The operator runs in k3s, discovers hardware, and manages the GPUCluster
CRD lifecycle.

**Deliverables:**
- GPUCluster CRD (api/v1alpha1).
- Device discovery module (CPU, GPU, memory via NVML + system APIs).
- Operator controller with reconciliation loop.
- Health and readiness endpoints.

### M3: Helm Chart Template Generation for AI Workloads
**Goal:** The operator generates Helm values files from hardware discovery combined
with user-defined application profiles in the GPUCluster spec.

**Deliverables:**
- Application profile schema in GPUCluster spec.
- Helm values generation logic.
- Generated ConfigMaps with rendered values.

---

## 5. Epics

| Epic | Priority | Title | Milestone |
|------|----------|-------|-----------|
| E1 | P0 | Pre-Install Script | M1 |
| E2 | P0 | Device Discovery | M2 |
| E3 | P0 | Operator Core (CRD + Controller) | M2 |
| E4 | P1 | Configuration Templates | M3 |
| E5 | P2 | Monitoring (GPU Metrics Endpoint) | M3 |

### E1: Pre-Install Script (P0)
Install NVIDIA drivers, NVIDIA Container Toolkit, k3s, and the nvidia
RuntimeClass. Validate each component. Single script, idempotent, targets Ubuntu
with consumer NVIDIA GPUs.

### E2: Device Discovery (P0)
Enumerate CPU capabilities (model, cores, threads, instruction set features), GPU
capabilities (model, VRAM, driver version, CUDA version, compute capability,
UUID) via NVML, and system memory. Output a structured `DeviceInfo` that feeds
into the GPUCluster CRD status.

### E3: Operator Core -- CRD + Controller (P0)
Define the `GPUCluster` CRD with spec (desired configuration) and status
(discovered hardware, conditions). Implement the controller reconciliation loop
using controller-runtime. Auto-populate status on creation. Report conditions:
`Ready`, `Error`, `Discovering`.

### E4: Configuration Templates (P1)
Allow users to define application profiles in GPUCluster spec with Helm chart
references, resource requirements, and environment overrides. The operator
generates Helm values files by combining hardware discovery data with these
profiles and writes the result as ConfigMaps.

### E5: Monitoring -- GPU Metrics Endpoint (P2)
Expose a `/metrics` endpoint with basic GPU telemetry: temperature, utilization
(GPU and memory), power usage, clock speeds. Prometheus-compatible format. Use
NVML directly (not DCGM, which has limited consumer GPU support).

---

## 6. Non-Goals

The following are explicitly out of scope for this project:

- **Multi-node clusters.** This operator targets a single k3s node only.
- **Multi-Instance GPU (MIG).** The GTX 1660 Ti does not support MIG. No MIG
  logic will be implemented.
- **vGPU / GPU virtualization.** Consumer GPUs do not support NVIDIA vGPU.
- **Driver containers.** Drivers are pre-installed on the host. The display GPU
  cannot have its drivers managed by a container.
- **Cloud deployment.** This is for a local gaming PC, not cloud instances.
- **Container Toolkit containers.** The toolkit is installed on the host by the
  pre-install script.
- **Node Feature Discovery (NFD).** Single node; we label it directly.
- **Kubernetes Device Plugin API.** The operator does not implement the device
  plugin gRPC interface. GPU scheduling relies on the NVIDIA device plugin
  installed via the container toolkit.
- **GPU sharing (time-slicing / MPS) management.** May be a future enhancement
  but is not part of this scope.
- **High availability or failover.** Single-node, single-replica operator.
- **Windows or macOS support.** Linux (Ubuntu) only.

---

## 7. Success Criteria

| Criterion | Verification |
|-----------|-------------|
| k3s is running with GPU support after pre-install script | `kubectl get nodes` shows Ready; `nvidia-smi` works; RuntimeClass exists |
| GPU is schedulable in k3s | A test pod requesting `nvidia.com/gpu: 1` runs successfully |
| Operator discovers hardware correctly | GPUCluster status contains accurate GPU model, VRAM, CUDA version, CPU info, memory |
| GPUCluster CRD reconciles | Creating a GPUCluster CR triggers discovery and status population within 30 seconds |
| Operator reports conditions | GPUCluster status.conditions includes Ready=True after successful reconciliation |
| Helm values are generated | ConfigMap with generated Helm values exists and contains hardware-derived values |
| Operator health endpoints work | `/healthz` returns 200; `/readyz` returns 200 when reconciled |
| Pre-install script is idempotent | Running the script twice produces no errors and no duplicate installations |

---

## 8. Constraints

- **Hardware:** NVIDIA GTX 1660 Ti Mobile (6GB VRAM, Turing, compute capability 7.5),
  Intel i5-9300H (4 cores, 8 threads). Some NVML APIs return NOT_SUPPORTED on
  consumer GPUs (ECC, MIG, NVLink); all NVML calls must handle graceful degradation.
- **OS:** Ubuntu Linux (22.04 or 24.04).
- **Runtime:** k3s with embedded containerd. Containerd config path is
  `/var/lib/rancher/k3s/agent/etc/containerd/config.toml`.
- **Language:** Go 1.22+ per project stack decision.
- **Display GPU sharing:** The GTX 1660 Ti is also the display GPU. Container
  workloads share VRAM with the desktop compositor. 512MB headroom should be
  reserved for display stability.
- **VRAM limit:** 6GB total (5.5GB usable with display headroom) constrains the
  size of models and batch sizes that can be deployed.
- **Single replica:** The operator runs as a single-replica Deployment (no leader
  election needed).

---

## 9. Dependencies

| Dependency | Version | Notes |
|-----------|---------|-------|
| Go | 1.22+ | Operator language |
| controller-runtime | Latest stable | Operator framework |
| kubebuilder | Latest stable | Project scaffolding |
| go-nvml | Latest stable | NVIDIA GPU interaction |
| ghw | Latest stable | CPU / memory enumeration |
| k3s | Latest stable | Target Kubernetes distribution |
| NVIDIA drivers | 560+ (consumer branch) | Pre-installed on host |
| NVIDIA Container Toolkit | Latest stable | Pre-installed on host |
| Helm | 3.x | Template generation |

---

## 10. Story Map

```
E1 Pre-Install (P0/M1)
  |-- S1.1 Single-script prerequisite installation
  |-- S1.2 GPU detection and driver installation
  |-- S1.3 k3s installation with GPU validation

E2 Device Discovery (P0/M2)
  |-- S2.1 CPU capability enumeration
  |-- S2.2 GPU capability enumeration via NVML
  |-- S2.3 System memory enumeration
  |-- S2.4 Structured DeviceInfo output

E3 Operator Core (P0/M2)
  |-- S3.1 GPUCluster CRD definition
  |-- S3.2 Auto-discovery on GPUCluster creation
  |-- S3.3 Reconciliation loop with conditions
  |-- S3.4 Health and readiness endpoints

E4 Configuration Templates (P1/M3)
  |-- S4.1 Application profiles in GPUCluster spec
  |-- S4.2 Helm values generation
  |-- S4.3 Generated ConfigMap output

E5 Monitoring (P2/M3)
  |-- S5.1 GPU metrics endpoint (future -- not defined in stories yet)
```

---

## References

- Research: `/home/bibs/work/idealab/docs/research/gpu-operator-research.md`
- Stories: `/home/bibs/work/idealab/docs/stories/`
- Project Config: `/home/bibs/work/idealab/CLAUDE.md`
