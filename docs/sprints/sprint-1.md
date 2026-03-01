# Sprint 1 Plan -- idealab GPU Operator

**Sprint:** 1
**Start Date:** 2026-03-01
**Duration:** 2 weeks
**Author:** Tech Manager
**Status:** Planning

---

## Sprint Goal

k3s pre-install script validated, GPU Operator core deployed and reconciling
GPUCluster CRDs with device discovery.

---

## Scope

| Epic | Stories/Tasks | Total Tickets |
|------|--------------|---------------|
| Infrastructure | IDEAL-010 | 1 |
| E1: Pre-Install Script | IDEAL-101 | 1 |
| E2: Device Discovery | IDEAL-201, IDEAL-202, IDEAL-203, IDEAL-204, IDEAL-205 | 5 |
| E3: Operator Core | IDEAL-301, IDEAL-302, IDEAL-303, IDEAL-304, IDEAL-305, IDEAL-306 | 6 |
| **Total** | | **13 tickets** |

---

## Tickets in Implementation Order

The following order respects all dependency constraints. Items at the same
phase level can be worked in parallel.

### Phase 1: Foundation

| Order | ID | Title | Size | Deps | DoD |
|-------|-----|-------|------|------|-----|
| 1 | IDEAL-010 | Initialize Go module with dependencies | S | -- | `go build ./...` compiles, all deps resolve |

### Phase 2: Parallel Tracks (all depend only on IDEAL-010)

| Order | ID | Title | Size | Deps | DoD |
|-------|-----|-------|------|------|-----|
| 2 | IDEAL-101 | Update pre-install script | M | IDEAL-010 | Script runs end-to-end on Ubuntu, driver >= 560, GPU test pod passes |
| 3 | IDEAL-201 | Define discovery interfaces and types | S | IDEAL-010 | Discoverer interface, all structs, MockDiscoverer, unit tests pass |
| 4 | IDEAL-301 | Define CRD Go types | M | IDEAL-010 | Types compile, scheme registration, DeepCopy, 7 unit tests pass |
| 5 | IDEAL-302 | Implement health server | S | IDEAL-010 | /healthz + /readyz endpoints, 7 unit tests pass |

### Phase 3: Discovery Implementations (depend on IDEAL-201)

| Order | ID | Title | Size | Deps | DoD |
|-------|-----|-------|------|------|-----|
| 6 | IDEAL-202 | Implement CPU discovery | S | IDEAL-201 | CPUInfo populated, 3 unit tests pass |
| 7 | IDEAL-203 | Implement GPU discovery via NVML | M | IDEAL-201 | GPUInfo populated, NOT_SUPPORTED handled, 6 unit tests pass |
| 8 | IDEAL-204 | Implement memory discovery | S | IDEAL-201 | MemoryInfo populated, 2 unit tests pass |

### Phase 4: Discovery Assembly (depends on IDEAL-202, IDEAL-203, IDEAL-204)

| Order | ID | Title | Size | Deps | DoD |
|-------|-----|-------|------|------|-----|
| 9 | IDEAL-205 | Integrate NVMLDiscoverer | M | IDEAL-202, IDEAL-203, IDEAL-204 | Full DeviceInfo returned, JSON matches CRD, < 5s, 3 unit tests pass |

### Phase 5: Controller (depends on IDEAL-301, IDEAL-205)

