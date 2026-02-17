# Observability Capacity Planning

This document covers capacity planning for MicroFoundry's observability stack:
Grafana Beyla (eBPF), Prometheus recording rules, Loki log aggregation, and Grafana dashboards.

## Architecture Overview

```
┌─────────────────┐    eBPF hooks     ┌──────────────────┐
│ Application Pods │ ◄──────────────── │ Beyla DaemonSet  │
│ (microfoundry ns)│                   │ (monitoring ns)  │
└─────────────────┘                   └────────┬─────────┘
                                               │ /metrics :9090
                                      ┌────────▼─────────┐
                                      │ Prometheus        │
                                      │ (recording rules) │
                                      └────────┬─────────┘
                                               │
                                      ┌────────▼─────────┐
                                      │ Grafana / Admin UI│
                                      └──────────────────┘
```

## Beyla eBPF Resource Overhead

### Per-Node Resource Consumption

| Metric           | Idle (0 req/s) | Light (~50 req/s) | Medium (~200 req/s) | Heavy (~1000 req/s) |
|------------------|----------------|--------------------|---------------------|---------------------|
| CPU              | ~10m           | ~50m               | ~150m               | ~400m               |
| Memory (RSS)     | ~60Mi          | ~80Mi              | ~120Mi              | ~200Mi              |
| BPF map memory   | ~8Mi           | ~12Mi              | ~20Mi               | ~32Mi               |

**Current limits**: 256Mi memory, 500m CPU (set in `beyla-config.yaml`).

### Scaling Breakpoints

- **256Mi limit**: Sufficient for ~500 req/s per node with `BEYLA_BPF_TRACK_REQUEST_HEADERS=false`
- **512Mi limit**: Required if header tracking is enabled or >500 req/s per node
- **1Gi limit**: For high-throughput nodes (>2000 req/s) or many distinct endpoints

### Configuration Knobs

| Setting                            | Effect on Resources  | Current Value |
|------------------------------------|----------------------|---------------|
| `BEYLA_BPF_TRACK_REQUEST_HEADERS`  | +30-50% memory       | `false`       |
| `BEYLA_BPF_BATCH_MAP_SIZE`         | Higher = more memory | `32`          |
| `BEYLA_OPEN_PORT` (fewer ports)    | Less CPU overhead    | 6 ports       |
| ServiceMonitor scrape interval     | More Prometheus load | `15s`         |

## Prometheus Cardinality Growth

### Recording Rule Time Series

Each deployed MicroFoundry app creates **5 recording rule time series**:

| Recording Rule                          | Labels                                   |
|-----------------------------------------|------------------------------------------|
| `microfoundry:http_request_rate:5m`     | `k8s_deployment_name`, `k8s_namespace`   |
| `microfoundry:http_error_rate:5m`       | `k8s_deployment_name`, `k8s_namespace`   |
| `microfoundry:http_latency_p50:5m`      | `k8s_deployment_name`, `k8s_namespace`   |
| `microfoundry:http_latency_p95:5m`      | `k8s_deployment_name`, `k8s_namespace`   |
| `microfoundry:http_latency_p99:5m`      | `k8s_deployment_name`, `k8s_namespace`   |

Plus **2 alert recording rules** (`microfoundry:alert:error_rate_high`, `microfoundry:alert:latency_p99_high`).

### Raw Beyla Metrics Cardinality

Beyla emits `http_server_request_duration_seconds` as a histogram with default buckets.
Cardinality per app ≈ `endpoints × status_codes × histogram_buckets`:

| Apps | Avg Endpoints | Status Codes | Buckets | Raw Series | Recording Rules | Total      |
|------|---------------|--------------|---------|------------|-----------------|------------|
| 5    | 5             | 3            | 15      | 1,125      | 35              | ~1,160     |
| 20   | 5             | 3            | 15      | 4,500      | 140             | ~4,640     |
| 50   | 8             | 4            | 15      | 24,000     | 350             | ~24,350    |
| 100  | 10            | 4            | 15      | 60,000     | 700             | ~60,700    |

**Key mitigation**: The `http_route!~"/\\*?|"` filter in recording rules excludes catch-all routes,
preventing unbounded cardinality from wildcard paths.

### Storage Projections

Prometheus TSDB storage per series ≈ 1-2 bytes/sample with compression.

| Apps | Series | Scrape Interval | Samples/day | Storage/day | Storage/15d retention |
|------|--------|-----------------|-------------|-------------|-----------------------|
| 5    | 1,160  | 15s             | 6.7M        | ~10MB       | ~150MB                |
| 20   | 4,640  | 15s             | 26.8M       | ~40MB       | ~600MB                |
| 50   | 24,350 | 15s             | 140M        | ~210MB      | ~3.1GB                |
| 100  | 60,700 | 15s             | 350M        | ~525MB      | ~7.9GB                |

