# Epic E5: Monitoring (P1)

**Milestone:** M4 -- Prometheus Metrics for GPU Telemetry and Reconciliation
**JIRA Epic:** IDEAL-E5
**Priority:** P1
**Dependencies:** E3 (GPUCluster reconciler must exist), E2 (discovery provides GPU data)

---

## Overview

The monitoring epic adds Prometheus metrics to the operator, covering GPU
hardware telemetry (temperature, utilization, VRAM usage, power draw) and
reconciliation operational metrics (cycle count, duration, errors). Metrics
are served via the controller-runtime metrics endpoint on `:8080`.

---

## S5.1: Prometheus Metrics Endpoint

**IDEAL-501**
**Priority:** P1

> As an operator administrator, I want the operator to expose a Prometheus
> metrics endpoint so that I can monitor GPU and operator health.

### Acceptance Criteria

**AC1: Metrics endpoint available**
```
GIVEN the operator is running
WHEN a GET request is made to :8080/metrics
THEN the response contains Prometheus-formatted metrics
AND all custom metrics use the "idealab_" namespace prefix
```

**AC2: Endpoint is configurable**
```
GIVEN the METRICS_BIND_ADDRESS env var is set to a custom address
WHEN the operator starts
THEN the metrics endpoint binds to the configured address
```

---

## S5.2: GPU Telemetry Metrics

**IDEAL-502**
**Priority:** P1

> As an operator administrator, I want GPU hardware telemetry exported as
> Prometheus gauges so that I can monitor GPU health and utilization over time.

### Acceptance Criteria

**AC1: GPU gauges exported**
```
GIVEN the operator has completed hardware discovery
WHEN the metrics endpoint is scraped
THEN the following gauges are available per GPU (labeled by gpu model and uuid):
  - idealab_gpu_temperature_celsius
  - idealab_gpu_utilization_percent
  - idealab_gpu_vram_total_mb
  - idealab_gpu_vram_used_mb
  - idealab_gpu_power_watts
```

**AC2: Metrics updated each reconcile cycle**
```
GIVEN the operator re-reconciles every 5 minutes
WHEN hardware discovery runs
THEN all GPU gauges are updated with current values
AND stale readings are replaced (not accumulated)
```

---

## S5.3: Reconciliation Metrics

**IDEAL-503**
**Priority:** P1

> As an operator administrator, I want reconciliation operational metrics so
> that I can track operator performance and error rates.

### Acceptance Criteria

**AC1: Reconcile counter and histogram**
```
GIVEN the operator is reconciling GPUCluster resources
WHEN the metrics endpoint is scraped
THEN the following metrics are available:
  - idealab_reconcile_total (counter, labeled by result: success/error)
  - idealab_reconcile_duration_seconds (histogram)
  - idealab_configmaps_generated (gauge)
```

**AC2: Counter increments per cycle**
```
GIVEN the operator completes a reconciliation cycle
WHEN the cycle succeeds
THEN idealab_reconcile_total{result="success"} increments by 1
AND idealab_reconcile_duration_seconds records the cycle duration
```
