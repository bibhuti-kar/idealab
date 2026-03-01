# Epic E4: Configuration Templates (P1)

**Milestone:** M3 -- Helm Chart Template Generation for AI Workloads
**JIRA Epic:** IDEAL-E4
**Priority:** P1
**Status:** IMPLEMENTED (Sprint 2)
**Dependencies:** E3 (GPUCluster CRD and operator core must exist)

---

## Overview

The configuration templates epic enables the operator to generate Helm values
files from the combination of discovered hardware capabilities and user-defined
application profiles. Users define what they want to deploy in the GPUCluster
spec; the operator combines that with what the hardware can support and outputs
ready-to-use ConfigMaps containing Helm values.

---

## S4.1: Application Profiles in GPUCluster Spec

**IDEAL-401**
**Priority:** P1

> As a developer, I want to define application profiles in GPUCluster spec with
> Helm chart references and resource requirements so that I can declare what AI
> workloads I intend to deploy.

### Acceptance Criteria

**AC1: Application profiles are accepted in spec**
```
GIVEN the GPUCluster CRD is installed
WHEN a GPUCluster resource is created with an applicationProfiles list in the spec
THEN each profile entry accepts: name (string), helmChart (repo URL + chart name +
     version), gpuMemoryRequired (bytes), cpuRequired (millicores),
     memoryRequired (bytes), and environment overrides (key-value map)
AND the resource is accepted by the API server
```

**AC2: Profile validation rejects invalid entries**
```
GIVEN the GPUCluster CRD is installed
WHEN a GPUCluster resource is created with an application profile that has
     an empty name or missing helmChart reference
THEN the API server rejects the resource with a validation error
AND the error message identifies which profile field is invalid
```

> **Implementation:** CRD YAML updated with `required: [name, helmChart]` and
> `minLength: 1` on both fields (`deploy/crds/gpucluster.yaml`).

---

## S4.2: Helm Values Generation

**IDEAL-402**
**Priority:** P1

> As the operator, I need to generate Helm values files from hardware discovery
> combined with application profiles so that each workload gets configuration
> matched to the actual hardware.

### Acceptance Criteria

**AC1: Values file contains hardware-derived settings**
```
GIVEN a GPUCluster with one or more application profiles in the spec
AND the operator has completed hardware discovery
WHEN the operator generates Helm values for a profile
THEN the generated values include GPU resource limits derived from discovered
     VRAM (minus the configured reserve)
AND CPU limits derived from discovered cores
AND memory limits derived from discovered system memory
AND the GPU device name and compute capability are included as metadata
```

> **Implementation:** `internal/config/values.go` — `GenerateValues()` builds
> hardware defaults (gpu model, vramMB minus 512 reserve, compute capability,
> CUDA/driver versions, resource limits) then deep-merges user HelmValues on top.

**AC2: Values file contains user-specified overrides**
```
GIVEN a GPUCluster application profile includes environment overrides
WHEN the operator generates Helm values for that profile
THEN the generated values include all user-specified environment key-value pairs
AND user overrides take precedence over hardware-derived defaults
```

> **Implementation:** `internal/config/merge.go` — `mergeMaps()` performs
> recursive deep merge where user values (src) always win over defaults (dst).

**AC3: Resource allocation respects hardware limits**
```
GIVEN multiple application profiles whose total gpuMemoryRequired exceeds
     available VRAM (minus reserve)
WHEN the operator generates Helm values
THEN the operator sets a condition on the GPUCluster with type "ResourceWarning"
     and status "True" indicating that requested resources exceed available hardware
AND the values files are still generated (the warning is advisory, not blocking)
```

> **Implementation:** `internal/config/validate.go` — `CheckResourceOvercommit()`
> returns advisory warning string. Set on `status.resourceWarning` field.
> `internal/controller/configmaps.go` — `checkResourceWarning()` called after
> ConfigMap reconciliation.

---

## S4.3: Generated ConfigMap Output

**IDEAL-403**
**Priority:** P1

> As a developer, I want to see the generated configuration as a ConfigMap in the
> cluster so that I can inspect, version, and use the generated Helm values with
> standard Kubernetes tooling.

### Acceptance Criteria

**AC1: ConfigMap is created per application profile**
```
GIVEN the operator has generated Helm values for an application profile
WHEN the reconciliation loop completes
THEN a ConfigMap is created in the same namespace as the GPUCluster resource
AND the ConfigMap name follows the pattern: {gpucluster-name}-{profile-name}-values
AND the ConfigMap data contains a key "values.yaml" with the generated YAML content
AND the ConfigMap has labels linking it to the GPUCluster resource
```

> **Implementation:** `internal/controller/configmaps.go` — ConfigMaps use labels
> (`idealab.io/gpucluster`, `idealab.io/profile`, `app.kubernetes.io/managed-by`)
> instead of owner references (cross-scope: cluster-scoped CR can't own namespaced
> ConfigMaps). Cleanup via finalizer `idealab.io/configmap-cleanup` in
> `internal/controller/finalizer.go`.

**AC2: ConfigMap is updated when GPUCluster changes**
```
GIVEN a ConfigMap was previously generated for an application profile
WHEN the GPUCluster spec is updated (e.g., profile resource requirements change)
THEN the operator re-generates the Helm values
AND updates the existing ConfigMap with the new content
AND the ConfigMap's resourceVersion changes to reflect the update
```

> **Implementation:** `reconcileConfigMaps()` in `configmaps.go` checks for
> existing ConfigMap via `Get`, then calls `Update` if found, `Create` if not.
> Orphan profiles (removed from spec) are cleaned up via `cleanupOrphanConfigMaps()`.