| Order | ID | Title | Size | Deps | DoD |
|-------|-----|-------|------|------|-----|
| 10 | IDEAL-303 | Implement GPUCluster reconciler | L | IDEAL-301, IDEAL-205 | 7-step reconcile loop, conditions, events, 18 unit tests pass |
| 11 | IDEAL-304 | Implement node labeling | S | IDEAL-303 | 5 idealab.io/* labels, sanitized, MergeFrom, 3 unit tests pass |

### Phase 6: Wiring and Integration (depends on IDEAL-302, IDEAL-303, IDEAL-304)

| Order | ID | Title | Size | Deps | DoD |
|-------|-----|-------|------|------|-----|
| 12 | IDEAL-305 | Implement operator main entry point | M | IDEAL-301, IDEAL-302, IDEAL-303 | `go build` compiles, env vars parsed, mock mode works |
| 13 | IDEAL-306 | Integration tests with envtest | M | IDEAL-303, IDEAL-304, IDEAL-305 | 13 integration tests pass, no goroutine leaks |

---

## Sprint Success Criteria

All six criteria must be satisfied for the sprint to be considered complete.

| # | Criterion | Verification Method |
|---|-----------|-------------------|
| 1 | Pre-install script installs NVIDIA drivers (>= 560), container toolkit, k3s | Run script on Ubuntu with NVIDIA GPU, verify `nvidia-smi`, k3s running |
| 2 | GPUCluster CRD installed in k3s | `kubectl get crd gpuclusters.idealab.io` shows Established |
| 3 | Operator starts, reconciles GPUCluster, populates status with hardware info | Create GPUCluster CR, verify status.phase=Ready within 30s, status.node populated |
| 4 | Node labeled with GPU metadata | `kubectl get node --show-labels` contains `idealab.io/gpu-model`, `gpu-vram-mb`, etc. |
| 5 | Health endpoints respond correctly | `curl :8081/healthz` returns 200; `curl :8081/readyz` returns 200 after reconcile, 503 before |
| 6 | All unit + integration tests pass with >= 80% coverage | `go test ./... -coverprofile=coverage.txt` reports >= 80% line coverage |

---

## Quality Gate Checklist

All items must pass before Sprint 1 is marked Done.

- [ ] Tests pass: `go test ./...` exits 0
- [ ] Coverage >= 80% line, >= 70% branch on new code
- [ ] Zero lint errors: `golangci-lint run` exits 0
- [ ] Zero type errors: `go vet ./...` exits 0
- [ ] 100% P0 acceptance criteria have passing tests (28 ACs, 70 test cases)
- [ ] Docker builds: `docker build .` succeeds
- [ ] Docker runs non-root: container process runs as UID 1000+
- [ ] Docker health check: `/healthz` responds in container
- [ ] No secrets in code: no hardcoded URLs, passwords, tokens
- [ ] Dependency audit: `go mod verify` clean, no known CVEs

---

## Test Coverage Summary

From the test plan, Sprint 1 requires:

| Tier | Test Count | Scope |
|------|-----------|-------|
| Unit | 38 | Discovery (13), Controller (18), Health (7), CRD Types (7) -- note: some tests shared |
| Integration | 13 | Controller + CRD via envtest |
| E2E | 22 | Pre-install (10) + Full operator (12) |
| **Total** | **70** | **28 acceptance criteria covered** |

### Per-Ticket Test Count

| Ticket | Unit Tests | Integration Tests |
|--------|-----------|-------------------|
| IDEAL-201 | 5 (mock + serialization) | -- |
| IDEAL-202 | 3 (CPU) | -- |
| IDEAL-203 | 6 (GPU + NVML) | -- |
| IDEAL-204 | 2 (memory) | -- |
| IDEAL-205 | 3 (DeviceInfo assembly) | -- |
| IDEAL-301 | 7 (CRD types) | 4 (CRD CRUD) |
| IDEAL-302 | 7 (health endpoints) | -- |
| IDEAL-303 | 18 (reconciler) | 5 (reconciliation + events) |
| IDEAL-304 | 3 (node labels) | 1 (node labeled) |
| IDEAL-305 | -- | 1 (startup) |
| IDEAL-306 | -- | 13 (all integration) |

---

## Acceptance Criteria Traceability

Every P0 acceptance criterion from E1, E2, E3 stories maps to at least one
test. The full mapping is in the test plan at
`/home/bibs/work/idealab/docs/design/test-plan.md`, section 5.

### E1: Pre-Install Script (9 ACs)

| Story | AC | Ticket | Test Tier |
|-------|----|--------|-----------|
| IDEAL-101 | AC1: Installs all components | IDEAL-101 | E2E |
| IDEAL-101 | AC2: Idempotent | IDEAL-101 | E2E |
| IDEAL-101 | AC3: Validates each step | IDEAL-101 | E2E |
| IDEAL-102 | AC1: Detects GPU | IDEAL-101 | E2E |
| IDEAL-102 | AC2: Installs driver >= 560 | IDEAL-101 | E2E |
| IDEAL-102 | AC3: Skips existing driver | IDEAL-101 | E2E |
| IDEAL-103 | AC1: k3s running | IDEAL-101 | E2E |
| IDEAL-103 | AC2: containerd configured | IDEAL-101 | E2E |
| IDEAL-103 | AC3: GPU test pod | IDEAL-101 | E2E |

### E2: Device Discovery (9 ACs)

| Story | AC | Ticket | Test Tier |
|-------|----|--------|-----------|
| IDEAL-201 | AC1: CPU model/cores/threads | IDEAL-202 | Unit |
| IDEAL-201 | AC2: CPU features | IDEAL-202 | Unit |
| IDEAL-202 | AC1: GPU properties via NVML | IDEAL-203 | Unit |
| IDEAL-202 | AC2: Multiple GPUs enumerated | IDEAL-203 | Unit |
| IDEAL-202 | AC3: NOT_SUPPORTED degradation | IDEAL-203 | Unit |
| IDEAL-203 | AC1: Total and available memory | IDEAL-204 | Unit |
| IDEAL-203 | AC2: Consistent with /proc/meminfo | IDEAL-204 | Unit |
| IDEAL-204 | AC1: DeviceInfo matches CRD | IDEAL-205 | Unit |
| IDEAL-204 | AC2: Enumeration under 5s | IDEAL-205 | Unit |

### E3: Operator Core (10 ACs)

| Story | AC | Ticket | Test Tier |
|-------|----|--------|-----------|
| IDEAL-301 | AC1: CRD installable | IDEAL-301 | Unit + Integration |
| IDEAL-301 | AC2: Spec/status schema complete | IDEAL-301 | Unit + Integration |
| IDEAL-301 | AC3: CRD enforces validation | IDEAL-306 | Integration |
| IDEAL-302 | AC1: Status populated within 30s | IDEAL-303 | Unit + Integration |
| IDEAL-302 | AC2: GPU matches nvidia-smi | IDEAL-303 | Unit + E2E |
| IDEAL-303 | AC1: Discovering condition | IDEAL-303 | Unit + Integration |
| IDEAL-303 | AC2: Ready condition | IDEAL-303 | Unit + Integration |
| IDEAL-303 | AC3: Error condition | IDEAL-303 | Unit + Integration |
| IDEAL-304 | AC1: /healthz returns 200 | IDEAL-302 | Unit + E2E |
| IDEAL-304 | AC2: /readyz reflects state | IDEAL-302 | Unit + E2E |

---

## Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|-----------|------------|
| NVML cgo build complexity | Blocks GPU discovery tickets | Medium | Build tag separation for tests; MockDiscoverer covers unit/integration |
| envtest setup requires downloading API server + etcd binaries | Blocks IDEAL-306 | Low | Document in Makefile `setup-envtest` target; can download during IDEAL-010 |
| Pre-install script E2E tests require real GPU hardware | Blocks E2E validation of IDEAL-101 | Medium | Manual validation on target hardware; E2E tests tagged `//go:build e2e,gpu` |
| Controller-runtime API changes | Compile failures | Low | Pin controller-runtime version in go.mod |
| CRD YAML drift from Go types | Status/spec mismatch | Medium | Unit test `TestGPUCluster_JSONTags` catches drift; run controller-gen to regenerate |

---

## Sprint 2 Preview (Backlog)

The following tickets are scoped for Sprint 2 and are not part of this sprint.
They are listed here for forward planning only.

| ID | Title | Epic | Priority |
|----|-------|------|----------|
| IDEAL-401 | Application profiles in GPUCluster spec | E4 | P1 |
| IDEAL-402 | Helm values generation | E4 | P1 |
| IDEAL-403 | Generated ConfigMap output | E4 | P1 |

---

## References

- PRD: `/home/bibs/work/idealab/docs/prd.md`
- Technical Design: `/home/bibs/work/idealab/docs/design/technical-design.md`
- Test Plan: `/home/bibs/work/idealab/docs/design/test-plan.md`
- Stories: `/home/bibs/work/idealab/docs/stories/`
- Ticket Map: `/home/bibs/work/idealab/docs/tickets/ticket-map.md`
