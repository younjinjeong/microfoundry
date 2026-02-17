# CloudFoundry Architecture Reference

> Deep analysis of CloudFoundry's architecture, component relationships, and core concepts.
> Generated from `cloudfoundry/cli` (v9) and `cloudfoundry/cf-deployment` (v54.9.0).
> This document serves as the architectural blueprint for MicroFoundry.

---

## Table of Contents

1. [High-Level Architecture](#1-high-level-architecture)
2. [Component Topology](#2-component-topology)
3. [CLI Architecture (Command → Actor → API)](#3-cli-architecture)
4. [Cloud Controller (CAPI)](#4-cloud-controller-capi)
5. [Diego — Container Orchestration](#5-diego--container-orchestration)
6. [Router (Gorouter & TCP Router)](#6-router-gorouter--tcp-router)
7. [Loggregator — Logging & Metrics Pipeline](#7-loggregator--logging--metrics-pipeline)
8. [UAA — Authentication & Authorization](#8-uaa--authentication--authorization)
9. [Service Binding & Service Catalog](#9-service-binding--service-catalog)
10. [Buildpack Lifecycle (cf push)](#10-buildpack-lifecycle-cf-push)
11. [Networking (Silk / Container-to-Container)](#11-networking-silk--container-to-container)
12. [Multi-Tenancy Model](#12-multi-tenancy-model)
13. [Data Model & Resource Types](#13-data-model--resource-types)
14. [Backing Services & Databases](#14-backing-services--databases)
15. [Certificate & TLS Structure](#15-certificate--tls-structure)
16. [Network Ports & Protocols](#16-network-ports--protocols)
17. [Key Architectural Patterns](#17-key-architectural-patterns)
18. [MicroFoundry Mapping](#18-microfoundry-mapping)

---

## 1. High-Level Architecture

CloudFoundry is a Platform-as-a-Service (PaaS) that abstracts infrastructure and provides:
- **Application lifecycle management** (push, scale, update, delete)
- **Service marketplace** (bind backing services to apps via Open Service Broker API)
- **HTTP/TCP routing** (automatic route management and load balancing)
- **Multi-tenancy** (organizations, spaces, quotas, roles)
- **Logging & metrics** (centralized log aggregation and Prometheus-compatible metrics)

### Architecture Layers

```
┌──────────────────────────────────────────────────────────────┐
│                     CLI / API Clients                        │
│              (cf CLI, API consumers, CI/CD)                  │
└──────────────┬───────────────────────────────┬───────────────┘
               │                               │
┌──────────────▼───────────┐   ┌───────────────▼──────────────┐
│    Gorouter (HTTP/HTTPS) │   │    TCP Router (TCP traffic)  │
│    *.system_domain       │   │    Ports 1024-1033           │
└──────────────┬───────────┘   └───────────────┬──────────────┘
               │                               │
┌──────────────▼───────────────────────────────▼──────────────┐
│                    Cloud Controller (CAPI)                    │
│           API server, workers, clock, deployment updater     │
└───────┬──────────┬──────────┬──────────┬────────────────────┘
        │          │          │          │
┌───────▼──┐ ┌────▼────┐ ┌──▼────┐ ┌───▼──────┐
│  Diego   │ │   UAA   │ │ NATS  │ │ Blobstore│
│ (cells,  │ │ (auth)  │ │ (msg) │ │ (assets) │
│  BBS,    │ │         │ │       │ │          │
│  auction)│ │         │ │       │ │          │
└───────┬──┘ └────┬────┘ └──┬────┘ └───┬──────┘
        │         │         │          │
┌───────▼─────────▼─────────▼──────────▼──────────────────────┐
│                    MySQL (PXC Galera)                         │
│  cloud_controller | diego | uaa | routing-api | locket       │
│  network_policy | network_connectivity | credhub              │
└──────────────────────────────────────────────────────────────┘
```

---

## 2. Component Topology

CloudFoundry deploys **17 instance groups** across 2 availability zones (z1, z2):

| Instance Group | Instances | AZs | Key Jobs | Purpose |
|----------------|-----------|-----|----------|---------|
| **nats** | 2 | z1,z2 | nats-tls | Message bus for route registration and component communication |
| **database** | 1 | z1 | pxc-mysql, proxy, galera-agent | MySQL 8.0 with Galera clustering, 8 databases |
| **diego-api** | 2 | z1,z2 | bbs, silk-controller, locket | Container state store (BBS), network controller, distributed locks |
| **uaa** | 2 | z1,z2 | uaa, route_registrar | OAuth2/OIDC identity server |
| **singleton-blobstore** | 1 | z1 | blobstore (WebDAV) | Stores buildpacks, packages, droplets (100GB disk) |
| **api** | 2 | z1,z2 | cloud_controller_ng, routing-api, policy-server, buildpacks, file_server, valkey | Main CF API, routing API, network policies, buildpack storage |
| **cc-worker** | 2 | z1,z2 | cloud_controller_worker | Background jobs (uploads, deletions, staging) |
| **scheduler** | 2 | z1,z2 | auctioneer, cc_clock, ssh_proxy, tps, service-discovery-controller | Task scheduling, SSH access, service discovery |
| **router** | 2 | z1,z2 | gorouter | HTTP/HTTPS request routing |
| **tcp-router** | 2 | z1,z2 | tcp_router | TCP traffic routing (ports 1024-1033) |
| **diego-cell** | 3 | z1,z2 | rep, garden, route_emitter, silk-daemon, vxlan-policy-agent | Container runtime (runs applications) |
| **log-cache** | 1 | z1,z2 | log-cache, log-cache-gateway, log-cache-syslog-server | In-memory log/metrics cache |
| **doppler** | 3 | z1,z2 | doppler | Log aggregation from all VMs |
| **log-api** | 2 | z1,z2 | loggregator_trafficcontroller, reverse_log_proxy | Log streaming API (Firehose) |
| **credhub** | 2 | z1,z2 | credhub | Credential management and generation |
| **smoke-tests** | 1 | z1 | smoke_tests | Errand: validates deployment |
| **rotate-cc-database-key** | 1 | z1 | rotate_cc_database_key | Errand: rotates CC encryption key |

### Addons (deployed to ALL VMs)

| Addon | Purpose |
|-------|---------|
| **loggregator_agent** | Collects container logs, forwards to Doppler via gRPC-TLS |
| **forwarder_agent** | Forwards logs from local agents to the pipeline |
| **loggr-syslog-agent** | Syslog binding for applications |
| **prom_scraper** | Scrapes Prometheus /metrics endpoints every 60s from all jobs |
| **bpm** | BOSH Process Manager (manages job lifecycle) |
| **bosh-dns-aliases** | DNS service discovery (maps *.service.cf.internal to instance groups) |

---

## 3. CLI Architecture

The CF CLI uses a **4-layer architecture**: Command → Actor → API Client → HTTP.

### Layer 1: Commands (`command/v7/`)

207+ commands organized by functional area. Each command is a Go struct with flags defined via struct tags:

```go
type BindServiceCommand struct {
    BaseCommand                          // SharedActor, Config, UI, Actor injection
    RequiredArgs    flag.BindServiceArgs // Positional: app-name, service-name
    BindingName     flag.BindingName     // --binding-name
    ParametersAsJSON flag.Path           // -c (JSON parameters)
    Wait            bool                 // --wait
}
```

**Command categories**: Applications (push, delete, scale, logs, ssh), Services (create-service, bind-service, marketplace), Routes (create-route, map-route), Orgs/Spaces, Buildpacks, Security Groups, Networking Policies, Deployments/Revisions, Admin/Auth.

### Layer 2: Actors (`actor/v7action/`)

Business logic layer that coordinates multi-step operations:

```go
type Actor struct {
    CloudControllerClient  CloudControllerClient  // CC V3 API
    Config                 Config                  // User config
    SharedActor            SharedActor             // File utilities
    UAAClient              UAAClient               // Auth tokens
    RoutingClient          RoutingClient            // Routing API
    Clock                  clock.Clock             // Time (testable)
    AuthActor              AuthActor               // K8s or default auth
}
```

Actors handle: resource lookup/validation, sequential operation chaining (`railway.Sequentially()`), async job polling, error translation.

### Layer 3: API Clients (`api/cloudcontroller/ccv3/`)

HTTP client for the Cloud Controller V3 API with 170+ endpoints:

```go
// Pattern: MakeRequest with typed parameters
_, warnings, err := client.MakeRequest(RequestParams{
    RequestName:  internal.PostServiceInstanceRequest,
    RequestBody:  serviceInstance,
    URIParams:    internal.Params{"service_instance_guid": guid},
    Query:        []Query{{Key: SpaceGUIDFilter, Values: []string{spaceGUID}}},
    ResponseBody: &result,
})
```

Additional API clients: **UAA** (OAuth2 authentication), **CF Networking** (container-to-container policies), **Routing** (route registration).

### Layer 4: HTTP Transport

Connection wrappers handle: TLS, retry on auth failure, automatic token refresh, request/response logging.

### CLI → Platform Relationship

```
cf push myapp
  → Command: PushCommand.Execute()
    → Actor: CreateAndUploadBitsPackageByApplicationNameAndSpace()
      → CC Client: POST /v3/packages → POST /v3/packages/:id/upload
    → Actor: StagePackage()
      → CC Client: POST /v3/builds → Poll GET /v3/builds/:id
    → Actor: SetApplicationDroplet()
      → CC Client: PATCH /v3/apps/:id/relationships/current_droplet
    → Actor: CreateRoute() + MapRoute()
      → CC Client: POST /v3/routes → POST /v3/routes/:id/destinations
```

---

## 4. Cloud Controller (CAPI)

The Cloud Controller is the **central API server** for CloudFoundry. It exposes the V3 REST API and manages all platform resources.

### Components

| Job | Instance Group | Purpose |
|-----|---------------|---------|
| **cloud_controller_ng** | api (x2) | Main API server |
| **cloud_controller_worker** | cc-worker (x2) | Background job processing |
| **cloud_controller_clock** | scheduler (x2) | Periodic tasks (quota rollup, state reconciliation) |
| **cc_deployment_updater** | scheduler (x2) | Rolling deployment status updates |
| **cc_uploader** | api (x2) | Handles buildpack/package uploads from Diego cells |
| **tps** | scheduler (x2) | Task/Process/Staging status bridge (CC ↔ BBS) |

### Key Responsibilities

1. **Application Management**: CRUD for apps, processes, environment variables
2. **Package/Build/Droplet Pipeline**: Manages the cf push lifecycle
3. **Service Orchestration**: Coordinates with service brokers via OSBAPI
4. **Route Management**: Creates/manages routes and domains
5. **Multi-Tenancy**: Organizations, spaces, quotas, roles
6. **Security Groups**: ASG rules for application network egress

### Integrations

- **Database**: MySQL (`cloud_controller` DB) for all state
- **Blobstore**: WebDAV for buildpacks, packages, droplets, resource_pool
- **Diego (BBS)**: Sends DesiredLRP requests for app scheduling
- **UAA**: Token validation and user lookup
- **CredHub**: Service key credential storage
- **NATS**: Route registration via route_registrar

### System Buildpacks (installed in order)

1. staticfile, 2. java, 3. ruby, 4. dotnet-core, 5. nodejs, 6. go, 7. python, 8. php, 9. nginx, 10. r, 11. binary

**Default stack**: `cflinuxfs4` (Ubuntu 22.04)

---

## 5. Diego — Container Orchestration

Diego is CloudFoundry's container orchestration system (analogous to Kubernetes). It manages the lifecycle of application containers.

### Components

```
┌─────────────────────────────────────────────┐
│              Cloud Controller               │
│         (creates DesiredLRPs)               │
└──────────────────┬──────────────────────────┘
                   │
┌──────────────────▼──────────────────────────┐
│                BBS                           │
│    (Bulletin Board System)                   │
│    Stores: DesiredLRP, ActualLRP, Tasks      │
│    Database: MySQL (diego DB)                │
│    Encryption: AES (diego_bbs_encryption)    │
└──────────┬───────────────┬──────────────────┘
           │               │
┌──────────▼────┐   ┌─────▼────────────────────┐
│  Auctioneer   │   │     Locket               │
│  (scheduler)  │   │  (distributed locks)      │
│  Bids cells   │   │  Ensures singletons       │
│  for placement│   │  Database: MySQL (locket)  │
└──────────┬────┘   └──────────────────────────┘
           │
┌──────────▼──────────────────────────────────┐
│              Diego Cell (x3)                 │
│  ┌──────────┐  ┌─────────┐  ┌────────────┐ │
│  │   Rep    │  │ Garden  │  │ Route      │ │
│  │ (agent)  │  │ (runc)  │  │ Emitter    │ │
│  │          │  │         │  │ → NATS      │ │
│  └──────────┘  └─────────┘  └────────────┘ │
│  ┌──────────────────────────────────────┐   │
│  │ Silk Daemon + VXLAN Policy Agent     │   │
│  │ (container-to-container networking)   │   │
│  └──────────────────────────────────────┘   │
└─────────────────────────────────────────────┘
```

### Key Concepts

- **DesiredLRP** (Long-Running Process): Describes *what should be running* (app instances, memory, disk, routes). Created by Cloud Controller.
- **ActualLRP**: Describes *what is actually running* (container state, IP, port). Maintained by Rep agents.
- **Task**: One-off job execution (e.g., staging, running a task via `cf run-task`).
- **Auction**: Auctioneer sends bid requests to all Reps. Each Rep evaluates available resources and bids. Lowest-cost cell wins the placement.

### Container Runtime

- **Garden**: Container runtime using `containerd` (runc under the hood)
- **CNI**: External networking via Silk CNI plugin
- **Instance Identity**: Each container gets a unique identity certificate signed by `diego_instance_identity_ca`
- **Container Proxy**: Sidecar that handles mTLS between router and app containers

### Cell Job List

| Job | Purpose |
|-----|---------|
| **rep** | Cell agent — accepts work from Auctioneer, manages containers |
| **garden** | Container runtime (containerd mode) |
| **route_emitter** | Watches BBS, announces routes to NATS |
| **garden-cni** | CNI plugin integration for container networking |
| **silk-daemon** | VXLAN overlay for container-to-container communication |
| **silk-cni** | CNI driver (DNS: 169.254.0.2) |
| **vxlan-policy-agent** | Enforces network policies from policy-server |
| **bosh-dns-adapter** | DNS resolution for apps.internal domain |
| **cflinuxfs4-rootfs-setup** | Root filesystem for Linux containers |

---

## 6. Router (Gorouter & TCP Router)

### Gorouter (HTTP/HTTPS)

Gorouter is a reverse proxy that routes HTTP/HTTPS traffic to application containers.

**How routes are discovered**:
1. **Route Emitter** on each Diego Cell watches BBS for ActualLRP changes
2. When a container starts/stops, Route Emitter publishes `route.register`/`route.unregister` to **NATS**
3. **Gorouter** subscribes to NATS and maintains an in-memory route table
4. Incoming requests matched by Host header → forwarded to container IP:port

**Key configuration**:
- Ports: 80 (HTTP), 443 (HTTPS), 8443 (health check)
- TLS: `router_ssl` certificate covers `*.system_domain` + `system_domain`
- Backend TLS: mTLS to app containers via `gorouter_backend_tls`
- Tracing: Zipkin enabled
- Route source: NATS messages + Routing API (for TCP routes)

### TCP Router

Routes non-HTTP TCP traffic on ports 1024-1033 (configurable via routing-api):
- Router group: `default-tcp`
- Backend TLS: mTLS via `tcp_router_backend_tls`
- Route discovery: Polls Routing API (not NATS)

### Routing API

Manages TCP route registrations, stored in MySQL (`routing-api` DB). Provides router groups and port assignments.

---

## 7. Loggregator — Logging & Metrics Pipeline

```
┌────────────────────┐
│ App Container       │ stdout/stderr
│ (on Diego Cell)     │──────┐
└────────────────────┘      │
                             ▼
┌────────────────────────────────────┐
│ loggregator_agent (addon on cell)  │ gRPC-TLS
│ Port: 3459                         │──────┐
└────────────────────────────────────┘      │
                                            ▼
┌──────────────────────────────────────────────┐
│              Doppler (x3)                     │
│  Aggregates logs from all agents             │
│  Port: 3458 (gRPC)                           │
└──────────┬───────────────────┬───────────────┘
           │                   │
┌──────────▼────┐    ┌────────▼──────────────┐
│ Log Cache (x1)│    │ Traffic Controller (x2)│
│ In-memory     │    │ Firehose API           │
│ /metrics cache│    │ Port: 8081 (HTTPS)     │
└──────────┬────┘    └────────┬──────────────┘
           │                  │
           ▼                  ▼
    cf logs myapp       Firehose consumers
    (recent/stream)     (monitoring tools)
```

### Components

| Component | Purpose | Port |
|-----------|---------|------|
| **loggregator_agent** | Addon: collects logs from containers on each VM | 3459 (gRPC) |
| **forwarder_agent** | Addon: forwards logs to pipeline | - |
| **loggr-syslog-agent** | Addon: syslog binding for external drain | 3460 |
| **prom_scraper** | Addon: scrapes Prometheus /metrics every 60s | - |
| **doppler** | Aggregates all logs from agents | 3458 (gRPC) |
| **log-cache** | In-memory cache for recent logs/metrics | 8081 |
| **loggregator_trafficcontroller** | Firehose API and app log streaming | 8081 (HTTPS) |
| **reverse_log_proxy** | gRPC gateway to log consumers | - |

### Metrics Flow

```
Job (e.g., CC, BBS, Rep)
  └→ /metrics endpoint (Prometheus format)
       └→ prom_scraper (every 60s)
            └→ loggregator_agent (convert to envelope)
                 └→ doppler (aggregate)
                      └→ log-cache (store)
                           └→ API consumers / Grafana
```

---

## 8. UAA — Authentication & Authorization

UAA (User Account and Authentication) is CloudFoundry's OAuth2/OIDC identity server.

### Key Features

- **OAuth2 Grant Types**: password, client_credentials, authorization_code, jwt_bearer, refresh_token
- **JWT Signing**: RSA key pair (`uaa_jwt_signing_key`) + symmetric key
- **SAML**: Integration via `uaa_login_saml` certificate
- **Encryption**: Data encryption with `uaa_default_encryption_passphrase`
- **Database**: MySQL (`uaa` DB)

### OAuth2 Clients (18 configured)

| Client | Grant Type | Used By |
|--------|------------|---------|
| **cf** | password, refresh_token, jwt_bearer | CF CLI (end users) |
| **gorouter** | client_credentials | Router: reads routes |
| **cc_routing** | client_credentials | CC: reads router groups |
| **cc-service-dashboards** | client_credentials | CC: manages service dashboard SSO |
| **cc_service_key_client** | client_credentials | CC: reads/writes CredHub |
| **cloud_controller_username_lookup** | client_credentials | CC: resolves usernames |
| **doppler** | client_credentials | Log API: resource access |
| **ssh-proxy** | authorization_code | SSH: authenticates users |
| **routing_api_client** | client_credentials | Routing API: reads/writes routes |
| **network-policy** | client_credentials | Network policies: admin read |
| **tcp_emitter** | client_credentials | TCP routing: writes routes |
| **tcp_router** | client_credentials | TCP router: reads routes |
| **cf_smoke_tests** | client_credentials | Smoke tests: admin access |
| **credhub_admin_client** | client_credentials | CredHub: full access |

### Authentication Flow

```
1. cf login → POST /oauth/token (password grant)
2. UAA validates credentials
3. Returns: access_token (JWT) + refresh_token
4. CLI includes Bearer token in all API calls
5. CC validates JWT signature against UAA public key
6. Token refresh: POST /oauth/token (refresh_token grant)
```

---

## 9. Service Binding & Service Catalog

CloudFoundry implements the **Open Service Broker API (OSBAPI)** for backing service integration.

### Concepts

```
Service Broker ──registers──→ Cloud Controller
     │                              │
     │ catalog                      │ stores
     ▼                              ▼
Service Offerings            Service Plans
  (e.g., "postgresql")        (e.g., "small", "large")
                                    │
                            creates  │
                                    ▼
                            Service Instance
                           (provisioned resource)
                                    │
                              binds │
                                    ▼
                       Service Credential Binding
                      (credentials injected into app)
```

### Resource Types

**Service Broker**:
```go
type ServiceBroker struct {
    GUID, Name, URL string
    CredentialsType  string    // "basic"
    Username, Password string
    SpaceGUID        string    // Empty for global, set for space-scoped
}
```

**Service Offering** (from broker's `/v2/catalog`):
```go
type ServiceOffering struct {
    GUID, Name, Description string
    ServiceBrokerGUID       string
    AllowsInstanceSharing   bool
    Tags                    []string
}
```

**Service Plan**:
```go
type ServicePlan struct {
    GUID, Name, Description  string
    Available                bool
    VisibilityType           string  // "public", "admin", "organization", "space"
    Free                     bool
    Costs                    []ServicePlanCost
    ServiceOfferingGUID      string
    MaintenanceInfoVersion   string
}
```

**Service Instance**:
```go
type ServiceInstance struct {
    Type            string  // "managed" or "user-provided"
    GUID, Name      string
    SpaceGUID       string
    ServicePlanGUID string  // For managed instances
    Tags, Credentials, Parameters  interface{}
    SyslogDrainURL, RouteServiceURL, DashboardURL string
    LastOperation   LastOperation
}
```

**Service Credential Binding**:
```go
type ServiceCredentialBinding struct {
    Type                string  // "app" or "key"
    GUID, Name          string
    ServiceInstanceGUID string
    AppGUID             string
    LastOperation       LastOperation
    Parameters          interface{}
    Strategy            string  // "single" or "multiple"
}
```

### Complete Service Lifecycle

#### Step 1: Register Broker
```
cf create-service-broker mybroker admin pass https://broker.example.com
  → CC: POST /v3/service_brokers → Poll job
  → CC calls broker: GET /v2/catalog
  → CC stores offerings + plans
```

#### Step 2: Enable Access
```
cf enable-service-access postgresql
  → CC: Sets plan visibility to "public"
```

#### Step 3: Create Service Instance
```
cf create-service postgresql small mydb -c '{"version":"14"}'
  → Actor: Resolve offering → plan → plan GUID
  → CC: POST /v3/service_instances → Poll job
  → CC calls broker: PUT /v2/service_instances/{id}
  → Broker provisions the database
```

#### Step 4: Bind to Application
```
cf bind-service myapp mydb
  → Actor: Resolve app GUID + instance GUID
  → CC: POST /v3/service_credential_bindings → Poll job
  → CC calls broker: PUT /v2/service_instances/{id}/service_bindings/{bid}
  → Broker creates credentials
  → Credentials stored in CC, injected into app's VCAP_SERVICES env var
```

#### Step 5: Unbind
```
cf unbind-service myapp mydb
  → CC calls broker: DELETE /v2/service_instances/{id}/service_bindings/{bid}
  → Binding removed from app environment
```

#### Step 6: Delete Instance
```
cf delete-service mydb
  → CC calls broker: DELETE /v2/service_instances/{id}
  → Broker deprovisions the resource
```

### User-Provided Services

No broker involved — credentials set directly:
```
cf create-user-provided-service myups -p '{"uri":"postgres://..."}'
cf bind-service myapp myups
```

### Route Services

Bind a service to a route (traffic passes through service before reaching app):
```
cf bind-route-service example.com --hostname myapp myproxy-service
  → Broker returns route_service_url
  → Router forwards requests through the service first
```

---

## 10. Buildpack Lifecycle (cf push)

### End-to-End Push Flow

```
┌──────────────┐
│ cf push myapp│
└──────┬───────┘
       │
       ▼
┌──────────────────────────────────────┐
│ Phase 1: PACKAGE CREATION            │
│  1. Gather local files               │
│  2. Fingerprint resources             │
│  3. Create Package (type: bits)       │
│  4. Upload only new/changed files     │
│  5. Poll until Package state = READY  │
└──────────────────┬───────────────────┘
                   │
                   ▼
┌──────────────────────────────────────┐
│ Phase 2: STAGING (Build)             │
│  1. Create Build from Package        │
│  2. Diego Cell runs staging task:    │
│     a. Download Package from Blob    │
│     b. Run buildpack detect (order): │
│        staticfile → java → ruby →    │
│        dotnet → nodejs → go →        │
│        python → php → nginx →        │
│        r → binary                    │
│     c. First match wins              │
│     d. Buildpack compile phase       │
│     e. Buildpack release phase       │
│  3. Poll Build until state = STAGED  │
│  4. Droplet created (compiled app)   │
└──────────────────┬───────────────────┘
                   │
                   ▼
┌──────────────────────────────────────┐
│ Phase 3: DEPLOYMENT                  │
│  1. Set current droplet on app       │
│  2. Create/update Process (web type) │
│  3. CC creates DesiredLRP in BBS     │
│  4. Auctioneer places on cell        │
│  5. Rep creates container via Garden │
│  6. Container starts with droplet    │
└──────────────────┬───────────────────┘
                   │
                   ▼
┌──────────────────────────────────────┐
│ Phase 4: ROUTING                     │
│  1. Route Emitter detects new LRP    │
│  2. Publishes route.register to NATS │
│  3. Gorouter adds to route table     │
│  4. Traffic flows to container       │
└──────────────────────────────────────┘
```

### Key Types in the Pipeline

| Type | States | Purpose |
|------|--------|---------|
| **Package** | awaiting_upload → ready → expired/failed | Uploaded app source code or Docker image reference |
| **Build** | staging → staged → failed | Compilation process (runs buildpacks) |
| **Droplet** | processing → staged → expired/failed | Compiled, runnable artifact (code + deps + runtime) |
| **Process** | - | Instance configuration (type, instances, memory, disk, health check) |

### Docker Support

```
cf push myapp --docker-image ghcr.io/org/image:tag
  → Package type = "docker" (no upload, just image reference)
  → No staging — Docker lifecycle uses image directly
  → Droplet references the Docker image
```

### Cloud Native Buildpack (CNB) Support

```
lifecycle_bundles:
  cnb/cflinuxfs4: cnb_app_lifecycle/cnb_app_lifecycle.tgz
```

---

## 11. Networking (Silk / Container-to-Container)

### Architecture

```
┌─────────────────────────────────┐
│ Policy Server (on api group)    │ ← cf add-network-policy
│ Database: network_policy (MySQL)│
└──────────────┬──────────────────┘
               │ policies
               ▼
┌──────────────────────────────────┐
│ VXLAN Policy Agent (on each cell)│
│ Reads policies, enforces rules   │
└──────────────┬───────────────────┘
               │
┌──────────────▼───────────────────┐
│ Silk Daemon (on each cell)       │
│ VXLAN overlay network            │
│ Container IP allocation          │
└──────────────┬───────────────────┘
               │
┌──────────────▼───────────────────┐
│ Silk Controller (on diego-api)   │
│ Central network state            │
│ Database: network_connectivity   │
└──────────────────────────────────┘
```

### Network Policy

```go
type Policy struct {
    Source      PolicySource      // App GUID
    Destination PolicyDestination // App GUID + Protocol + Ports
}
```

Example: Allow `frontend` → `backend` on port 8080:
```
cf add-network-policy frontend backend --port 8080 --protocol tcp
```

### Service Discovery (apps.internal)

- Internal domain: `apps.internal`
- DNS queries routed via `bosh-dns-adapter` → `service-discovery-controller`
- Apps can reach each other by `app-name.apps.internal` without going through Gorouter
- Used for internal microservice communication

---

## 12. Multi-Tenancy Model

```
Cloud Foundry Platform
├── Organizations (tenants)
│   ├── Org Quotas (memory, routes, service instances, app instances)
│   ├── Private Domains (e.g., company.internal)
│   ├── Isolation Segments (dedicated Diego cells)
│   └── Spaces (environments within an org)
│       ├── Space Quotas (subset of org quota)
│       ├── Applications
│       │   ├── Processes (web, worker, etc.)
│       │   ├── Instances (scaled containers)
│       │   ├── Environment Variables
│       │   └── Service Bindings (→ VCAP_SERVICES)
│       ├── Service Instances
│       │   ├── Managed (via broker)
│       │   ├── User-Provided (direct credentials)
│       │   └── Shared (across spaces)
│       ├── Routes (host.domain.com/path)
│       └── Security Groups (egress firewall rules)
│
├── Shared Domains (e.g., apps.example.com)
├── System Buildpacks (global)
└── Service Brokers (global or space-scoped)
```

### Roles

- **Admin**: Platform-wide admin
- **Org Manager**: Manages org users, spaces, quotas
- **Org Auditor**: Read-only org access
- **Space Manager**: Manages space users and roles
- **Space Developer**: Deploys apps, binds services, creates routes
- **Space Auditor**: Read-only space access

---

## 13. Data Model & Resource Types

### Core Resources (from CLI `resources/` directory)

| Resource | Key Fields | Relationships |
|----------|-----------|---------------|
| **Application** | GUID, Name, State, LifecycleType (buildpack/docker/cnb), StackName | → Space |
| **Process** | GUID, Type (web/worker), Instances, MemoryInMB, DiskInMB, HealthCheckType | → Application |
| **Package** | GUID, Type (bits/docker), State, DockerImage | → Application |
| **Build** | GUID, State (staging/staged/failed), PackageGUID, DropletGUID | → Package |
| **Droplet** | GUID, State, Buildpacks, Stack, Image (docker) | → Application, → Build |
| **Route** | GUID, Host, Path, Port, Protocol, URL, Destinations[] | → Domain, → Space |
| **Domain** | GUID, Name, Internal, RouterGroup | → Organization (private) |
| **ServiceBroker** | GUID, Name, URL, Username, Password | → Space (space-scoped) |
| **ServiceOffering** | GUID, Name, Description, Tags | → ServiceBroker |
| **ServicePlan** | GUID, Name, Free, Available, VisibilityType, Costs[] | → ServiceOffering |
| **ServiceInstance** | GUID, Name, Type, Tags, Credentials, DashboardURL | → Space, → ServicePlan |
| **ServiceCredentialBinding** | GUID, Name, Type (app/key), Strategy | → ServiceInstance, → Application |
| **Organization** | GUID, Name, Suspended, QuotaGUID | |
| **Space** | GUID, Name | → Organization |
| **Buildpack** | GUID, Name, Position, Stack, Enabled, Locked | |
| **SecurityGroup** | GUID, Name, Rules[], GloballyEnabled | → Spaces (staging/running) |
| **Deployment** | GUID, State, Strategy, StatusValue, StatusReason | → Application |
| **Revision** | GUID, Version, Deployable, Description | → Application |

### JSON API Patterns

- **Relationships**: `{"relationships": {"space": {"data": {"guid": "..."}}}}}`
- **Included Resources**: API returns related objects to avoid N+1
- **Pagination**: `?page=1&per_page=50` with `pagination.next.href`
- **Filtering**: `?names=myapp&space_guids=...`
- **Labels/Annotations**: First-class metadata on all resources
- **Async Operations**: Return `JobURL` → poll `/v3/jobs/:guid`

---

## 14. Backing Services & Databases

### Internal MySQL (PXC Galera)

8 databases on a single MySQL 8.0 cluster:

| Database | Owner | Purpose |
|----------|-------|---------|
| **cloud_controller** | CC | Apps, orgs, spaces, services, bindings, buildpacks, routes |
| **diego** | BBS | DesiredLRPs, ActualLRPs, Tasks (encrypted with AES) |
| **uaa** | UAA | Users, OAuth clients, tokens, groups |
| **routing-api** | Routing API | TCP routes, router groups |
| **network_policy** | Policy Server | Container-to-container network policies |
| **network_connectivity** | Silk Controller | Container IP allocations |
| **locket** | Locket | Distributed locks (singleton jobs) |
| **credhub** | CredHub | Encrypted credentials and secrets |

### Blobstore (WebDAV)

100GB persistent disk storing:

| Store | Contents |
|-------|----------|
| **buildpacks** | System buildpack ZIP archives |
| **packages** | Application source code archives |
| **droplets** | Compiled application artifacts |
| **resource_pool** | Deduplicated file chunks |

Can be externalized to: S3, GCS, Azure Blob (via ops files).

### CredHub

Manages generated credentials:
- Passwords, certificates, RSA keys, SSH keys
- Service instance credentials
- Application secrets
- Encrypted at rest with AES

---

## 15. Certificate & TLS Structure

CloudFoundry uses **mTLS everywhere**. 74+ certificates organized by CA:

### Certificate Authorities (10+)

| CA | Protects |
|----|----------|
| **service_cf_internal_ca** | Most internal components |
| **loggregator_ca** | Doppler, agents, traffic controller |
| **router_ca** | Router SSL (public-facing) |
| **uaa_ca** | UAA SSL, SAML, JWT |
| **diego_instance_identity_ca** | Per-container identity certs |
| **nats_ca** / **nats_internal_ca** | NATS external/internal |
| **pxc_server_ca** / **pxc_galera_ca** | MySQL server/replication |
| **silk_ca** | Container networking |
| **network_policy_ca** | Network policy server |
| **log_cache_ca** | Log cache service |

### Public-Facing Certificates

| Certificate | Domains |
|-------------|---------|
| **router_ssl** | `*.system_domain`, `system_domain` |
| **cc_public_tls** | `api.system_domain` |
| **uaa_ssl** | `uaa.system_domain`, `login.system_domain` |
| **logcache_ssl** | `log-cache.system_domain` |
| **credhub_tls** | `credhub.system_domain` |

### Secrets & Keys

- 19 password variables
- 12 OAuth client secrets
- 3 encryption passphrases (BBS, CredHub, UAA)
- 2 SSH/RSA keys (SSH proxy host key, JWT signing key)

---

## 16. Network Ports & Protocols

| Component | Port | Protocol | Purpose |
|-----------|------|----------|---------|
| Gorouter | 80/443 | HTTP/HTTPS | Public app traffic |
| Gorouter Health | 8443 | HTTPS | Load balancer health checks |
| TCP Router | 1024-1033 | TCP | TCP app traffic |
| NATS | 4222 | TCP/TLS | Message bus |
| BBS | 8889 | gRPC-TLS | Diego state store |
| Locket | 8891 | gRPC-TLS | Distributed locks |
| Rep | 1801 | HTTPS | Cell agent |
| SSH Proxy | 2222 | SSH/TLS | `cf ssh` access |
| CC API | 9022/9024 | HTTP/HTTPS | Cloud Controller |
| Routing API | 3001 | HTTPS | Route management |
| Policy Server | 4002 | HTTPS | Network policies |
| UAA | 8443 | HTTPS | OAuth2/OIDC |
| CredHub | 8844 | HTTPS | Credential management |
| Blobstore | 4443 | HTTPS | WebDAV storage |
| MySQL | 3306 | TCP | Database |
| Doppler | 3458 | gRPC | Log aggregation |
| Log Cache | 8081 | HTTP | Log/metrics cache |
| Firehose | 8081 | HTTPS | Log streaming |
| Garden | 7777 | HTTP | Container API (local only) |
| prom_scraper | various | HTTPS | Prometheus metrics |

---

## 17. Key Architectural Patterns

### 1. Async Job Polling

Long-running operations return a `JobURL` immediately. Clients poll `/v3/jobs/:guid` until completion:
```
POST /v3/service_instances → 202 Accepted + Location: /v3/jobs/abc123
GET /v3/jobs/abc123 → {"state": "PROCESSING"}
GET /v3/jobs/abc123 → {"state": "COMPLETE"}
```

### 2. Railway Pattern (Sequential Operations)

Actors chain operations with early exit on error:
```go
warnings, err := railway.Sequentially(
    func() { serviceInstance = getServiceInstance(...) },
    func() { app = getApplication(...) },
    func() { binding = createBinding(serviceInstance, app) },
)
```

### 3. Open Service Broker API

Service brokers implement a standard API:
- `GET /v2/catalog` — List offerings and plans
- `PUT /v2/service_instances/{id}` — Provision
- `DELETE /v2/service_instances/{id}` — Deprovision
- `PUT /v2/service_instances/{id}/service_bindings/{bid}` — Bind
- `DELETE /v2/service_instances/{id}/service_bindings/{bid}` — Unbind

### 4. NATS Pub/Sub for Route Registration

All components with HTTP endpoints register routes via `route_registrar` → NATS → Gorouter. No central route database for HTTP (only for TCP via routing-api).

### 5. mTLS Everywhere

All internal communication uses mutual TLS. Each component has its own certificate signed by a shared CA. App containers get per-instance identity certificates.

### 6. Distributed Locking (Locket)

Singleton components (BBS master, Auctioneer leader, CC deployment updater) use Locket for distributed locking to ensure only one active instance.

### 7. Multi-Layer Error Handling

```
CC API errors → ccerror/ (HTTP status-specific)
  → Action errors → actionerror/ (175+ user-friendly error types)
    → Command layer (formatted for CLI output)
```

---

## 18. MicroFoundry Mapping

How CloudFoundry components map to MicroFoundry's Kubernetes-based architecture:

| CF Component | CF Technology | MicroFoundry Equivalent |
|-------------|---------------|------------------------|
| **Diego Cell** | Garden (containerd) | **Kubernetes Pod** (native container runtime) |
| **Diego BBS** | Custom state store | **Kubernetes etcd** (via K8s API) |
| **Auctioneer** | Custom scheduler | **Kubernetes Scheduler** (kube-scheduler) |
| **Gorouter** | Custom Go proxy | **Ingress Controller** (Kong/Nginx/API Gateway) |
| **TCP Router** | Custom TCP proxy | **K8s Service (LoadBalancer/NodePort)** |
| **NATS** | Message bus | **K8s Events / Controller watches** |
| **Cloud Controller** | Ruby/Go API | **MicroFoundry API Server** (Go) |
| **UAA** | Java OAuth2 server | **Keycloak OIDC + OPA (Rego) + SCIM v2** |
| **MySQL** | PXC Galera | **AWS RDS / Cloud SQL / CockroachDB** |
| **Blobstore** | WebDAV | **S3 / GCS / MinIO** |
| **Buildpacks** | CF Buildpacks | **Cloud Native Buildpacks (CNB) / Paketo** |
| **Loggregator** | Custom pipeline | **Fluentd/Fluentbit + Loki** or native K8s logging |
| **CredHub** | Custom vault | **K8s Secrets / HashiCorp Vault / AWS Secrets Manager** |
| **Silk (CNI)** | VXLAN overlay | **Kubernetes CNI** (Calico/Cilium, native) |
| **Policy Server** | Custom | **K8s NetworkPolicy** (native) |
| **Service Broker** | OSBAPI | **MicroFoundry Service Broker** (OSBAPI-compatible) |
| **Route Emitter** | NATS publisher | **K8s Ingress/Service reconciler** |
| **Locket** | Distributed locks | **K8s Leader Election** (client-go) |

### What MicroFoundry Keeps

- **cf push-like experience** → `mf push` with buildpack/Docker support
- **Service binding via OSBAPI** → Bind AWS RDS, BigQuery, etc.
- **Multi-tenancy** → Namespaces as spaces, RBAC as roles
- **Route management** → Ingress resources managed declaratively
- **Logging/Metrics** → Kubernetes-native (Fluentd + Prometheus)

### What MicroFoundry Simplifies

- **No BOSH** → Kubernetes replaces VM orchestration
- **No Diego** → Kubernetes replaces container orchestration
- **No NATS** → Kubernetes controller pattern replaces pub/sub
- **No Garden** → Kubernetes uses containerd directly
- **No custom networking** → Kubernetes CNI replaces Silk/VXLAN
- **Fewer certificates** → Kubernetes handles most internal TLS via service mesh

---

*This document was generated by analyzing `cloudfoundry/cli` (v9, Go) and `cloudfoundry/cf-deployment` (v54.9.0, BOSH). It serves as the reference architecture for MicroFoundry development.*