## Loki Log Storage

### Log Volume Estimates

| Apps | Avg Log Rate  | Daily Volume | 7-day Retention |
|------|---------------|--------------|-----------------|
| 5    | 10 lines/s    | ~4.3GB       | ~30GB           |
| 20   | 10 lines/s    | ~17GB        | ~120GB          |
| 50   | 10 lines/s    | ~43GB        | ~300GB          |

**Mitigation**: Promtail only collects from pods labeled `app.kubernetes.io/managed-by: microfoundry`,
avoiding system and sidecar container logs.

## Scaling Recommendations

### Small (1-10 apps, Docker Desktop / dev)

- Beyla: 256Mi/500m per node (default)
- Prometheus: 1Gi memory, 10Gi storage
- Loki: filesystem storage, no persistence needed
- Recording rules at 15s interval

### Medium (10-50 apps, staging)

- Beyla: 256Mi/500m per node (default still sufficient)
- Prometheus: 2Gi memory, 50Gi storage
- Loki: persistent volume, 50Gi
- Consider increasing scrape interval to 30s if Prometheus is under pressure

### Large (50-200 apps, production)

- Beyla: 512Mi/1000m per node
- Prometheus: 4Gi+ memory, 100Gi+ storage, consider Thanos/Mimir for long-term
- Loki: distributed mode or Grafana Cloud
- Recording rules are essential to keep dashboard queries fast
- Enable `BEYLA_BPF_TRACK_REQUEST_HEADERS` only if needed for debugging

## eBPF Kernel Compatibility

| Kernel Version | Beyla Support | Notes                          |
|----------------|---------------|--------------------------------|
| 5.8+           | Full          | All features available         |
| 5.4-5.7        | Partial       | No BTF, fallback probes used   |
| < 5.4          | Not supported | Beyla will not start           |

Docker Desktop (WSL2) uses kernel 5.15+ so full support is available.

## Monitoring the Monitors

### Key Metrics to Watch

| Metric                                    | Threshold       | Action                       |
|-------------------------------------------|-----------------|------------------------------|
| `container_memory_working_set_bytes{container="beyla"}` | >200Mi | Consider increasing limit   |
| `up{job="beyla-metrics"}`                 | == 0 for 2m     | MFBeylaDown alert fires      |
| `prometheus_tsdb_head_series`             | >100k           | Review cardinality, add filters |
| `prometheus_rule_evaluation_duration_seconds` | >1s         | Optimize recording rules     |
| `loki_ingester_memory_chunks`             | >50k            | Add retention or scale Loki  |

### Alert Coverage

The following alerts provide observability-of-observability:

- **MFBeylaDown**: Beyla DaemonSet not being scraped (critical)
- **MFAppHighErrorRate**: >5% 5xx error rate per app (warning)
- **MFAppHighLatency**: p99 latency >2s per app (warning)
- **MFAppNoTraffic**: Running app with zero HTTP traffic for 15m (info)

## Authorization Audit Buffer

The OPA authorization middleware records every allow/deny decision in an in-memory ring buffer (`pkg/auth/audit.go`). This is separate from the Prometheus/Loki observability stack — it provides an IAM-specific audit trail visible in the admin UI.

### Capacity

| Setting | Default | Notes |
|---------|---------|-------|
| Buffer size | 1000 entries | Circular — oldest entries evicted on overflow |
| Entry size | ~300 bytes | Timestamp, user, action, resource, path, method, org, IP, reason |
| Memory footprint | ~300 KB | Fixed allocation, does not grow |

At 100 requests/minute, the buffer holds ~10 minutes of history. For production deployments requiring persistent audit trails, export entries to an external log sink (Loki, SIEM) via the `GET /api/audit` endpoint.

### Query Parameters

| Parameter | Description |
|-----------|-------------|
| `user` | Filter by user email |
| `resource` | Filter by resource type (apps, services, scim, etc.) |
| `action` | Filter by action (read, write, delete) |
| `limit` | Max entries to return (default: 100) |

---

## Recording Rule Staleness

When an app is deleted, its recording rule series become stale after the Prometheus
`staleness_delta` (default: 5 minutes). The `microfoundry:http_request_rate:5m` rule will
return no data, and the `MFAppNoTraffic` alert requires a matching
`kube_deployment_status_replicas_available > 0` to avoid false positives on deleted apps.
