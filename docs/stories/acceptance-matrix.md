# Acceptance Matrix

**Project:** idealab GPU Operator
**Date:** 2026-03-01
**JIRA Key:** IDEAL

---

## Legend

- **Pass:** All GIVEN/WHEN/THEN conditions met.
- **Fail:** Any condition not met.
- **N/T:** Not yet tested.
- **N/A:** Not applicable to current test environment.

---

## E1: Pre-Install Script (P0)

| Story | AC | Description | Status | Verified By | Date |
|-------|----|-------------|--------|-------------|------|
| IDEAL-101 S1.1 | AC1 | Script installs all required components in correct order | N/T | | |
| IDEAL-101 S1.1 | AC2 | Script is idempotent (safe to re-run) | N/T | | |
| IDEAL-101 S1.1 | AC3 | Script validates each step before proceeding | N/T | | |
| IDEAL-102 S1.2 | AC1 | Script detects NVIDIA GPU by PCI vendor ID | N/T | | |
| IDEAL-102 S1.2 | AC2 | Script installs correct driver version (>= 560) | N/T | | |
| IDEAL-102 S1.2 | AC3 | Script skips driver if compatible driver exists | N/T | | |
| IDEAL-103 S1.3 | AC1 | k3s is installed and node is Ready | N/T | | |
| IDEAL-103 S1.3 | AC2 | containerd configured for NVIDIA runtime + RuntimeClass | N/T | | |
| IDEAL-103 S1.3 | AC3 | GPU test pod runs nvidia-smi successfully | N/T | | |

**E1 Total:** 9 acceptance criteria

---

## E2: Device Discovery (P0)

| Story | AC | Description | Status | Verified By | Date |
|-------|----|-------------|--------|-------------|------|
| IDEAL-201 S2.1 | AC1 | CPU model, cores, threads, architecture reported | N/T | | |
| IDEAL-201 S2.1 | AC2 | CPU feature flags reported from system APIs | N/T | | |
| IDEAL-202 S2.2 | AC1 | GPU properties reported via NVML (name, UUID, VRAM, driver, CUDA, compute) | N/T | | |
| IDEAL-202 S2.2 | AC2 | Multiple GPUs enumerated with unique UUIDs | N/T | | |
| IDEAL-202 S2.2 | AC3 | Unsupported NVML functions degrade gracefully | N/T | | |
| IDEAL-203 S2.3 | AC1 | Total and available memory reported | N/T | | |
| IDEAL-203 S2.3 | AC2 | Memory values consistent with /proc/meminfo | N/T | | |
| IDEAL-204 S2.4 | AC1 | DeviceInfo struct matches CRD status schema | N/T | | |
| IDEAL-204 S2.4 | AC2 | Enumeration completes in under 5 seconds | N/T | | |

**E2 Total:** 9 acceptance criteria

---

## E3: Operator Core (P0)

| Story | AC | Description | Status | Verified By | Date |
|-------|----|-------------|--------|-------------|------|
| IDEAL-301 S3.1 | AC1 | CRD is installable and kubectl reports Established | N/T | | |
| IDEAL-301 S3.1 | AC2 | CRD spec and status schema are complete | N/T | | |
| IDEAL-301 S3.1 | AC3 | CRD enforces validation on invalid spec | N/T | | |
| IDEAL-302 S3.2 | AC1 | Status populated within 30s of creation | N/T | | |
| IDEAL-302 S3.2 | AC2 | GPU info in status matches nvidia-smi output | N/T | | |
| IDEAL-303 S3.3 | AC1 | Discovering condition set during reconciliation | N/T | | |
| IDEAL-303 S3.3 | AC2 | Ready condition set after successful reconciliation | N/T | | |
| IDEAL-303 S3.3 | AC3 | Error condition set on discovery failure with backoff | N/T | | |
| IDEAL-304 S3.4 | AC1 | /healthz returns 200 when operator is running | N/T | | |
| IDEAL-304 S3.4 | AC2 | /readyz reflects reconciliation state (200 vs 503) | N/T | | |

**E3 Total:** 10 acceptance criteria

---

## E4: Configuration Templates (P1)

| Story | AC | Description | Status | Verified By | Date |
|-------|----|-------------|--------|-------------|------|
| IDEAL-401 S4.1 | AC1 | Application profiles accepted in GPUCluster spec | N/T | | |
| IDEAL-401 S4.1 | AC2 | Profile validation rejects invalid entries | N/T | | |
| IDEAL-402 S4.2 | AC1 | Values file contains hardware-derived settings | N/T | | |
| IDEAL-402 S4.2 | AC2 | Values file contains user-specified overrides | N/T | | |
| IDEAL-402 S4.2 | AC3 | Resource allocation warns when exceeding hardware | N/T | | |
| IDEAL-403 S4.3 | AC1 | ConfigMap created per profile with owner reference | N/T | | |
| IDEAL-403 S4.3 | AC2 | ConfigMap updated when GPUCluster spec changes | N/T | | |

**E4 Total:** 7 acceptance criteria

---

## Summary

| Epic | Priority | Stories | Acceptance Criteria | Status |
|------|----------|---------|---------------------|--------|
| E1 Pre-Install Script | P0 | 3 | 9 | Not Started |
| E2 Device Discovery | P0 | 4 | 9 | Not Started |
| E3 Operator Core | P0 | 4 | 10 | Not Started |
| E4 Configuration Templates | P1 | 3 | 7 | Not Started |
| E5 Monitoring | P2 | TBD | TBD | Not Started |
| **Total** | | **14** | **35** | |
