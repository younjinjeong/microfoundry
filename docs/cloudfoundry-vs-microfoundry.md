# Cloud Foundry vs MicroFoundry — Architectural Comparison

> A detailed side-by-side comparison of Cloud Foundry's enterprise PaaS architecture and MicroFoundry's Kubernetes-native approach.

---

## Table of Contents

1. [Philosophy](#philosophy)
2. [Component-by-Component Mapping](#component-by-component-mapping)
   - [API Server](#1-api-server)
   - [Container Runtime & Scheduling](#2-container-runtime--scheduling)
   - [Routing / Ingress](#3-routing--ingress)
   - [Authentication & Identity](#4-authentication--identity)
   - [Authorization / RBAC](#5-authorization--rbac)
   - [Logging & Monitoring](#6-logging--monitoring)
   - [Service Brokers / Backing Services](#7-service-brokers--backing-services)
   - [Build System / Staging](#8-build-system--staging)
   - [Multi-Tenancy Model](#9-multi-tenancy-model)
   - [Infrastructure / Deployment Model](#10-infrastructure--deployment-model)
3. [CLI Experience](#cli-experience)
4. [Web Admin UI](#web-admin-ui)
5. [Resource Comparison](#resource-comparison)
6. [What MicroFoundry Borrows from CF](#what-microfoundry-borrows-from-cf)
7. [What MicroFoundry Does Differently](#what-microfoundry-does-differently)
8. [Architecture Diagrams](#architecture-diagrams)

---

## Philosophy

| | Cloud Foundry | MicroFoundry |
|---|---|---|
| **Core idea** | Enterprise PaaS on VMs via BOSH | Lightweight PaaS on Kubernetes |
| **Deployment unit** | 40-80+ VMs (production HA) | Single Go binary (~15 MB container) |
| **Abstraction layer** | IaaS VMs (BOSH CPI) | Kubernetes API |
| **Developer experience** | `cf push` abstracts everything | `mf push` + full K8s visibility |
| **Operator experience** | BOSH manifests, stemcells, releases | Helm chart + `mf admin` web UI |
| **State storage** | PostgreSQL/MySQL databases | Kubernetes objects (ConfigMaps, Secrets, Deployments) |
| **Language** | Ruby (CC), Go (Diego, Router), Java (UAA) | Go (100%) |

---

## Component-by-Component Mapping

### 1. API Server

| | Cloud Foundry | MicroFoundry |
|---|---|---|
| **Component** | Cloud Controller (CAPI) — Ruby/Nginx | `pkg/admin/server.go` — Go HTTP server |
| **API style** | CF API v3 (REST, JSON) | REST + HTMX (server-rendered HTML) |
| **Deployment** | 2-4 BOSH VMs + workers + clock | Single container running `mf admin` |
| **Database** | PostgreSQL/MySQL | K8s API (ConfigMaps as data store) |
| **Async jobs** | Delayed Job (Ruby) + CC Worker VMs | Synchronous or K8s-native (watch/reconcile) |

**Key difference**: CF's Cloud Controller is a complex stateful Ruby app with background job workers. MicroFoundry is stateless — it uses Kubernetes itself as the database, storing all app/service/config metadata as K8s objects.

### 2. Container Runtime & Scheduling

| | Cloud Foundry | MicroFoundry |
|---|---|---|
| **Component** | Diego (BBS + Auctioneer + Rep + Garden) | Kubernetes (native scheduler) |
| **Container runtime** | Garden-runC (OCI) | containerd/CRI-O via K8s |
| **Scheduling** | Auction-based (Auctioneer queries Cells) | K8s scheduler (bin-packing, affinity, etc.) |
| **State store** | BBS (gRPC + MySQL/PostgreSQL) | etcd (via K8s API) |
| **Deployment** | 10-500 Diego Cell VMs | Existing K8s nodes |
| **Health checks** | Rep does port/HTTP checks on containers | K8s readiness/liveness probes |
| **App representation** | DesiredLRP -> ActualLRP | K8s Deployment -> ReplicaSet -> Pod |

**Key difference**: CF builds its own scheduler on top of VMs. MicroFoundry delegates entirely to Kubernetes, translating `mf push` into K8s Deployments. Zero scheduling infrastructure to operate.

### 3. Routing / Ingress

| | Cloud Foundry | MicroFoundry |
|---|---|---|
| **Component** | Gorouter (Go HTTP proxy) | Pluggable gateway: Nginx/Kong/Traefik |
| **Route registration** | NATS pub/sub (Route Emitter -> Gorouter) | K8s Ingress objects |
| **Load balancing** | Round-robin with sticky sessions | Gateway provider native (IPVS, etc.) |
| **TLS** | Gorouter terminates TLS | Ingress controller + cert-manager/mkcert |
| **WebSocket** | Upgrade detection in Gorouter | Gateway annotations per provider |
| **TCP routing** | Separate TCP Router component | Gateway-specific (Kong TCP, Traefik TCP) |
| **gRPC** | Limited (requires workarounds) | Native via gateway backend-protocol annotations |
| **Deployment** | 2-6 BOSH VMs behind external LB | K8s DaemonSet/Deployment (community-maintained) |

**Key difference**: CF maintains its own router. MicroFoundry uses the `GatewayProvider` interface (`pkg/k8s/gateway.go`) with 3 implementations — operators choose their preferred ingress controller. Route management is done through K8s Ingress resources, not a custom messaging bus.

### 4. Authentication & Identity

| | Cloud Foundry | MicroFoundry |
|---|---|---|
| **Component** | UAA (Java/Spring Boot) | Keycloak (external) + OIDC client |
| **Protocol** | OAuth2/OIDC, SAML, LDAP | OAuth2/OIDC |
| **User store** | Internal DB, LDAP federation | Keycloak (internal DB, LDAP, SAML federation) |
| **Token format** | JWT | JWT |
| **Session mgmt** | Stateless token-based | Cookie-based sessions (`pkg/auth/session.go`) |
| **SCIM** | Built into UAA | `pkg/auth/scim.go` — SCIM v2 endpoints |
| **Deployment** | 2-4 BOSH VMs (Java on Tomcat) | Keycloak as K8s Deployment |
| **Admin API** | `uaac` CLI | `pkg/auth/keycloak_admin.go` — REST client |

**Key difference**: CF bundles UAA as a core component. MicroFoundry externalizes identity to Keycloak, treating it as a pluggable backing service. Both provide OIDC/OAuth2 + SCIM.

### 5. Authorization / RBAC

| | Cloud Foundry | MicroFoundry |
|---|---|---|
| **Model** | CC-enforced roles (OrgManager, SpaceDeveloper, etc.) | OPA Rego policies (`pkg/auth/policies/authz.rego`) |
| **Hierarchy** | Foundation -> Org -> Space | Platform -> Workspace -> Organization |
| **Roles** | 8 roles (Admin, OrgManager, SpaceDeveloper, SpaceAuditor, etc.) | 5 tiers (platform-admin, workspace-admin, org-admin, org-member, viewer) |
| **Policy storage** | CC database (hardcoded in CC Ruby code) | K8s ConfigMap + Rego files (editable at runtime) |
| **Quotas** | Org/Space quotas (memory, instances, routes) | Not yet implemented |
| **Isolation** | Isolation Segments (placement tags on Diego Cells) | Multi-cluster (separate K8s contexts) |

**Key difference**: CF's RBAC is hardcoded in the Cloud Controller. MicroFoundry uses OPA with Rego policies — the authorization logic is declarative and editable through the admin UI's policy editor. More flexible but less mature on quota enforcement.

### 6. Logging & Monitoring

| | Cloud Foundry | MicroFoundry |
|---|---|---|
| **Log pipeline** | Loggregator (Metron -> Doppler -> Traffic Controller) | Loki + direct K8s pod logs (`pkg/monitoring/loki.go`) |
| **Metrics** | Loggregator envelopes (Counter, Gauge, Timer) | Prometheus (`pkg/monitoring/prometheus.go`) + Beyla eBPF |
| **Dashboards** | External nozzles -> Datadog/Splunk/etc. | Grafana (embedded in admin UI) |
| **Log streaming** | `cf logs` via Firehose WebSocket | `mf logs` via K8s pod log stream |
| **Alerting** | External (via nozzles) | AlertManager (`pkg/monitoring/alertmanager.go`) |
| **Auto-instrumentation** | None (apps must emit metrics) | Beyla eBPF (zero-code RED metrics) |
| **CSP-native** | Not built-in (requires third-party nozzles) | Configurable URLs -> CloudWatch/Cloud Monitoring/Azure Monitor |

**Key difference**: CF has a complex custom log pipeline (Loggregator) that requires nozzles for external integration. MicroFoundry uses standard open-source tools (Prometheus/Grafana/Loki) and can swap to CSP-native monitoring by changing URL configuration — no code changes needed.

### 7. Service Brokers / Backing Services

| | Cloud Foundry | MicroFoundry |
|---|---|---|
| **Protocol** | OSBAPI v2 (industry standard) | OSBAPI-inspired (built-in catalog) |
| **Catalog** | External brokers register via `cf create-service-broker` | Built-in catalog (`pkg/service/catalog.go`) with 10 service types |
| **Provisioning** | Broker HTTP endpoint handles provisioning | Direct K8s provisioning (StatefulSet + PVC) or Terraform |
| **Binding** | Broker returns credentials -> VCAP_SERVICES | `pkg/service/binder.go` -> VCAP_SERVICES K8s Secret |
| **VCAP_SERVICES** | Auto-injected env var | Auto-injected env var (`pkg/service/vcap.go`) |
| **User-provided services** | `cf cups` | `mf create-secret` |
| **Terraform integration** | None | `pkg/terraform/executor.go` for complex topologies |
| **Services** | Any (via external brokers) | MariaDB, PostgreSQL, Redis, RabbitMQ, MinIO, Kong, Elasticsearch, OpenSearch, InfluxDB, MongoDB |

**Key difference**: CF requires external service brokers to be developed and operated. MicroFoundry has a built-in catalog that provisions services directly as K8s StatefulSets — simpler but less extensible. Both inject credentials via VCAP_SERVICES.

### 8. Build System / Staging

| | Cloud Foundry | MicroFoundry |
|---|---|---|
| **Mechanism** | Buildpacks (detect -> compile -> release -> droplet) | Dockerfile or Cloud Native Buildpacks (`pkg/build/builder.go`) |
| **Runtime** | Diego Task (staging cell) | Docker build or `pack build` |
| **Output** | Droplet (tarball) | OCI container image |
| **Registry** | Internal blobstore | Configurable (Harbor, ECR, ACR, AR, Docker Hub) |
| **Multi-buildpack** | Supply + Finalize lifecycle | CNB layer composition |

**Key difference**: CF's buildpack system is proprietary (though it spawned CNB). MicroFoundry supports both Dockerfile and CNB, producing standard OCI images that work anywhere.

### 9. Multi-Tenancy Model

| | Cloud Foundry | MicroFoundry |
|---|---|---|
| **Top level** | Foundation (entire CF deployment) | Platform |
| **Middle level** | Organization | Workspace -> Organization |
| **Bottom level** | Space | (Organization is the bottom) |
| **Isolation** | Isolation Segments (dedicated Diego Cell pools) | Separate K8s clusters/namespaces |
| **Roles per level** | OrgManager, BillingManager, OrgAuditor, SpaceManager, SpaceDeveloper, SpaceAuditor, SpaceSupporter | platform-admin, workspace-admin, org-admin, org-member, viewer |
| **Audit** | CF events (API-level) | `pkg/auth/audit.go` — auth events, role changes, resource access |

**Key difference**: CF has a deeper hierarchy (3 levels) with more granular roles (8 vs 5). MicroFoundry adds Workspaces as an extra grouping above Orgs. CF has quotas at every level; MicroFoundry doesn't enforce quotas yet.

### 10. Infrastructure / Deployment Model

| | Cloud Foundry | MicroFoundry |
|---|---|---|
| **Orchestrator** | BOSH (manages VMs via CPI) | Kubernetes (manages containers) |
| **Base image** | Stemcell (hardened Ubuntu VM image) | Alpine Linux container (~15 MB) |
| **IaaS abstraction** | CPI (Cloud Provider Interface) for AWS/GCP/Azure/vSphere | K8s runs anywhere + Terraform modules per CSP |
| **Updates** | BOSH rolling update (VM-by-VM) | K8s rolling deployment |
| **HA** | BOSH resurrection + multiple VM instances | K8s ReplicaSet + liveness probes |
| **Networking** | Silk overlay (VXLAN) + ASGs | K8s CNI (Calico/Cilium/etc.) + NetworkPolicies |
| **Min. footprint** | 15-20 VMs (non-HA), 40-80+ VMs (production) | 1 container + existing K8s cluster |
| **DNS** | BOSH DNS | K8s CoreDNS |

**Key difference**: This is the most dramatic difference. CF requires BOSH to manage dozens of VMs across 12+ component types. MicroFoundry is a single binary that runs on any existing Kubernetes cluster. The operational overhead is orders of magnitude lower.

---

## CLI Experience

| CF CLI | MF CLI | Notes |
|--------|--------|-------|
| `cf push` | `mf push` | Both deploy from source/Dockerfile |
| `cf apps` | `mf apps` | List apps |
| `cf logs` | `mf logs` | Stream logs |
| `cf scale` | `mf scale` | Scale instances |
| `cf marketplace` | `mf catalog` | Browse services |
| `cf create-service` | `mf create-service` | Provision service |
| `cf bind-service` | `mf bind-service` | Bind service -> VCAP_SERVICES |
| `cf target -o/-s` | `mf workspaces` / `mf orgs` | Switch context |
| `cf ssh` | — | Not yet implemented |
| `cf set-env` | — | Via `mf push` config |
| `cf create-user-provided-service` | `mf create-secret` | Manual credentials |
| — | `mf admin` | Web UI (no CF equivalent) |
| — | `mf setup` | Interactive setup wizard |

---

## Web Admin UI

| | Cloud Foundry | MicroFoundry |
|---|---|---|
| **Built-in UI** | Apps Manager (proprietary, Tanzu only) | Built-in admin dashboard (48 HTML templates) |
| **Technology** | React SPA (commercial product) | Go templates + HTMX + Tailwind CSS |
| **Features** | App management, marketplace, org/space management | Dashboard, apps, services, catalog, secrets, clusters, monitoring, IAM, settings, audit logs |
| **Availability** | Commercial license (Tanzu Application Service) | Open source, included in the binary |

---

## Resource Comparison

| Metric | Cloud Foundry | MicroFoundry |
|--------|--------------|--------------|
| **Min. VMs/containers** | 40-80+ VMs | 1 container |
| **Languages** | Ruby, Go, Java | Go only |
| **Source files** | ~100+ repos, millions of lines | 100+ Go files, 48 HTML templates |
| **Components** | 12+ distinct systems | 1 binary, 16 Go packages |
| **Install time** | Hours (BOSH deploy) | Minutes (Helm install) |
| **Upgrade process** | BOSH rolling update (hours) | K8s rolling deployment (seconds) |
| **Container overhead per app** | Diego Cell overhead + Garden | Zero (native K8s pods) |
| **Monitoring** | Requires external nozzle integration | Integrated (Prometheus/Grafana/Loki/Beyla) |

---

## What MicroFoundry Borrows from CF

1. **`push` workflow** — source-to-running-app with one command
2. **VCAP_SERVICES / VCAP_APPLICATION** — environment variable injection for service credentials
3. **Org-based multi-tenancy** — hierarchical isolation with role-based access
4. **Service catalog with plans** — marketplace of backing services with tiered plans
5. **Buildpack support** — Cloud Native Buildpacks as first-class build option
6. **SCIM v2** — user provisioning protocol
7. **CLI command parity** — `push`, `apps`, `logs`, `scale`, `bind-service`, `catalog` mirror CF CLI
8. **Routes/domains** — domain-based routing with path support

---

## What MicroFoundry Does Differently

1. **Kubernetes-native** — no custom scheduler, no BOSH, no VMs; everything is K8s objects
2. **Single binary** — all components in one Go binary (~15 MB) vs 12+ distributed systems
3. **K8s as database** — ConfigMaps/Secrets as state store vs PostgreSQL/MySQL
4. **OPA authorization** — editable Rego policies vs hardcoded RBAC in Ruby
5. **Pluggable gateway** — choose Nginx/Kong/Traefik vs fixed Gorouter
6. **Built-in web UI** — server-rendered admin dashboard vs CLI-only (or commercial Apps Manager)
7. **eBPF monitoring** — Beyla auto-instrumentation vs no built-in app metrics
8. **Multi-cluster** — manage multiple K8s clusters from one instance
9. **CSP-native monitoring** — swap monitoring URLs per cloud provider
10. **Terraform integration** — IaC for complex service topologies
11. **Workloads survive removal** — deployed apps are standard K8s resources that persist without MF

---

## Architecture Diagrams

### Cloud Foundry (simplified)

```
Developer -> cf CLI -> Cloud Controller (Ruby) -> Diego BBS -> Auctioneer -> Diego Cells
                            |                         ^
                      PostgreSQL/MySQL           Route Emitter -> NATS -> Gorouter <- External LB
                            |
                      UAA (Java) <- LDAP/SAML
                            |
                      Loggregator (Metron -> Doppler -> Traffic Controller)
                            |
                      BOSH Director -> Stemcells -> VMs (40-80+)
```

### MicroFoundry (simplified)

```
Developer -> mf CLI -+
                     +-> mf binary (Go) -> Kubernetes API -> Pods/Deployments/Services/Ingress
Admin -> Web UI -----+       |                    ^
                     Keycloak (OIDC)    Ingress Controller (nginx/kong/traefik)
                     OPA (Rego)         Prometheus/Grafana/Loki/Beyla
                     Service Catalog    (or CSP-native: CloudWatch/Cloud Monitoring/Azure Monitor)
```

### The Fundamental Shift

```
Cloud Foundry                              MicroFoundry
==============                             ==============

BOSH Director                              Kubernetes
  |                                          |
  +-- VM: Cloud Controller (Ruby)           +-- Pod: mf binary (Go)
  +-- VM: Cloud Controller Worker                |
  +-- VM: Cloud Controller Clock                 +-- Admin Dashboard
  +-- VM: UAA (Java)                             +-- API Server
  +-- VM: Diego BBS                              +-- CLI Backend
  +-- VM: Diego Cell x N                         +-- Service Broker
  +-- VM: Auctioneer                             +-- Auth (OIDC + OPA)
  +-- VM: Gorouter x N                           +-- Monitoring Client
  +-- VM: NATS x N
  +-- VM: Doppler x N                      Keycloak Pod (external)
  +-- VM: Log Cache                        Ingress Controller (existing)
  +-- VM: Traffic Controller               Prometheus/Grafana/Loki (existing)
  +-- VM: CredHub
  +-- VM: MySQL (PXC Galera)
  +-- VM: Blobstore
  +-- VM: TCP Router
  = 40-80+ VMs                             = 1-2 Pods + existing infra
```

---

## Summary

MicroFoundry achieves ~80% of Cloud Foundry's developer experience with ~1% of the operational complexity, by standing on Kubernetes rather than building its own infrastructure layer. The trade-off: CF has deeper multi-tenancy controls (quotas, isolation segments, ASGs) and a more mature ecosystem of service brokers, while MicroFoundry offers faster deployment, lower operational overhead, built-in observability, and the flexibility to run on any Kubernetes cluster across any cloud provider.

---

## Related Documentation

- [CloudFoundry Architecture Reference](cloudfoundry-architecture.md) — Deep dive into CF's internal components
- [MicroFoundry Architecture](architecture.md) — Technical architecture and component design
- [User Manual](user-manual.md) — Complete guide to deploying and managing applications

---

*This comparison is based on CloudFoundry `cf-deployment` v54.9.0 / CF CLI v9 and MicroFoundry v0.1.0.*
