# Epic E3: Operator Core -- CRD + Controller (P0)

**Milestone:** M2 -- Operator with Device Discovery + GPUCluster CRD
**JIRA Epic:** IDEAL-E3
**Priority:** P0
**Dependencies:** E2 (device discovery provides hardware data for CRD status)

---

## Overview

The operator core defines the GPUCluster Custom Resource Definition and implements
the controller reconciliation loop using controller-runtime. On creation of a
GPUCluster resource, the controller triggers device discovery, populates the status
with hardware data, and reports conditions. The operator runs as a single-replica
Deployment in k3s.

---

## S3.1: GPUCluster CRD Definition

**IDEAL-301**
**Priority:** P0

> As a developer, I want a GPUCluster CRD that captures my cluster's GPU
> configuration so that I have a single Kubernetes resource representing my
> hardware and desired workload state.

### Acceptance Criteria

**AC1: CRD is installable and valid**
```
GIVEN the GPUCluster CRD YAML is generated from Go type definitions (api/v1alpha1)
WHEN kubectl apply -f is run against the CRD manifest
THEN the CRD is created in the cluster
AND kubectl get crd gpuclusters.idealab.dev returns the CRD with status Established
```

**AC2: CRD spec and status schema are complete**
```
GIVEN the GPUCluster CRD is installed
WHEN a GPUCluster resource is created with a valid spec
THEN the spec accepts fields for: cluster name, gpu memory reserve (bytes),
     and application profiles (list)
AND the status contains fields for: discovered CPU info, GPU info list,
     memory info, conditions list, and last discovery timestamp
AND the status is a subresource (status updates do not require spec changes)
```

**AC3: CRD enforces validation**
```
GIVEN the GPUCluster CRD is installed
WHEN a GPUCluster resource is created with an invalid spec (e.g., negative
     gpu memory reserve)
THEN the API server rejects the request with a validation error
AND a clear error message is returned
```

---

## S3.2: Auto-Discovery on GPUCluster Creation

**IDEAL-302**
**Priority:** P0

> As a developer, I want the operator to auto-discover hardware and populate
> GPUCluster status on creation so that I get an accurate hardware profile without
> running separate tools.

### Acceptance Criteria

**AC1: Status is populated after creation**
```
GIVEN the operator is running in the cluster
WHEN a GPUCluster resource is created via kubectl apply
THEN the operator reconciles the resource within 30 seconds
AND the GPUCluster status is populated with discovered CPU, GPU, and memory info
AND the status.lastDiscoveryTimestamp is set to the time of discovery
```

**AC2: GPU info in status matches actual hardware**
```
GIVEN the operator has reconciled a GPUCluster resource
WHEN the GPUCluster status is read via kubectl get gpucluster -o yaml
THEN the GPU entries contain the correct device name, UUID, total VRAM,
     driver version, CUDA version, and compute capability
AND these values match the output of nvidia-smi on the host
```

---

## S3.3: Reconciliation Loop with Conditions

**IDEAL-303**
**Priority:** P0

> As a developer, I want the operator to reconcile GPUCluster state and report
> conditions (Ready, Error, Discovering) so that I can monitor the operator's
> status through standard Kubernetes mechanisms.

### Acceptance Criteria

**AC1: Discovering condition is set during reconciliation**
```
GIVEN a GPUCluster resource exists
WHEN the operator begins reconciliation
THEN the operator sets the condition type "Discovering" with status "True"
     and reason "DiscoveryInProgress"
AND after discovery completes, the "Discovering" condition is set to "False"
```

**AC2: Ready condition is set after successful reconciliation**
```
GIVEN the operator completes device discovery without errors
WHEN the reconciliation loop finishes
THEN the operator sets the condition type "Ready" with status "True"
     and reason "ReconcileSucceeded"
AND the condition's lastTransitionTime is set to the current time
```

**AC3: Error condition is set on failure**
```
GIVEN device discovery fails (e.g., NVML initialization error, no GPU found)
WHEN the reconciliation loop encounters the error
THEN the operator sets the condition type "Ready" with status "False"
     and reason "DiscoveryFailed"
AND the condition message includes the error description
AND the operator requeues the reconciliation with exponential backoff
```

---

## S3.4: Health and Readiness Endpoints

**IDEAL-304**
**Priority:** P0

> As a developer, I want health endpoints (/healthz, /readyz) for the operator so
> that Kubernetes can determine if the operator pod is alive and ready to serve.

### Acceptance Criteria

**AC1: Health endpoint responds**
```
GIVEN the operator is running
WHEN an HTTP GET request is sent to /healthz on the operator's health port
THEN the response status code is 200
AND the response body contains "ok" or equivalent health-check response
```

**AC2: Readiness endpoint reflects reconciliation state**
```
GIVEN the operator is running and has successfully reconciled at least one
     GPUCluster resource
WHEN an HTTP GET request is sent to /readyz on the operator's health port
THEN the response status code is 200
AND if the operator has not yet reconciled any resource the response status
     code is 503
```
