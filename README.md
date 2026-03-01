# idealab

A simplified GPU Operator for k3s single-node edge deployments. Enumerates CPU/GPU capabilities and provides Helm chart configuration templates for multi-service AI workloads with GPU resource scheduling.

## Overview

idealab is a Kubernetes operator designed for single-node k3s deployments on gaming PCs and edge devices with NVIDIA GPUs. It:

1. **Discovers hardware** — Enumerates CPU cores/features, GPU model/VRAM/compute capability
2. **Manages GPU resources** — Makes NVIDIA GPUs schedulable as `nvidia.com/gpu` resources in Kubernetes
3. **Generates configurations** — Produces Helm values and deployment templates optimized for detected hardware
4. **Schedules AI workloads** — Allocates GPU, CPU, and memory for multi-service AI applications

## Prerequisites

- Linux (Ubuntu 22.04+ recommended)
- NVIDIA GPU (consumer or data center)
- Root access for pre-install

## Quick Start

### 1. Pre-install (sets up drivers, k3s, container toolkit)

```bash
sudo make preinstall
```

This installs:
- NVIDIA drivers (if not present)
- NVIDIA Container Toolkit
- k3s with GPU support
- Required system packages

### 2. Validate k3s + GPU

```bash
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
kubectl get nodes
kubectl describe node | grep nvidia
```

### 3. Deploy the operator

```bash
make deploy
```

### 4. Check operator status

```bash
kubectl get gpucluster -A
kubectl logs -l app=idealab-operator -n idealab-system
```

## Development

```bash
# Run tests
make test

# Lint
make lint

# Build binaries
make build

# Run locally
make dev-local
```

## Architecture

```
cmd/
  operator/     — Operator entrypoint
  preinstall/   — Pre-install script
internal/
  discovery/    — CPU/GPU device enumeration
  controller/   — Kubernetes reconciler
  config/       — Config templates and rendering
  health/       — Health check server
api/
  v1alpha1/     — CRD type definitions
deploy/
  crds/         — CustomResourceDefinitions
  operator/     — Operator deployment manifests
  templates/    — Helm value templates
```

## License

Proprietary
