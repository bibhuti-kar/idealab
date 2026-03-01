# Epic E1: Pre-Install Script (P0)

**Milestone:** M1 -- Pre-Install + k3s Running with GPU Support
**JIRA Epic:** IDEAL-E1
**Priority:** P0
**Dependencies:** None (first epic in the chain)

---

## Overview

A single bash script that installs and validates all host-level prerequisites for
running GPU workloads on k3s. The script targets Ubuntu Linux with a consumer
NVIDIA GPU. It is idempotent -- safe to run multiple times without causing errors
or duplicate installations.

---

## S1.1: Single-Script Prerequisite Installation

**IDEAL-101**
**Priority:** P0

> As a developer, I want to run a single script that installs all GPU
> prerequisites so that I do not have to manually configure each component.

### Acceptance Criteria

**AC1: Script installs all required components**
```
GIVEN a fresh Ubuntu 22.04 or 24.04 machine with an NVIDIA GPU
WHEN I run the pre-install script with sudo privileges
THEN the script installs NVIDIA drivers, NVIDIA Container Toolkit, k3s, and
     creates the nvidia RuntimeClass
AND each component is installed in the correct order (drivers -> toolkit -> k3s -> RuntimeClass)
```

**AC2: Script is idempotent**
```
GIVEN a machine where the pre-install script has already run successfully
WHEN I run the pre-install script again
THEN the script completes without errors
AND no components are duplicated or re-installed unnecessarily
AND the final system state is identical to the first run
```

**AC3: Script validates each step before proceeding**
```
GIVEN the pre-install script is running
WHEN a component installation completes
THEN the script runs a validation check for that component before proceeding
     to the next step
AND if validation fails the script exits with a non-zero exit code and a
     clear error message identifying which component failed
```

---

## S1.2: GPU Detection and Driver Installation

**IDEAL-102**
**Priority:** P0

> As a developer, I want the script to detect my GPU model and install the correct
> driver version so that I get a compatible driver without researching version
> matrices.

### Acceptance Criteria

**AC1: Script detects NVIDIA GPU**
```
GIVEN a machine with an NVIDIA GPU
WHEN the pre-install script runs the GPU detection step
THEN the script identifies the GPU by PCI vendor ID (0x10de)
AND prints the GPU model name to stdout (e.g., "Detected: NVIDIA GeForce GTX 1660 Ti")
```

**AC2: Script installs correct driver version**
```
GIVEN a detected NVIDIA GPU
WHEN the pre-install script runs the driver installation step
THEN the script installs a compatible NVIDIA driver (version 560 or later)
AND after installation nvidia-smi executes successfully and reports the
     correct GPU model and driver version
```

**AC3: Script skips driver installation if compatible driver exists**
```
GIVEN a machine where a compatible NVIDIA driver (>= 560) is already installed
WHEN the pre-install script runs the driver installation step
THEN the script detects the existing driver via nvidia-smi
AND skips driver installation
AND prints a message indicating the existing driver version is sufficient
```

---

## S1.3: k3s Installation with GPU Validation

**IDEAL-103**
**Priority:** P0

> As a developer, I want k3s installed and validated with GPU support so that I can
> immediately deploy GPU workloads after the script finishes.

### Acceptance Criteria

**AC1: k3s is installed and running**
```
GIVEN the NVIDIA drivers and Container Toolkit are installed
WHEN the pre-install script runs the k3s installation step
THEN k3s is installed and the k3s systemd service is active and running
AND kubectl get nodes shows the single node in Ready state
```

**AC2: containerd is configured for NVIDIA runtime**
```
GIVEN k3s is installed
WHEN the pre-install script configures the container runtime
THEN the NVIDIA Container Toolkit configures containerd at
     /var/lib/rancher/k3s/agent/etc/containerd/config.toml
AND the nvidia runtime handler is registered in the containerd configuration
AND the nvidia RuntimeClass resource exists in the cluster
```

**AC3: GPU workload runs successfully**
```
GIVEN k3s is running with NVIDIA runtime configured
WHEN the pre-install script runs the GPU validation step
THEN the script deploys a test pod that requests nvidia.com/gpu: 1 and runs
     nvidia-smi inside the container
AND the test pod completes successfully (exit code 0)
AND the script cleans up the test pod after validation
```
