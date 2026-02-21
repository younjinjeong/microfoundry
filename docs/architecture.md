# MicroFoundry Architecture

Technical architecture documentation for MicroFoundry — a micro CloudFoundry implementation on Kubernetes.

---

## Table of Contents

1. [System Overview](#system-overview)
2. [Component Architecture](#component-architecture)
3. [Application Deployment Flow](#application-deployment-flow)
4. [Kubernetes Resource Model](#kubernetes-resource-model)
5. [Service Provisioning](#service-provisioning)
6. [Observability Stack](#observability-stack)
7. [Authentication & Authorization](#authentication--authorization)
8. [Multi-Cluster Architecture](#multi-cluster-architecture)
9. [Platform Settings Storage](#platform-settings-storage)
10. [Security](#security)
11. [Documentation System](#documentation-system)
12. [Project Structure](#project-structure)

---

## System Overview

```
                          ┌─────────────────────────────────┐
                          │         Developer / AI          │
                          └──────────┬──────────┬───────────┘
                                     │          │
                              ┌──────▽──┐  ┌────▽──────┐
                              │  CLI    │  │  MCP      │
                              │ mf push │  │  Server   │
                              │ mf logs │  │ (Claude,  │
                              │ mf bind │  │  Cursor)  │
                              └──────┬──┘  └────┬──────┘
                                     │          │
                              ┌──────▽──────────▽──────┐
                              │  MicroFoundry API +    │
                              │  Admin Dashboard (:8080)│
                              └─────────────┬──────────┘
                                            │
          ┌─────────────────────────────────┼─────────────────────────────────┐
          │                                 │                                 │
  ┌───────▽────────┐              ┌─────────▽──────────┐            ┌────────▽────────┐
  │  Build System  │              │  Kubernetes API     │            │  Observability  │
  │                │              │                     │            │                 │
  │  Dockerfile    │              │  Deployments (Apps) │            │  Prometheus     │
  │  CNB/Paketo    │              │  Services (Network) │            │  Grafana        │
  │  pack CLI      │              │  Ingress (Routes)   │            │  Loki           │
  └────────────────┘              │  Secrets (Creds)    │            │  Beyla (eBPF)   │
                                  │  ConfigMaps         │            │  AlertManager   │
                                  └─────────┬──────────┘            └─────────────────┘
                                            │
                    ┌───────────────────────┼───────────────────────┐
                    │                       │                       │
            ┌───────▽───────┐       ┌───────▽───────┐      ┌───────▽───────┐
            │ Ingress       │       │ Backing       │      │ Settings      │
            │ Controller    │       │ Services      │      │ Store         │
            │ (Nginx/Kong)  │       │ (DB/Cache/MQ) │      │ (ConfigMap/   │
            └───────────────┘       └───────────────┘      │  Secret)      │
                                                           └───────────────┘
```

MicroFoundry is a single Go binary (`mf`) that acts as both a CLI tool and an HTTP server. It communicates directly with the Kubernetes API using `client-go` — there is no separate database or message queue for the control plane. Kubernetes itself is the single source of truth.

---

## Component Architecture

### Go Packages

```
pkg/
├── admin/           # HTTP server, handlers, templates (Admin UI + API)
│   ├── server.go    # Route registration, server lifecycle
│   ├── handlers.go  # App detail, dashboard, tab routing, docs handler
│   ├── api.go       # JSON API endpoints
│   ├── logs.go      # SSE log streaming
│   ├── templates.go # Go template renderer with clone pattern
│   ├── performance_handlers.go  # RED metrics integration
│   ├── service_handlers.go      # Service CRUD UI
│   ├── cluster_handlers.go      # Multi-cluster management
│   ├── monitoring_handlers.go   # Alerting & monitoring UI
│   ├── secret_handlers.go       # Secret management UI
│   ├── settings_handlers.go     # Registry, webhooks, SMTP, endpoints config
│   ├── topology_handlers.go     # Service topology visualization
│   ├── workspace_handlers.go    # Workspace CRUD UI + API
│   ├── org_handlers.go          # Organization management UI + API
│   ├── iam_handlers.go          # Keycloak user, policy, audit UI + API
│   ├── scim_handlers.go         # SCIM v2 protocol handlers
│   └── static/                  # Embedded HTML/CSS templates
├── auth/            # OIDC authentication (Keycloak)
│   ├── oidc.go      # Authorization code flow with PKCE
│   ├── session.go   # Cookie-based session management
│   ├── keycloak.go  # Keycloak realm/client configuration
│   ├── org.go       # Organization & member management
│   └── middleware.go# Auth middleware (InjectUser)
├── build/           # Source-to-image build
│   └── builder.go   # Dockerfile/CNB detection, docker build/push
├── config/          # Configuration loading
│   └── config.go    # Viper-based config with multi-cluster support
├── github/          # GitHub integration via gh CLI
│   ├── client.go    # gh CLI wrapper (exec-based, no API token needed)
│   ├── branches.go  # Branch listing and management
│   ├── issues.go    # Issue operations
│   └── pulls.go     # Pull request operations
├── hosts/           # /etc/hosts management
│   └── hosts.go     # Add/remove host entries for local dev
├── k8s/             # Kubernetes client operations
│   ├── client.go    # K8s API client wrapper
│   ├── app.go       # Deployment, Service, Pod operations
│   ├── ingress.go   # Ingress route management
│   ├── manager.go   # Multi-cluster ClientManager
│   ├── registry.go  # imagePullSecret management
│   └── keycloak.go  # Keycloak K8s deployment
├── manifest/        # CF manifest.yml parser
│   └── manifest.go  # Parse CF manifest, convert to MF models
├── models/          # Core data structures
│   ├── app.go       # App, AppDetail, AppListItem, InstanceStatus
│   ├── service.go   # ServiceType, ServicePlan, ServiceInstance
│   ├── cluster.go   # ClusterInfo, ClusterDetail, NodeInfo
│   ├── settings.go  # RegistryConfig, WebhookConfig, SMTPConfig
│   └── ...
├── monitoring/      # Observability integration
│   ├── prometheus.go    # Prometheus query client (RED metrics)
│   ├── grafana.go       # Grafana dashboard URL builder
│   ├── loki.go          # Loki log query client
│   ├── alertmanager.go  # AlertManager client
│   ├── metrics.go       # Custom Prometheus metrics
│   ├── collector.go     # Background metrics collector
│   └── middleware.go    # HTTP metrics middleware
├── secrets/         # Secret management
│   └── manager.go   # K8s Secret CRUD operations
├── service/         # Service broker
│   ├── catalog.go   # 56 service types (10 local + 21 AWS + 12 GCP + 13 Azure)
│   ├── manager.go   # Service lifecycle management
│   ├── provisioner.go # K8s-native provisioning
│   ├── binder.go    # VCAP_SERVICES injection
│   ├── vcap.go      # VCAP_SERVICES JSON formatting
│   └── visibility.go # Plan visibility toggle (ConfigMap-backed)
├── settings/        # Platform settings store
│   └── store.go     # ConfigMap/Secret-backed persistence
├── tls/             # TLS certificate generation
│   └── mkcert.go    # mkcert-based .dev HTTPS certificates
└── terraform/       # Terraform integration
    └── topology.go  # HCL topology management
```

### CLI Commands

```
cmd/mf/
├── main.go            # Root command, K8s client factory
├── push.go            # mf push — build → push → deploy → ingress → hosts
├── apps.go            # mf apps — list deployments
├── logs.go            # mf logs — Loki query + live pod streaming
├── scale.go           # mf scale — patch deployment replicas
├── delete.go          # mf delete — remove deployment + service + ingress
├── admin.go           # mf admin — start HTTP server
├── catalog.go         # mf catalog — print service catalog
├── create_service.go  # mf create-service — provision service
├── services.go        # mf services / mf service — list/detail
├── bind_service.go    # mf bind-service — bind to app
├── unbind_service.go  # mf unbind-service — unbind from app
├── delete_service.go  # mf delete-service — deprovision
├── secrets.go         # mf secrets / mf create-secret / mf delete-secret
├── users.go           # mf users / mf create-user — Keycloak user management
├── orgs.go            # mf orgs / mf create-org — organization management
├── workspaces.go      # mf workspaces / mf create-workspace — workspace management
├── login.go           # mf login — OIDC authentication
└── setup.go           # mf setup keycloak / keycloak-realm / keycloak-idp
```

---

## Application Deployment Flow

The `mf push` command implements a 5-phase deployment pipeline:

```
Source Code (Dockerfile or source)
         │
    Phase 1: Build
         │  build.DetectBuildStrategy() → Dockerfile or CNB
         │  build.NewBuilder(imagePrefix).Build(name, srcDir)
         │  → local Docker image: microfoundry/<app-name>:latest
         │
    Phase 1.5: Registry Push (optional)
         │  settings.NewStore(k8sClient).Get() → RegistryConfig
         │  builder.Login(url, username, password)
         │  builder.Push(imageRef)
         │  k8sClient.EnsureImagePullSecret()
         │  → image pushed to harbor.local:30003/microfoundry/<app-name>
         │
    Phase 2: Deploy to K8s
         │  k8sClient.EnsureNamespace()
         │  k8sClient.DeployApp(app, routes)
         │  → creates/updates: Deployment, Service
         │  → annotations: microfoundry.io/owner, lifecycle, guid, created-at
         │
    Phase 3: Create Ingress
         │  k8sClient.CreateIngress(name, routes)
         │  → creates: Ingress with nginx annotations
         │  → rule: <app-name>.<domain> → service:<port>
         │
    Phase 4: Update Hosts
         │  hosts.Add("<app-name>.<domain>")
         │  → appends: 127.0.0.1 <app-name>.cf-local.dev to /etc/hosts
         │
    Phase 5: Wait for Rollout
         │  k8sClient.WaitForRollout(name, 120s)
         │  → waits for deployment to reach desired replica count
         ▼
    Result: App accessible at http://<app-name>.cf-local.dev
```

### Kubernetes Resources Created per App

| Resource | Name | Purpose |
|----------|------|---------|
| `Deployment` | `<app-name>` | Pod management, replicas, image, env |
| `Service` | `<app-name>` | ClusterIP service for internal routing |
| `Ingress` | `<app-name>` | External routing via ingress controller |
| `Secret` | `mf-registry-pull` | imagePullSecret (when registry configured) |

### Deployment Annotations

MicroFoundry stores metadata as Kubernetes annotations on Deployments:

| Annotation | Example | Purpose |
|------------|---------|---------|
| `microfoundry.io/owner` | `younj` | Who deployed the app |
| `microfoundry.io/lifecycle` | `docker` | Build strategy used |
| `microfoundry.io/guid` | `uuid` | Unique app identifier |
| `microfoundry.io/created-at` | `2025-01-15T...` | Initial deploy timestamp |
| `microfoundry.io/buildpacks` | `paketo/go` | Buildpack (if CNB) |
| `microfoundry.io/disk-mb` | `1024` | Disk limit |
| `microfoundry.io/port` | `8080` | Container port |

---

## Kubernetes Resource Model

### App Data Extraction

The `pkg/k8s/app.go` module extracts all app information from Kubernetes resources:

- **Container spec**: image, command, resources (CPU/memory limits), env vars, health probes
- **Deployment metadata**: annotations, labels, creation time, replica count
- **Routes**: parsed from Ingress rules (host, domain, path, protocol)
- **Secrets**: K8s Secrets labeled with `microfoundry.io/app`
- **Instances**: Pod status (running/pending/failed), restart count, node assignment, pod name, image ID

### Enriched Models

```go
// AppDetail — 25+ field struct for app detail view
type AppDetail struct {
    Name, State, Org, Owner, LifecycleType string
    ImageRef, ImageDigest, GUID string
    Instances, MemoryMB, CPUMillis, DiskMB, Port int
    HealthCheckType string
    Routes []RouteDetail
    Env map[string]string
    Labels, Annotations map[string]string
    Services []ServiceBindingInfo
    Secrets []SecretInfo
    InstanceList []InstanceStatus
    CreatedAt, UpdatedAt time.Time
}

// AppListItem — denormalized view for list table
type AppListItem struct {
    Name, Org, Owner, State, LifecycleType string
    RunningCount, TotalCount, MemoryMB int
    Routes string
    CreatedAt time.Time
}
```

---

## Service Provisioning

### Architecture

```
mf create-service postgresql small my-db
         │
         ▼
┌─────────────────────┐
│  service.Manager    │  Orchestrates create/bind/unbind/delete
│  ├── catalog.go     │  56 types × 3 plans (10 local + 21 AWS + 12 GCP + 13 Azure)
│  ├── provisioner.go │  K8s resource creation
│  ├── binder.go      │  VCAP_SERVICES injection
│  └── vcap.go        │  CF-compatible JSON format
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│  Kubernetes         │
│  ├── Deployment     │  Service workload
│  ├── Service        │  ClusterIP networking
│  ├── PVC            │  Persistent storage (databases)
│  └── Secret         │  Credentials (mf-svc-<name>)
└─────────────────────┘
```

### Catalog Structure

Each service type defines:
- **ID** — `postgresql`, `redis`, `aws-rds-postgresql`, etc.
- **Provider** — `local` (K8s-native) or `aws` (CSP-managed via Terraform)
- **Category** — `database`, `cache`, `messaging`, `storage`, `gateway`
- **Plans** — `small` (dev), `medium` (staging), `large` (production)
- **Resources** — memory, CPU, storage allocations per plan

Local services (10) are provisioned as K8s StatefulSets with PVCs. AWS services (7) are provisioned via Terraform topology templates stored as ConfigMaps.

### Binding Flow

```
mf bind-service hello-world my-db
         │
         ▼
1. Look up service instance (K8s Deployment with label microfoundry.io/service)
2. Look up credentials (K8s Secret mf-svc-my-db)
3. Build VCAP_SERVICES JSON with credentials
4. Create/update binding secret with VCAP_SERVICES
5. Patch app Deployment to add envFrom referencing the binding secret
6. App automatically restarts with new env
```

### Plan Visibility

Service plan visibility is managed via a K8s ConfigMap (`mf-service-visibility`). Plans can be enabled/disabled per service type from the admin UI's Service Catalog page.

---

## Observability Stack

### Component Diagram

```
┌──────────────────────────────────────────────────────────────────┐
│  Observability Stack (monitoring namespace)                       │
│                                                                   │
│  ┌──────────┐    eBPF     ┌────────────┐   scrape  ┌──────────┐ │
│  │  App Pod  │◀───────────│  Beyla      │──────────▶│Prometheus│ │
│  │ (any lang)│  zero-code │  DaemonSet  │           │          │ │
│  └──────────┘  intercept  └────────────┘           └────┬─────┘ │
│       │                                                  │       │
│       │ stdout/stderr      ┌────────────┐         ┌─────▽─────┐ │
│       └───────────────────▶│  Promtail  │────────▶│  Grafana  │ │
│                            └────────────┘         │  + Loki   │ │
│                                                   └─────┬─────┘ │
│  Recording Rules (pre-computed RED):                    │       │
│    microfoundry:http_request_rate:5m                    │       │
│    microfoundry:http_error_rate:5m              ┌───────▽─────┐ │
│    microfoundry:http_latency_p50/p95/p99:5m     │AlertManager │ │
│                                                  └─────────────┘ │
└──────────────────────────────────────────────────────────────────┘
```

### Beyla eBPF Auto-Instrumentation

Grafana Beyla runs as a DaemonSet with privileged access to the kernel. It uses eBPF to intercept HTTP calls at the kernel level — providing request rate, error rate, and latency metrics for any application in any language without code changes.

This is inspired by Netflix's approach where the Atlas runtime agent auto-injects into JVM applications. MicroFoundry extends this to any language via eBPF.

### Recording Rules

Pre-computed recording rules aggregate Beyla's raw metrics into per-app RED summaries:

```yaml
# Rate: requests per second per app
microfoundry:http_request_rate:5m

# Error rate: 5xx ratio
microfoundry:http_error_rate:5m

# Latency percentiles
microfoundry:http_latency_p50:5m
microfoundry:http_latency_p95:5m
microfoundry:http_latency_p99:5m
```

### Prometheus Client

The `monitoring.PrometheusClient` queries recording rules with parallel execution (errgroup) and a 5-second timeout circuit breaker. Input validation prevents PromQL injection via regex (`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`).

### Log Architecture

- **Collection**: Promtail DaemonSet collects stdout/stderr from all pods
- **Aggregation**: Loki stores and indexes logs
- **Query**: Admin UI's Logs tab queries Loki via HTTP API
- **Live Streaming**: SSE (Server-Sent Events) streams live pod logs via K8s Pod API

---

## Authentication & Authorization

### OIDC Flow

```
User → Login Page → Keycloak Authorization Endpoint
                          │
                    [OIDC Authorization Code Flow with PKCE]
                          │
                    ← Authorization Code
                          │
Server exchanges code → Token Endpoint
                          │
                    ← ID Token + Access Token
                          │
Verify ID Token → Extract Claims (sub, email, name, realm_access.roles)
                          │
Create gorilla/sessions cookie → Set UserSession
                          │
Redirect to / (dashboard)
```

### Session Management

- Cookie-based sessions using `gorilla/sessions`
- Session key: configurable 64-char hex string (auto-generated if empty)
- Stores: user ID, email, name, roles, active org ID

### Organization Model

Organizations are stored as K8s ConfigMaps (`mf-org-*`) with:
- Org ID, name, owner email
- Member list with roles (admin, member, viewer)
- Created/updated timestamps

### Middleware Chain

The auth middleware chain wraps every request in this order (outermost → innermost):

```
Request
  → InstrumentHandler (Prometheus metrics — outermost)
    → InjectUser (session cookie → user context)
      → OPAMiddleware (policy evaluation → allow/deny)
        → Handler (business logic)
```

**InjectUser** runs OUTSIDE OPA so it executes first, populating the user context that OPA needs for policy evaluation.

1. Reads session cookie
2. Extracts `UserSession` from session store
3. Injects user into request context via `auth.UserFromContext(r.Context())`
4. If no valid session, user is `nil` (unauthenticated)

### OPA Authorization Engine

MicroFoundry embeds an [Open Policy Agent](https://www.openpolicyagent.org/) engine for fine-grained authorization using Rego policies.

```
OPAMiddleware(request)
  → Skip if public path (/static/, /login, /auth/*, /health, /metrics)
  → buildAuthzInput: classify route (method+path → action+resource)
  → Resolve org membership role (orgStore.ListMembers)
  → opa.Evaluate(input) → {allow: bool, reason: string}
  → Record audit entry (timestamp, user, action, resource, allowed)
  → If denied: redirect to /login (unauthenticated) or 403 (insufficient role)
```

**Route classification** maps HTTP method + path to action + resource:
- `GET /apps` → action=`read`, resource=`apps`
- `POST /services/create` → action=`write`, resource=`services`
- `DELETE /secrets/mykey` → action=`delete`, resource=`secrets`
- `GET /scim/v2/Users` → action=`read`, resource=`scim`

**Default policy** (`pkg/auth/policies/authz.rego`):
- Platform admins → full access
- Authenticated users → read any resource
- Org admins → write/delete within their org
- Org members → write apps, services, secrets only
- SCIM, settings, clusters → platform-admin only
- Unauthenticated (null user) → public resources only (not SCIM/settings/clusters/users)

**Policy hot-reload**: The `UpdatePolicy` endpoint uses copy-on-write — compiles new modules in a temporary copy first, only swaps on success. Invalid Rego never corrupts the live policy set.

### SCIM v2 Integration

MicroFoundry exposes SCIM v2 (RFC 7643/7644) endpoints that proxy to the Keycloak Admin REST API:

```
SCIM Client → /scim/v2/Users → SCIMHandler
                                    │
                              Convert SCIM ↔ Keycloak
                                    │
                              Keycloak Admin REST API
                              (client_credentials grant)
                                    │
                              /admin/realms/{realm}/users
```

**Keycloak Admin Client** (`pkg/auth/keycloak_admin.go`):
- Uses `client_credentials` grant (not password grant)
- Token caching with automatic refresh before expiry
- User CRUD: List, Get, Create, Update, Delete
- Role management: GetUserRoles, AssignUserRole, RemoveUserRole, GetRealmRoles
- Password reset: ResetPassword (temporary or permanent)

### Audit Subsystem

An in-memory ring buffer records every OPA authorization decision:

```go
type AuditLog struct {
    entries []AuditEntry  // Fixed-size circular buffer
    size    int           // Max entries (default: 1000)
    head    int           // Write position
    count   int           // Current entry count
}
```

Each entry records: timestamp, user email/ID, action, resource, path, method, org ID, allowed/denied, reason, IP address.

At ~100 requests/minute, the default 1000-entry buffer provides ~10 minutes of history. The buffer is queryable by user, resource, and action via the admin UI (Audit tab) and API (`GET /api/audit`).

When auth is disabled, all features work without authentication — OPA middleware is skipped entirely.

---

## Multi-Cluster Architecture

### ClientManager

The `k8s.ClientManager` manages lazy-initialized K8s clients with read-write mutex protection:

```go
type ClientManager struct {
    clients map[string]*Client          // Cached K8s clients
    configs map[string]ClusterConfig    // Cluster configurations
    active  string                      // Currently active cluster
    mu      sync.RWMutex                // Concurrent access protection
}
```

- **Lazy initialization**: Clients are created on first access via `GetClient(id)`
- **Thread safety**: Read-write mutex with double-check locking
- **Cookie-based switching**: Admin UI sends `mf-cluster` cookie for per-request cluster selection
- **Health checks**: `CheckHealth()` verifies connectivity via K8s discovery API

### Cluster Resolution Priority

1. Cookie `mf-cluster` (per-request override from UI)
2. Config `kubernetes.active` (default cluster)

### Provider Detection

Auto-detected from kubeconfig context names:
- `docker-desktop` → `docker-desktop`
- Contains `eks` or `aws` → `eks`
- Contains `gke` → `gke`
- Contains `aks` or `azure` → `aks`
- Other → `native`

---

## Platform Settings Storage

Runtime platform settings (registry, webhooks, SMTP) are stored in Kubernetes rather than config files. This allows configuration via the admin UI without file access.

### Storage Model

```
K8s namespace: microfoundry
  ConfigMap: mf-platform-settings    ← JSON with non-sensitive settings
  Secret:    mf-platform-credentials ← Passwords and tokens
```

### Settings Types

| Setting | ConfigMap Fields | Secret Fields |
|---------|-----------------|---------------|
| **Registry** | URL, project, username, insecure, enabled | `registry-password` |
| **SMTP** | Host, port, username, from_addr, TLS, enabled | `smtp-password` |
| **Webhooks** | Name, URL, events, enabled, created_at | Per-webhook secrets |

### Precedence

Runtime settings from the admin UI (stored in K8s) take precedence over file-based defaults in `mf.yaml`.

---

## Security

### Pre-commit Secret Detection

MicroFoundry uses [pre-commit](https://pre-commit.com/) hooks with [gitleaks](https://github.com/gitleaks/gitleaks) to prevent accidental secret commits. The hook configuration is in `.pre-commit-config.yaml` and runs automatically on every `git commit`.

**Hooks installed:**

| Hook | Source | Purpose |
|------|--------|---------|
| `gitleaks` | gitleaks/gitleaks v8.21.2 | Detect hardcoded secrets, API keys, passwords |
| `trailing-whitespace` | pre-commit-hooks v5.0.0 | Remove trailing whitespace |
| `end-of-file-fixer` | pre-commit-hooks v5.0.0 | Ensure files end with newline |
| `check-yaml` | pre-commit-hooks v5.0.0 | Validate YAML syntax |
| `check-json` | pre-commit-hooks v5.0.0 | Validate JSON syntax |
| `check-merge-conflict` | pre-commit-hooks v5.0.0 | Detect unresolved merge markers |
| `detect-private-key` | pre-commit-hooks v5.0.0 | Detect committed private keys |

**Setup:** `make hooks` (or `pip install pre-commit && pre-commit install`)

### Gitleaks Configuration

The `.gitleaks.toml` file extends gitleaks' default ruleset with project-specific allowlists:

- **Allowed paths**: `configs/mf.yaml` (gitignored), `test/`, `vendor/`, `go.sum`
- **Allowed patterns**: Go struct tags, K8s secret type constants, variable declarations (not values), local-dev Helm passwords

### Sensitive File Protection

The `.gitignore` excludes files likely to contain secrets:

- `configs/mf.yaml` — active config (use `mf.example.yaml` as template)
- `.mf/` — local MicroFoundry state directory
- `.env` / `.env.*` — environment variable files
- `deploy/k8s/overlays/*/generated/` — generated K8s manifests

### Secrets in Kubernetes

For production deployments, MicroFoundry recommends:

- **Sealed Secrets** or **SOPS** for encrypting K8s Secrets at rest in Git
- Platform credentials (registry passwords, SMTP passwords, webhook secrets) are stored in K8s Secrets (`mf-platform-credentials`), never in config files
- Service instance credentials are stored in per-service K8s Secrets (`mf-svc-<name>`)

---

## Documentation System

### Embedded Markdown Docs

Documentation files in `docs/` are embedded into the binary at build time via `docs/embed.go`:

```go
package docs

import "embed"

//go:embed *.md
var Files embed.FS
```

The `DocsHandler` in `pkg/admin/handlers.go` serves these docs through the admin UI at `GET /docs`:

1. **Landing page** (`/docs`) — Displays a card grid of all documentation entries from the `docCatalog` registry
2. **Doc view** (`/docs?tab=<key>`) — Reads the markdown file from `docs.Files`, converts it to HTML using [goldmark](https://github.com/yuin/goldmark), and renders it inside the admin layout

```
docs/embed.go  ──embed.FS──▶  docs.Files
                                  │
admin/handlers.go  ◀──ReadFile───┘
  DocsHandler()
    │
    ├── tab="" → render card grid (docCatalog)
    └── tab=<key> → docs.Files.ReadFile(entry.File)
                      → goldmark.Convert(md, &buf)
                      → render docs.html with HTML content
```

This means all documentation is available in the admin dashboard without external file access, and stays in sync with the binary version.

---

## Project Structure

```
microfoundry/
├── cmd/mf/                    # CLI entry points (20 command modules)
├── pkg/                       # Go packages (core logic)
│   ├── admin/                 # Web dashboard + API handlers
│   │   └── static/            # Embedded HTML/CSS templates
│   │       ├── templates/     # Page templates (48 templates)
│   │       │   ├── partials/  # Shared partials (nav, header, secret_rows)
│   │       │   └── tabs/      # HTMX tab partials (13 tabs)
│   │       └── css/           # Tailwind CSS (CDN)
│   ├── auth/                  # OIDC + sessions + orgs + workspaces
│   ├── build/                 # Docker + CNB build
│   ├── config/                # Viper config loader
│   ├── github/                # GitHub integration (gh CLI wrapper)
│   ├── hosts/                 # /etc/hosts management
│   ├── k8s/                   # Kubernetes client
│   ├── manifest/              # CF manifest parser
│   ├── models/                # Data structures
│   ├── monitoring/            # Observability clients
│   ├── secrets/               # Secret management
│   ├── service/               # Service broker + catalog
│   ├── settings/              # Platform settings store
│   ├── tls/                   # mkcert TLS certificates
│   └── terraform/             # Terraform topology
├── deploy/
│   ├── k8s/                   # K8s manifests (base + overlays)
│   ├── monitoring/            # Observability stack
│   │   ├── install.sh         # One-command setup
│   │   ├── beyla-config.yaml  # eBPF DaemonSet
│   │   ├── prometheus-recording-rules.yaml
│   │   ├── dashboards/        # Grafana dashboards
│   │   └── alerts/            # Alert rules
│   ├── helm/                  # Helm chart (OCI artifact)
│   └── packages/              # Cloud deployment packages
│       ├── aws-eks/           # EKS: Terraform + Helm values + install script
│       ├── aws-ecs-fargate/   # ECS Fargate: Terraform + install script
│       ├── gcp-gke/           # GKE: Terraform + Helm values + install script
│       ├── azure-aks/         # AKS: Terraform + Helm values + install script
│       └── local-k8s/         # Local: Helm values + install script
├── docs/                      # Documentation (embedded at build time)
│   ├── embed.go               # Go embed directive: //go:embed *.md
│   ├── architecture.md        # This file
│   ├── user-manual.md         # CLI and admin UI guide
│   ├── admin-guide.md         # Platform administration guide
│   ├── cloudfoundry-architecture.md   # CF internals reference
│   ├── cloudfoundry-vs-microfoundry.md # CF ↔ MF comparison
│   └── observability-capacity.md      # Monitoring sizing guide
├── test/                      # E2E tests (Playwright, 82 cases, 12 suites)
├── configs/mf.example.yaml    # Example configuration
├── openapi.yaml               # OpenAPI 3.0 specification
├── .gitleaks.toml             # Gitleaks secret detection config
├── .pre-commit-config.yaml    # Pre-commit hooks (gitleaks, hygiene)
├── .goreleaser.yml            # GoReleaser build config
├── Dockerfile                 # Container image build
├── Makefile                   # Build targets (incl. `make hooks`)
└── CLAUDE.md                  # Agent workflow rules
```

### Template Architecture

Templates use Go's `html/template` with an embedded filesystem (`embed.FS`). The clone pattern prevents template name conflicts:

1. **Base template** — parsed once with layout, partials, and tabs
2. **Per-page clones** — each page template is parsed into a clone of base
3. **HTMX partials** — tab content rendered independently for dynamic updates

This ensures each page's `{{define "content"}}` block is isolated.

### Admin UI Navigation

The sidebar is structured into two groups:

**Operations** (all authenticated users):
- Dashboard, Applications, Services, Secrets, Documentation

**Settings** (platform-admin only):
- Users & Orgs (5-tab IAM: Workspaces, Orgs, Users, Policies, Audit), Workspaces, Clusters, Service Catalog, Registry, Webhooks, SMTP, Endpoints, Metrics & Alerts, Platform

---

## Technology Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| No external database | K8s API as data store | Simplicity — no additional infrastructure |
| embed.FS for templates | Go stdlib | Zero runtime file dependencies |
| embed.FS for docs | `docs/embed.go` + goldmark | Docs shipped inside binary, always in sync |
| goldmark | Markdown → HTML rendering | Pure Go, no CGO, used by Hugo/Gitea |
| Beyla eBPF | Zero-code metrics | Language-agnostic, Netflix-inspired |
| Viper config | File + env + defaults | Standard Go config pattern |
| gorilla/sessions | Cookie sessions | Lightweight, no server-side state |
| client-go | Direct K8s API | Full K8s API access, no abstractions |
| HTMX | Partial page updates | No JavaScript build step required |
| Tailwind CSS (CDN) | Utility-first styling | No CSS build step required |
| No ORM | Direct K8s API calls | K8s resources are the data model |
| pre-commit + gitleaks | Git hook secret detection | Prevent accidental credential commits |
| OpenAPI 3.0 spec | `openapi.yaml` at repo root | Machine-readable API contract |
