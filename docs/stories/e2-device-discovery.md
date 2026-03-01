# Epic E2: Device Discovery (P0)

**Milestone:** M2 -- Operator with Device Discovery + GPUCluster CRD
**JIRA Epic:** IDEAL-E2
**Priority:** P0
**Dependencies:** E1 (pre-install provides host-level GPU stack)

---

## Overview

The device discovery module enumerates CPU, GPU, and memory hardware on the node
using NVML (for GPU) and system APIs (for CPU and memory). It produces a structured
`DeviceInfo` that the operator controller uses to populate the GPUCluster CRD
status. All NVML calls handle graceful degradation for consumer GPU functions that
return NOT_SUPPORTED.

---

## S2.1: CPU Capability Enumeration

**IDEAL-201**
**Priority:** P0

> As the operator, I need to enumerate CPU capabilities (model, cores, threads,
> features) to report node hardware in the GPUCluster status.

### Acceptance Criteria

**AC1: CPU model and topology are reported**
```
GIVEN the discovery module runs on a Linux machine
WHEN CPU enumeration is invoked
THEN the result includes the CPU model name (e.g., "Intel(R) Core(TM) i5-9300H")
AND the number of physical cores (e.g., 4)
AND the number of logical threads (e.g., 8)
AND the CPU architecture (e.g., "x86_64")
```

**AC2: CPU feature flags are reported**
```
GIVEN the discovery module runs on a Linux machine
WHEN CPU enumeration is invoked
THEN the result includes a list of CPU instruction set features
     (e.g., SSE4.2, AVX, AVX2)
AND the list is derived from system APIs (not hardcoded)
```

---

## S2.2: GPU Capability Enumeration via NVML

**IDEAL-202**
**Priority:** P0

> As the operator, I need to enumerate GPU capabilities (model, VRAM, driver
> version, CUDA version, compute capability) via NVML to report GPU hardware in
> the GPUCluster status.

### Acceptance Criteria

**AC1: GPU properties are reported via NVML**
```
GIVEN the discovery module runs on a machine with an NVIDIA GPU and drivers installed
WHEN GPU enumeration is invoked
THEN the result includes the GPU device name (e.g., "NVIDIA GeForce GTX 1660 Ti")
AND the GPU UUID
AND total VRAM in bytes
AND the NVIDIA driver version string
AND the CUDA driver version
AND the compute capability (major.minor, e.g., "7.5")
```

**AC2: Multiple GPUs are enumerated**
```
GIVEN the discovery module runs on a machine with one or more NVIDIA GPUs
WHEN GPU enumeration is invoked
THEN the result contains an entry for each GPU reported by nvml.DeviceGetCount()
AND each entry has a unique UUID
```

**AC3: Unsupported NVML functions degrade gracefully**
```
GIVEN the discovery module runs on a consumer GPU (e.g., GTX 1660 Ti)
WHEN an NVML call returns NVML_ERROR_NOT_SUPPORTED (e.g., ECC mode, MIG mode)
THEN the corresponding field in the result is set to a zero value or "not supported"
AND no error is raised for the overall enumeration
AND a log message at debug level records which function was unsupported
```

---

## S2.3: System Memory Enumeration

**IDEAL-203**
**Priority:** P0

> As the operator, I need to enumerate system memory to calculate allocation limits
> and report total available resources in the GPUCluster status.

### Acceptance Criteria

**AC1: Total and available memory are reported**
```
GIVEN the discovery module runs on a Linux machine
WHEN memory enumeration is invoked
THEN the result includes total system memory in bytes
AND available memory in bytes at the time of enumeration
```

**AC2: Memory values are consistent with system reports**
```
GIVEN the discovery module runs on a Linux machine
WHEN memory enumeration is invoked
THEN the reported total memory is within 5% of the value reported by /proc/meminfo
     (accounting for kernel-reserved memory)
```

---

## S2.4: Structured DeviceInfo Output

**IDEAL-204**
**Priority:** P0

> As the operator, I need the discovery module to output a structured DeviceInfo
> that feeds into the GPUCluster CRD status so that hardware data flows directly
> into the Kubernetes resource.

### Acceptance Criteria

**AC1: DeviceInfo struct matches CRD status schema**
```
GIVEN the discovery module has completed CPU, GPU, and memory enumeration
WHEN DeviceInfo is produced
THEN it contains a CPUInfo field with model, cores, threads, architecture, features
AND it contains a list of GPUInfo entries each with name, UUID, VRAM, driver version,
    CUDA version, compute capability
AND it contains a MemoryInfo field with total and available bytes
AND the struct can be directly serialized into the GPUCluster status subresource
```

**AC2: DeviceInfo is produced within a reasonable time**
```
GIVEN the discovery module runs on the target hardware
WHEN DeviceInfo is produced
THEN the total enumeration completes in under 5 seconds
AND NVML is properly initialized before use and shut down after use
```
