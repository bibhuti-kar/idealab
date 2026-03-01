# Deploying GPU-Accelerated AI Applications on idealab

This guide explains how to prepare a Helm chart for a multi-service AI application where some services need GPU access (LLM inference) and others run on CPU only (API backends, workers). It uses the idealab operator's GPU discovery and node labeling to schedule workloads correctly.

The example deploys **Ollama** (GPU) + a **REST API server** (CPU) on a single-node k3s cluster with an NVIDIA GPU.

---

## Table of Contents

1. [Prerequisites](#1-prerequisites)
2. [Understanding Your GPU Resources](#2-understanding-your-gpu-resources)
3. [GPU vs CPU Services](#3-gpu-vs-cpu-services)
4. [The 5 GPU Helm Patterns](#4-the-5-gpu-helm-patterns)
5. [Example: Multi-Service AI Stack](#5-example-multi-service-ai-stack)
6. [Step-by-Step Deployment](#6-step-by-step-deployment)
7. [Model Selection for 6GB VRAM](#7-model-selection-for-6gb-vram)
8. [Troubleshooting](#8-troubleshooting)

---

## 1. Prerequisites

Before deploying a GPU workload, verify the idealab operator has discovered your hardware.

```bash
# Operator pod is running
kubectl get pods -n idealab-system
# NAME                               READY   STATUS    RESTARTS   AGE
# idealab-operator-xxxx-xxxx         1/1     Running   0          1h

# GPUCluster is in Ready phase
kubectl get gpuclusters
# NAME             PHASE   GPU                             VRAM   AGE
# my-gpu-cluster   Ready   NVIDIA GeForce GTX 1660 Ti      6144   1h

# Node has GPU labels
kubectl get nodes --show-labels | grep idealab
# idealab.io/gpu-model=NVIDIA-GeForce-GTX-1660-Ti
# idealab.io/gpu-vram-mb=6144
# idealab.io/gpu-cuda=12.8
# idealab.io/gpu-driver=570.211.01
# idealab.io/gpu-compute=7.5

# GPU resource is allocatable
kubectl describe node | grep nvidia.com/gpu
#   nvidia.com/gpu:  1
#   nvidia.com/gpu:  1

# RuntimeClass exists
kubectl get runtimeclass nvidia
# NAME     HANDLER   AGE
# nvidia   nvidia    1h

# Helm is installed
helm version
```

If any of these fail, see [Troubleshooting](#8-troubleshooting).

**Install Helm** (if not yet installed):

```bash
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

---

## 2. Understanding Your GPU Resources

The idealab operator discovers hardware and exposes it in two ways:

### GPUCluster Status (full details)

```bash
kubectl get gpucluster my-gpu-cluster -o yaml
```

```yaml
status:
  phase: Ready
  node:
    hostname: bibs-predator
    cpu:
      model: "Intel(R) Core(TM) i5-9300H CPU @ 2.40GHz"
      cores: 4
      threads: 8
      features: ["FMA", "SSE4.1", "SSE4.2", "AES-NI", "AVX", "AVX2"]
    gpu:
      model: "NVIDIA GeForce GTX 1660 Ti"
      vramMB: 6144
      driverVersion: "570.211.01"
      cudaVersion: "12.8"
      computeCapability: "7.5"
    memory:
      totalMB: 16384
```

Use this to decide what models fit your GPU (see [Model Selection](#7-model-selection-for-6gb-vram)).

### Node Labels (for scheduling)

The operator applies labels with the `idealab.io/` prefix to the k3s node:

| Label | Example Value | Use For |
|-------|--------------|---------|
| `idealab.io/gpu-model` | `NVIDIA-GeForce-GTX-1660-Ti` | Node affinity — schedule on GPU nodes |
| `idealab.io/gpu-vram-mb` | `6144` | Filter by VRAM — ensure model fits |
| `idealab.io/gpu-compute` | `7.5` | Minimum compute capability gates |
| `idealab.io/gpu-cuda` | `12.8` | CUDA version compatibility |
| `idealab.io/gpu-driver` | `570.211.01` | Driver version compatibility |

These labels let your Helm chart target the right node with `nodeAffinity`.

### The nvidia.com/gpu Resource

The NVIDIA device plugin registers `nvidia.com/gpu` as an extended resource on the node. This is what Kubernetes uses for GPU scheduling — when a pod requests `nvidia.com/gpu: 1`, the scheduler ensures the pod lands on a node with an available GPU.

On a single-GPU node, only **one pod** can claim the GPU at a time (unless using MIG or time-slicing, which consumer GPUs don't support).

---

## 3. GPU vs CPU Services

In a multi-service AI application, not every container needs GPU access. Only the inference engine (the service that loads and runs the model) needs the GPU. Everything else — API servers, databases, workers, frontends — runs on CPU.

| Service Type | Needs GPU? | RuntimeClass | nvidia.com/gpu | Example |
|-------------|-----------|-------------|---------------|---------|
| LLM inference | Yes | `nvidia` | `1` | Ollama, vLLM, TGI |
| Embedding server | Yes | `nvidia` | `1` | TEI, sentence-transformers |
| API gateway | No | _(omit)_ | _(omit)_ | FastAPI, Express, Go |
| Vector database | No | _(omit)_ | _(omit)_ | Qdrant, Chroma, pgvector |
| Frontend | No | _(omit)_ | _(omit)_ | React, Next.js |
| Worker/queue | No | _(omit)_ | _(omit)_ | Celery, BullMQ |

**Key rule:** If a container doesn't call CUDA/NVML, don't give it GPU resources. Adding `runtimeClassName: nvidia` or `nvidia.com/gpu` to a CPU-only pod wastes the GPU and blocks other workloads from using it.

---

## 4. The 5 GPU Helm Patterns

Every GPU workload on k3s with idealab needs these 5 patterns. CPU-only services need none of them.

### Pattern 1: RuntimeClass

```yaml
spec:
  runtimeClassName: nvidia
```

**Why:** Tells k3s containerd to use the NVIDIA container runtime instead of the default `runc`. The NVIDIA runtime mounts the host's GPU drivers (`libnvidia-ml.so`, `libcuda.so`) into the container. Without this, the container can't see the GPU even if it's scheduled on a GPU node.

### Pattern 2: GPU Resource Limits

```yaml
resources:
  limits:
    nvidia.com/gpu: "1"
```

**Why:** Requests a GPU from the Kubernetes scheduler. The NVIDIA device plugin tracks how many GPUs are available. If none are free, the pod stays `Pending`. On a single-GPU system, this means only one GPU pod runs at a time.

### Pattern 3: NVIDIA Environment Variables

```yaml
env:
  - name: NVIDIA_VISIBLE_DEVICES
    value: "all"
  - name: NVIDIA_DRIVER_CAPABILITIES
    value: "compute,utility"
```

**Why:** The NVIDIA container runtime reads these to decide which GPUs and capabilities to expose. `compute` enables CUDA; `utility` enables `nvidia-smi`. Other options: `graphics` (OpenGL), `video` (codec), `display`.

### Pattern 4: Shared Memory (/dev/shm)

```yaml
volumes:
  - name: shm
    emptyDir:
      medium: Memory
      sizeLimit: "2Gi"
# ...
volumeMounts:
  - name: shm
    mountPath: /dev/shm
```

**Why:** LLM inference engines (Ollama, vLLM, PyTorch) use shared memory for model weight loading and inter-process communication. The default `/dev/shm` in containers is 64MB — far too small for multi-billion parameter models. Set this to at least 1-2Gi for 7B models.

### Pattern 5: Node Affinity

```yaml
affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
        - matchExpressions:
            - key: idealab.io/gpu-model
              operator: Exists
```

**Why:** Ensures the pod only schedules on nodes where the idealab operator has discovered a GPU. This is especially useful in multi-node clusters, but good practice even on single-node setups for documentation and forward-compatibility.

You can also filter by specific GPU characteristics:

```yaml
# Only schedule on nodes with >= 6GB VRAM
- key: idealab.io/gpu-vram-mb
  operator: In
  values: ["6144", "8192", "16384", "24576"]

# Only schedule on nodes with compute capability >= 7.5
- key: idealab.io/gpu-compute
  operator: In
  values: ["7.5", "8.0", "8.6", "8.9", "9.0"]
```

---

## 5. Example: Multi-Service AI Stack

The example Helm chart at `deploy/examples/ai-stack/` deploys a two-service AI application:

### Architecture

```
┌───────────────────────────────────────────────────────────┐
│  k3s node (GTX 1660 Ti, 6GB VRAM, i5-9300H, 16GB RAM)   │
│                                                           │
│  ┌──────────────────┐       ┌───────────────────────┐    │
│  │  api-server       │──────▶│  ollama                │    │
│  │  (CPU only)       │       │  (GPU: nvidia.com/gpu) │    │
│  │                   │       │                         │    │
│  │  Flask app        │       │  phi3:mini model        │    │
│  │  POST /api/chat   │       │  port 11434             │    │
│  │  GET  /api/models │       │  runtimeClass: nvidia   │    │
│  │  GET  /health     │       │  /dev/shm: 2Gi          │    │
│  │  port 8000        │       │  PVC: 20Gi (models)     │    │
│  └────────┬─────────┘       └───────────────────────┘    │
│           │                                               │
│           ▼ NodePort :30080                               │
│      External access                                      │
└───────────────────────────────────────────────────────────┘
```

### Service Topology

| Service | Image | GPU? | Port | Purpose |
|---------|-------|------|------|---------|
| `ollama` | `ollama/ollama:latest` | Yes (1 GPU) | 11434 (ClusterIP) | Runs the LLM, serves inference API |
| `api-server` | `python:3.12-slim` | No | 8000 (NodePort 30080) | REST API that proxies to Ollama |

### How It Works

1. **Ollama** starts with `runtimeClassName: nvidia` and claims the GPU via `nvidia.com/gpu: 1`. A `postStart` lifecycle hook pulls the configured model (default: `phi3:mini`). Model files are stored on a PersistentVolumeClaim so they survive pod restarts.

2. **api-server** is a plain CPU container running a Flask app. It reads `OLLAMA_URL` from a ConfigMap and proxies chat requests to Ollama's `/api/generate` endpoint. It exposes a NodePort so you can call it from outside the cluster.

3. The **ConfigMap** (`ai-stack-config`) holds shared configuration: the Ollama URL, model name, and API port. All CPU services read from this — add more services by referencing the same ConfigMap.

### values.yaml Walkthrough

```yaml
ollama:
  image: ollama/ollama:latest
  model: "phi3:mini"         # Which model to pull (must fit in VRAM)
  port: 11434                # Ollama's default API port
  gpu:
    count: 1                 # GPUs to request (0 = CPU-only inference)
    shmSize: "2Gi"           # Shared memory for model loading
  resources:                 # CPU/memory limits for the container
    requests: { cpu: "1000m", memory: "4Gi" }
    limits:   { cpu: "4000m", memory: "8Gi" }
  persistence:
    enabled: true            # Store models on disk
    size: "20Gi"             # Space for model files
    storageClass: "local-path"  # k3s default storage class

api:
  image: python:3.12-slim    # Replace with your own API image
  replicas: 1                # Scale CPU services independently
  port: 8000
  serviceType: NodePort
  nodePort: 30080            # Access from http://<node-ip>:30080
  resources:
    requests: { cpu: "100m", memory: "128Mi" }
    limits:   { cpu: "500m", memory: "512Mi" }
```

---

## 6. Step-by-Step Deployment

### Install Helm (if needed)

```bash
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
helm version
```

### Deploy the AI Stack

```bash
# From the project root
helm install ai-stack deploy/examples/ai-stack/ \
  --create-namespace

# Watch pods come up
kubectl get pods -n ai-workloads -w
```

Expected output:

```
NAME                          READY   STATUS    RESTARTS   AGE
ollama-xxxx-xxxx              1/1     Running   0          2m
api-server-xxxx-xxxx          1/1     Running   0          2m
```

### Verify GPU Access

```bash
# Check Ollama logs for CUDA detection
kubectl logs -n ai-workloads deployment/ollama | head -20
# Should show: "Nvidia GPU detected"

# Verify the GPU is allocated
kubectl describe pod -n ai-workloads -l app=ollama | grep nvidia.com/gpu
#     nvidia.com/gpu: 1
```

### Test the API

```bash
# Check available models
curl http://localhost:30080/api/models

# Send a chat request
curl -X POST http://localhost:30080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"prompt": "What is Kubernetes in one sentence?"}'
```

### Override Values

```bash
# Use a different model
helm upgrade ai-stack deploy/examples/ai-stack/ \
  --set ollama.model="llama3.2:3b"

# Disable GPU (CPU-only inference, slower)
helm upgrade ai-stack deploy/examples/ai-stack/ \
  --set ollama.gpu.count=0

# Scale API replicas
helm upgrade ai-stack deploy/examples/ai-stack/ \
  --set api.replicas=3
```

### Uninstall

```bash
helm uninstall ai-stack -n ai-workloads
kubectl delete namespace ai-workloads
```

---

## 7. Model Selection for 6GB VRAM

The GTX 1660 Ti has 6GB VRAM. Some of that is used by the display (typically ~512MB on a laptop). Effective available VRAM for inference: **~5.5GB**.

### Models That Fit

| Model | Size | VRAM Usage | Quality | Speed |
|-------|------|-----------|---------|-------|
| `phi3:mini` (3.8B) | 2.3GB | ~3GB | Good for general tasks | Fast |
| `gemma2:2b` | 1.6GB | ~2.5GB | Good for simple tasks | Very fast |
| `llama3.2:3b` | 2.0GB | ~3GB | Strong reasoning | Fast |
| `mistral:7b-q4_0` (4-bit) | 3.8GB | ~4.5GB | Good quality, quantized | Moderate |
| `codellama:7b-q4_0` (4-bit) | 3.8GB | ~4.5GB | Code generation | Moderate |
| `llama3.1:8b-q4_0` (4-bit) | 4.7GB | ~5.5GB | Best quality, tight fit | Slow |

### Models That Don't Fit

| Model | Why |
|-------|-----|
| `llama3.1:8b` (FP16) | 16GB — needs 2x your VRAM |
| `mistral:7b` (FP16) | 14GB — too large |
| `llama3.1:70b` (any quant) | 40GB+ — needs A100/H100 |
| Any 13B+ model | Always > 6GB even at 4-bit |

### Choosing a Quantization

If a model is too large at full precision, use a quantized variant:

- **Q4_0** — 4-bit quantization, smallest, ~5% quality loss
- **Q4_K_M** — 4-bit with importance matrix, better quality than Q4_0
- **Q5_K_M** — 5-bit, larger but higher quality
- **Q8_0** — 8-bit, nearly lossless but 2x size of Q4

For 6GB VRAM, stick to **Q4_0** or **Q4_K_M** for 7B models.

### Set the Model in values.yaml

```yaml
ollama:
  model: "mistral:7b-q4_0"
```

Or override at install time:

```bash
helm install ai-stack deploy/examples/ai-stack/ \
  --set ollama.model="llama3.2:3b"
```

---

## 8. Troubleshooting

### Pod stuck in Pending

```bash
kubectl describe pod -n ai-workloads <pod-name>
```

| Message | Cause | Fix |
|---------|-------|-----|
| `0/1 nodes are available: Insufficient nvidia.com/gpu` | Another pod already has the GPU | Free the GPU: `kubectl delete pod` the other GPU workload |
| `0/1 nodes are available: node(s) didn't match Pod's node affinity` | Node missing `idealab.io/gpu-model` label | Check `kubectl get gpuclusters` — operator may not have reconciled |
| `RuntimeClass "nvidia" not found` | RuntimeClass not created | `kubectl apply -f` the RuntimeClass (see Prerequisites) |

### Pod in CrashLoopBackOff

```bash
kubectl logs -n ai-workloads <pod-name>
```

| Error | Cause | Fix |
|-------|-------|-----|
| `NVML init failed: ERROR_LIBRARY_NOT_FOUND` | Pod missing `runtimeClassName: nvidia` | Add `runtimeClassName: nvidia` to the pod spec |
| `CUDA out of memory` | Model too large for VRAM | Use a smaller model or quantized variant |
| `nvidia-container-cli: initialization error` | Container toolkit issue | Restart k3s: `sudo systemctl restart k3s` |

### Ollama Not Responding

```bash
# Check if Ollama is listening
kubectl exec -n ai-workloads deployment/ollama -- curl -s localhost:11434

# Check if model was pulled
kubectl exec -n ai-workloads deployment/ollama -- ollama list
```

If the model pull failed (e.g., no internet), pull it manually:

```bash
kubectl exec -n ai-workloads deployment/ollama -- ollama pull phi3:mini
```

### API Server Can't Reach Ollama

```bash
# Test DNS resolution from the API pod
kubectl exec -n ai-workloads deployment/api-server -- \
  python -c "import socket; print(socket.getaddrinfo('ollama', 11434))"

# Test connectivity
kubectl exec -n ai-workloads deployment/api-server -- \
  curl -s http://ollama:11434/
```

If DNS fails, check that both pods are in the same namespace and the Ollama Service exists:

```bash
kubectl get svc -n ai-workloads
```

### GPU Not Visible to k3s

If `nvidia.com/gpu` doesn't appear in node capacity:

```bash
# Check device plugin
kubectl get pods -n kube-system | grep nvidia
kubectl logs -n kube-system <nvidia-device-plugin-pod>

# Device plugin must run with runtimeClassName: nvidia
kubectl get daemonset nvidia-device-plugin-daemonset -n kube-system -o yaml | grep runtimeClassName
```

If missing, patch it:

```bash
kubectl patch daemonset nvidia-device-plugin-daemonset -n kube-system \
  --type='json' \
  -p='[{"op": "add", "path": "/spec/template/spec/runtimeClassName", "value": "nvidia"}]'
```
