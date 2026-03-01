# idealab

A simplified GPU Operator for k3s single-node edge deployments — enumerates CPU/GPU capabilities and provides Helm chart configuration templates for multi-service AI workloads with GPU resource scheduling.

## Stack
- Go 1.22+ / controller-runtime / client-go
- Kubernetes: k3s (single-node)
- GPU: NVIDIA (go-nvml bindings)
- Hardware: ghw for device enumeration
- Container Runtime: containerd + NVIDIA Container Toolkit

## Company
- Org: bibhuti-kar (personal)
- JIRA Key: IDEAL
- Repo: TBD

## Git Conventions
- Branch: `[type]/IDEAL-NNN-description`
- Commit: `IDEAL-NNN type: description`

## Modules
- `cmd/operator/` — Operator entrypoint
- `cmd/preinstall/` — Pre-install script for dependencies
- `internal/discovery/` — CPU/GPU device enumeration
- `internal/controller/` — Kubernetes operator reconciler
- `internal/config/` — Configuration and template rendering
- `internal/health/` — Health check endpoints
- `api/v1alpha1/` — CRD type definitions

## Key Decisions
- Single-node only (no multi-node scheduling complexity)
- Consumer GPU support (GTX 1660 Ti, not just data center)
- Pre-install handles: NVIDIA drivers, container toolkit, Go, k3s
- Operator handles: device discovery, CRD management, Helm value generation
