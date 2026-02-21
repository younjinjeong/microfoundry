# MicroFoundry User Manual

A complete guide to deploying and managing applications on MicroFoundry — a micro CloudFoundry for Kubernetes.

---

## Table of Contents

1. [Getting Started](#getting-started)
2. [Configuration](#configuration)
3. [Application Lifecycle](#application-lifecycle)
4. [Backing Services](#backing-services)
5. [Secret Management](#secret-management)
6. [Monitoring & Observability](#monitoring--observability)
7. [Authentication & IAM](#authentication--iam)
8. [Multi-Cluster Management](#multi-cluster-management)
9. [Container Registry](#container-registry)
10. [TLS Setup](#tls-setup)
11. [Admin Dashboard](#admin-dashboard)
12. [Security Tools](#security-tools)
13. [CLI Reference](#cli-reference)

---

## Getting Started

### Prerequisites

- **Go 1.25+** — for building the CLI
- **Docker Desktop** with Kubernetes enabled (for local development)
- **kubectl** — Kubernetes CLI
- **Helm 3** — for monitoring stack installation
- **Nginx Ingress Controller** — for routing (or Kong/CSP-native gateway)

### Install

```bash
# Clone the repository
git clone https://github.com/younjinjeong/microfoundry.git
cd microfoundry

# Build the CLI
make build              # Output: bin/mf

# Or install to GOPATH/bin
make install
```

### Verify Installation

```bash
mf version
# MicroFoundry dev
```

### Quick Start

```bash
# 1. Deploy your first app
cd your-app-directory/
mf push hello-world

# 2. Check it's running
mf apps

# 3. View app details
mf app hello-world

# 4. Stream logs
mf logs hello-world

# 5. Open admin dashboard
mf admin
# → http://localhost:8080  (or https://admin.cf-local.dev:8443 after mf setup tls)
```

---

## Configuration

MicroFoundry uses YAML configuration read by [Viper](https://github.com/spf13/viper). Config files are searched in order:

1. `./configs/mf.yaml` — project-local config
2. `~/.mf/mf.yaml` — user home config

Environment variables override file values with the prefix `MF_` (e.g., `MF_GITHUB_OWNER`).

### Example Config

Copy the example and edit:

```bash
cp configs/mf.example.yaml configs/mf.yaml
# or for user-wide:
cp configs/mf.example.yaml ~/.mf/mf.yaml
```

### Configuration Reference

```yaml
# GitHub integration (for source linking)
github:
  owner: "younjinjeong"
  repo: "microfoundry"

# Kubernetes clusters
kubernetes:
  active: "docker-desktop"       # Which cluster to use
  clusters:
    docker-desktop:
      name: "docker-desktop"
      context: "docker-desktop"  # kubectl context name
      namespace: "microfoundry"  # K8s namespace for apps
      domain: "cf-local.dev"     # App domain (subdomain routing)
      provider: "docker-desktop" # Provider hint
      enabled: true
      tls: false                 # Set to true after running `mf setup tls`
      ingress_class: ""          # API gateway: "nginx" (default), "kong", or "traefik"
    # Add more clusters:
    # eks-prod:
    #   context: "arn:aws:eks:us-east-1:..."
    #   namespace: "microfoundry"
    #   domain: "apps.example.com"
    #   provider: "eks"
    #   region: "us-east-1"
    #   ingress_class: "kong"

# Admin server
admin:
  port: 8080       # HTTP port (used when TLS is disabled, or for HTTP→HTTPS redirect)
  host: "0.0.0.0"  # Bind address for the admin server
  # domain: "admin.cf-local.dev"  # Admin UI domain (set automatically by `mf setup tls`)
  # tls: true                     # Auto-enable TLS using ~/.mf/cert.pem
  # tls_port: 8443                # HTTPS port (default: 8443; use 443 for clean URLs)

# Admin UI feature toggles
ui:
  tooltips: true  # Show contextual help tooltips on hover (disable with false)

# Monitoring stack endpoints
monitoring:
  grafana_url: "http://localhost:3000"
  loki_url: "http://loki.monitoring.svc.cluster.local:3100"
  alertmanager_url: "http://kube-prometheus-kube-prome-alertmanager.monitoring.svc.cluster.local:9093"
  prometheus_url: "http://kube-prometheus-kube-prome-prometheus.monitoring.svc.cluster.local:9090"
  beyla_enabled: true

# Container registry (file-based defaults; admin UI settings stored in K8s take precedence)
# registry:
#   url: "harbor.local:30003"
#   project: "microfoundry"
#   username: "admin"
#   insecure: false

# SMTP server (file-based defaults; admin UI settings stored in K8s take precedence)
# smtp:
#   host: "smtp.gmail.com"
#   port: 587
#   username: "user@example.com"
#   from_addr: "noreply@microfoundry.local"
#   tls: true

# Authentication (optional — Keycloak OIDC + OPA + SCIM)
# auth:
#   enabled: true
#   issuer_url: "http://localhost:8180/realms/microfoundry"
#   client_id: "mf-admin"
#   client_secret: "your-secret"
#   redirect_url: "https://admin.cf-local.dev:8443/auth/callback"
#   session_key: ""  # 64-char hex string; auto-generated if empty
#   admin_base_url: "http://localhost:8180"   # Keycloak Admin API
#   admin_client_id: "admin-cli"              # Client for Admin REST API
#   admin_client_secret: "your-admin-secret"  # Client credentials grant
#   realm: "microfoundry"                     # Keycloak realm name
```

### Admin UI Settings Pages

In addition to the YAML config file, several settings can be managed at runtime through the admin dashboard. These settings are stored in Kubernetes ConfigMaps and Secrets (not in files) and take precedence over file-based defaults:

| Settings Page | Path | Description |
|---------------|------|-------------|
| **Registry** | `/settings/registry` | Container registry URL, project, credentials, TLS skip, push toggle |
| **Webhooks** | `/settings/webhooks` | HTTP webhook endpoints for platform event notifications |
| **SMTP** | `/settings/smtp` | Email server configuration for alerts and notifications |
| **Endpoints** | `/settings/endpoints` | Override URLs for Prometheus, Grafana, Loki, AlertManager |
| **Platform** | `/settings/platform` | Read-only view of DNS, TLS certificates, ingresses, and environment info |

---

## Application Lifecycle

### Deploying an Application

MicroFoundry supports deploying from any directory containing a **Dockerfile** or source compatible with **Cloud Native Buildpacks** (Paketo/pack CLI).

```bash
# Deploy from current directory
mf push my-app

# Deploy with custom memory and instances
mf push my-app -m 512M -i 3

# Deploy from a specific path
mf push my-app -p /path/to/source
```

**Build strategies** (auto-detected in order):
1. **Dockerfile** — if `Dockerfile` exists in the source directory
2. **Cloud Native Buildpacks** — if `pack` CLI is available

**What happens during `mf push`:**

```
Phase 1:   Build image locally (Docker or CNB)
Phase 1.5: Push to registry (if registry configured)
Phase 2:   Deploy to Kubernetes (Deployment + Service)
Phase 3:   Create Ingress route (<app-name>.<domain>)
Phase 4:   Update /etc/hosts (local dev only)
Phase 5:   Wait for rollout (up to 120 seconds)
```

### Using manifest.yml

You can use a CloudFoundry-compatible `manifest.yml`:

```yaml
applications:
  - name: hello-world
    memory: 256M
    instances: 2
    buildpacks:
      - paketo-buildpacks/go
    routes:
      - route: hello-world.cf-local.dev
    env:
      PORT: "8080"
```

### Listing Applications

```bash
# List all deployed applications
mf apps
```

Output:
```
name            state     instances   memory
hello-world     STARTED   3/3         256M
api-gateway     STARTED   2/2         512M
```

### Viewing App Details

```bash
mf app hello-world
```

Shows: name, state, routes, instances (with pod names, restart counts, node assignments), image reference, memory, disk, CPU limits, and more.

### Streaming Logs

```bash
# Stream live logs (like cf logs)
mf logs hello-world

# Fetch recent logs (last 100 lines)
mf logs hello-world --recent
```

### Scaling

```bash
# Scale to 5 instances
mf scale hello-world -i 5

# Change memory limit (triggers restart)
mf scale hello-world -m 512M

# Change disk limit
mf scale hello-world -k 1G

# Combine and force (no restart prompt)
mf scale hello-world -m 512M -k 1G -f
```

### Deleting an App

```bash
mf delete hello-world
```

This removes the Deployment, Service, Ingress, and associated /etc/hosts entries.

---

## Backing Services

MicroFoundry provides a built-in service catalog with 56 service types across 4 providers (10 local K8s + 21 AWS + 12 GCP + 13 Azure).

### Service Catalog

**Local K8s Services** (provisioned as StatefulSets + PVCs):

| Category | Services |
|----------|----------|
| **Database** | MariaDB, PostgreSQL, ClickHouse |
| **Cache** | Redis, Memcached |
| **Messaging** | RabbitMQ, ActiveMQ Artemis |
| **Storage** | MinIO (S3-compatible) |
| **Gateway** | Kong, Nginx |

**AWS Managed Services** (provisioned via Terraform topologies):

| Category | Services |
|----------|----------|
| **Database** | RDS PostgreSQL, RDS MySQL, OpenSearch |
| **Cache** | ElastiCache Redis, ElastiCache Memcached |
| **Messaging** | MSK (Kafka) |
| **Storage** | S3 |

Each service offers 3 plans with resource allocations appropriate to its provider:

- **Small (Dev)** — minimal resources for development
- **Medium (Staging)** — moderate resources for integration testing
- **Large (Production)** — production-grade resources with HA where applicable

### Browse the Catalog

```bash
mf catalog
```

### Provision a Service

```bash
# Create a PostgreSQL instance with the small plan
mf create-service postgresql small my-db

# Create a Redis cache
mf create-service redis small my-cache

# Create a message queue
mf create-service rabbitmq medium my-queue
```

### List Provisioned Services

```bash
mf services
```

### View Service Details

```bash
mf service my-db
```

### Bind a Service to an App

Binding injects credentials as `VCAP_SERVICES` environment variable — the same mechanism as CloudFoundry:

```bash
mf bind-service hello-world my-db
```

This creates a K8s Secret (`mf-svc-my-db`) and adds it to the app's deployment as an environment source. The app receives a `VCAP_SERVICES` JSON blob with connection details:

```json
{
  "postgresql": [{
    "name": "my-db",
    "credentials": {
      "host": "mf-svc-my-db.microfoundry.svc.cluster.local",
      "port": "5432",
      "username": "postgres",
      "password": "auto-generated",
      "database": "mydb"
    }
  }]
}
```

### Unbind and Delete

```bash
# Unbind service from app
mf unbind-service hello-world my-db

# Delete the service instance
mf delete-service my-db
```

---

## Secret Management

MicroFoundry manages two types of Kubernetes secrets:

- **Service secrets** (`mf-svc-*`) — auto-created when provisioning backing services
- **User secrets** (`mf-secret-*`) — developer-managed key-value pairs

### List Secrets

```bash
mf secrets
```

### Create a User Secret

```bash
mf create-secret api-keys API_KEY=abc123 API_SECRET=xyz789
```

### View Secret Details

```bash
# Show secret metadata (values masked)
mf secret api-keys

# Reveal actual values
mf secret api-keys --reveal
```

### Delete a Secret

```bash
mf delete-secret api-keys
```

---

## Monitoring & Observability

MicroFoundry provides Netflix Atlas-inspired auto-instrumentation using **Grafana Beyla** (eBPF). Applications get full RED metrics without any code changes.

### Install the Monitoring Stack

```bash
make monitoring-install
# Or directly:
bash deploy/monitoring/install.sh
```

This installs:
1. **kube-prometheus-stack** — Prometheus + Grafana + AlertManager
2. **Grafana Beyla** — eBPF DaemonSet for zero-code HTTP metrics
3. **Loki + Promtail** — Log aggregation
4. **Grafana dashboards** — Pre-built dashboards
5. **Prometheus recording rules** — Pre-computed RED metrics
6. **Alert rules** — App health alerts

### RED Metrics

Every deployed application automatically gets these metrics via Beyla eBPF:

| Metric | Recording Rule | Description |
|--------|---------------|-------------|
| **Rate** | `microfoundry:http_request_rate:5m` | Requests per second |
| **Errors** | `microfoundry:http_error_rate:5m` | 5xx error ratio |
| **Duration p50** | `microfoundry:http_latency_p50:5m` | Median latency |
| **Duration p95** | `microfoundry:http_latency_p95:5m` | 95th percentile latency |
| **Duration p99** | `microfoundry:http_latency_p99:5m` | 99th percentile latency |

### Alert Rules

| Alert | Condition | Severity |
|-------|-----------|----------|
| `MFAppHighErrorRate` | Error rate > 5% for 5 minutes | warning |
| `MFAppHighLatency` | p95 latency > 1s for 5 minutes | warning |
| `MFAppNoTraffic` | Zero requests for 15 minutes | info |
| `MFBeylaDown` | Beyla DaemonSet not running | critical |

### Accessing Dashboards

```bash
# Grafana (via ingress)
open http://grafana.cf-local.dev
# Default: admin / microfoundry

# Prometheus (port-forward)
kubectl port-forward -n monitoring svc/kube-prometheus-kube-prome-prometheus 9090:9090

# AlertManager (port-forward)
kubectl port-forward -n monitoring svc/kube-prometheus-kube-prome-alertmanager 9093:9093
```

---

## Authentication & IAM

MicroFoundry provides a full IAM stack: OIDC authentication via **Keycloak**, fine-grained authorization via **OPA (Open Policy Agent)**, standard identity provisioning via **SCIM v2**, and an authorization **audit log**.

### Deploy Keycloak

```bash
# Deploy Keycloak to the active cluster
mf setup keycloak

# Wait for it, then port-forward:
kubectl port-forward -n microfoundry svc/keycloak 8180:8180

# Configure the realm, client, and roles
mf setup keycloak-realm --url http://localhost:8180

# (Optional) Add Google social login
mf setup keycloak-idp --provider google \
  --client-id <GOOGLE_CLIENT_ID> \
  --client-secret <GOOGLE_CLIENT_SECRET>
```

### Enable Authentication

Add to your `mf.yaml`:

```yaml
auth:
  enabled: true
  issuer_url: "http://localhost:8180/realms/microfoundry"
  client_id: "mf-admin"
  client_secret: "<from mf setup keycloak-realm>"
  redirect_url: "http://localhost:8080/auth/callback"
  # Keycloak Admin API (for user management & SCIM)
  admin_base_url: "http://localhost:8180"
  admin_client_id: "admin-cli"
  admin_client_secret: "<from Keycloak admin-cli>"
  realm: "microfoundry"
```

### Roles (5-tier RBAC)

Keycloak is configured with a 5-tier role hierarchy. The `mf setup keycloak-realm` command creates these realm roles automatically:

| Tier | Role | Scope |
|------|------|-------|
| 1 | **platform-admin** | Full platform access: SCIM, settings, clusters, audit, all workspaces and orgs |
| 2 | **workspace-admin** | Manage organizations and members within an assigned workspace |
| 3 | **org-admin** | Organization administrator — write and delete within their organization |
| 4 | **org-member** | Organization member — write apps, services, and secrets |
| 5 | **viewer** | Read-only access to all resources |

The admin UI sidebar adapts to the authenticated user's role:

- **Platform-admins** see the full **Settings** section (Users & Orgs, Clusters, Service Catalog, Registry, Webhooks, SMTP, Endpoints, Metrics & Alerts, Platform).
- **Workspace-admins** and **org-admins** see an **IAM** link in the Operations section, scoped to their workspace or organization.
- **Members** and **viewers** see only the Operations section (Dashboard, Applications, Services, Secrets).

### Workspaces

Workspaces provide a hierarchy level above organizations for multi-tenant platform management:

- **Platform → Workspace → Organization → User** hierarchy
- Workspace-admins can manage organizations and members within their workspace
- Create workspaces from the admin dashboard (Users & Orgs → Workspaces tab) or CLI (`mf workspaces create`)
- Switch active workspace to scope operations

### Organizations

When auth is enabled, users can create and manage organizations:

- Each user gets a default personal organization
- Invite members by email
- Assign roles: admin, member, viewer
- Switch active organization

### OPA Authorization

All API and UI routes are evaluated by the embedded **Open Policy Agent** using Rego policies. The middleware chain runs in order:

```
InstrumentHandler → InjectUser → OPAMiddleware → Handler
```

**Default authorization rules** (embedded in `pkg/auth/policies/authz.rego`):

| Rule | Condition |
|------|-----------|
| Public resources | Auth disabled (user is null), excludes SCIM/settings/clusters/users |
| Platform admin | Full access to all resources |
| Authenticated read | Any authenticated user can read any resource |
| Org admin write | Org admins can write within their organization |
| Org member write | Org-members can write apps, services, and secrets only |
| SCIM access | Requires platform-admin role |
| Delete | Requires org admin role (apps, services, secrets) |

**Custom policies**: Platform admins can add or replace Rego policies at runtime via the **Policies tab** in Users & IAM. Changes are compiled and swapped atomically.

### SCIM v2 Provisioning

MicroFoundry exposes 9 SCIM v2 endpoints (RFC 7643/7644) for standard identity provisioning. All SCIM endpoints require `platform-admin` role.

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/scim/v2/Users` | GET | List users (pagination, filter) |
| `/scim/v2/Users` | POST | Create user |
| `/scim/v2/Users/{id}` | GET | Get user by ID |
| `/scim/v2/Users/{id}` | PUT | Replace user |
| `/scim/v2/Users/{id}` | PATCH | Partial update (PatchOp) |
| `/scim/v2/Users/{id}` | DELETE | Delete user |
| `/scim/v2/ServiceProviderConfig` | GET | Discovery |
| `/scim/v2/ResourceTypes` | GET | Resource types |
| `/scim/v2/Schemas` | GET | Schema discovery |

SCIM requests use `Content-Type: application/scim+json`. Path IDs are validated as UUIDs. The `count` parameter is capped at 100.

### Audit Log

Every authorization decision is recorded in an in-memory ring buffer (1000 entries). Each entry captures:

- Timestamp, user, action, resource, HTTP path/method
- Organization ID, allow/deny decision, denial reason, client IP

View the audit log from the **Audit tab** in Users & IAM, or query via `GET /api/audit`.

---

## Multi-Cluster Management

MicroFoundry supports deploying to multiple Kubernetes clusters.

### Add a Cluster via Config

```yaml
kubernetes:
  active: "docker-desktop"
  clusters:
    docker-desktop:
      context: "docker-desktop"
      namespace: "microfoundry"
      domain: "cf-local.dev"
      provider: "docker-desktop"
      enabled: true
    eks-prod:
      context: "arn:aws:eks:us-west-2:123456789:cluster/my-cluster"
      namespace: "microfoundry"
      domain: "apps.example.com"
      provider: "eks"
      region: "us-west-2"
      enabled: true
```

### Add a Cluster via Admin UI

Navigate to **Settings > Clusters** in the admin dashboard to add, remove, and switch clusters.

### Provider Auto-Detection

The provider is auto-detected from the kubeconfig context name:
- `docker-desktop` → Docker Desktop
- Contains `eks` or `aws` → EKS
- Contains `gke` → GKE
- Contains `aks` or `azure` → AKS
- Other → Native

---

## Container Registry

By default, MicroFoundry builds images locally. You can configure a container registry (e.g., Harbor, Docker Hub) for remote image storage.

### Configure via Admin UI

Navigate to **Settings > Registry** and fill in:
- **Registry URL** — e.g., `harbor.local:30003`
- **Project** — e.g., `microfoundry`
- **Username/Password** — registry credentials
- **Skip TLS Verification** — for self-signed certs
- **Enable Registry Push** — toggle on

### How It Works

When a registry is configured and `mf push` runs:

1. Image is built locally with the registry prefix (e.g., `harbor.local:30003/microfoundry/my-app`)
2. CLI runs `docker login` to authenticate
3. Image is pushed to the registry
4. An `imagePullSecret` is created in the namespace
5. Deployment uses `imagePullSecrets` and `imagePullPolicy: Always`

Without a registry configured, images remain local (suitable for Docker Desktop development).

---

## TLS Setup

MicroFoundry can serve the admin dashboard and all application ingresses over HTTPS using locally-trusted certificates generated by [mkcert](https://github.com/FiloSottile/mkcert).

### Generate Certificates

```bash
mf setup tls
```

This command performs 8 steps:

1. Checks that `mkcert` is installed
2. Installs the mkcert root CA into the system trust store
3. Generates a wildcard certificate for `*.<domain>` (e.g., `*.cf-local.dev`)
4. Creates a TLS secret in the application namespace
5. Creates a TLS secret in the `monitoring` namespace
6. Creates a Keycloak ingress route with TLS
7. Updates the hosts file with `admin.<domain>`, `keycloak.<domain>`, and `grafana.<domain>`
8. Saves certificates to `~/.mf/cert.pem` and `~/.mf/key.pem` for the admin server

### Enable HTTPS on the Admin Server

After running `mf setup tls`, add the following to your `mf.yaml`:

```yaml
admin:
  domain: "admin.cf-local.dev"
  tls: true
  tls_port: 8443
```

Then start the admin server:

```bash
mf admin
# → https://admin.cf-local.dev:8443
```

The admin server auto-detects TLS certificates from `~/.mf/` when `admin.tls` is enabled. When TLS is active, an HTTP-to-HTTPS redirect server also starts on the configured `admin.port` (default 8080).

### Service URLs After TLS Setup

| Service | URL |
|---------|-----|
| Admin Dashboard | `https://admin.cf-local.dev:8443` |
| Keycloak | `https://keycloak.cf-local.dev` |
| Grafana | `https://grafana.cf-local.dev` |
| Applications | `https://<app-name>.cf-local.dev` |

---

## Admin Dashboard

The admin dashboard is a web-based UI for managing the MicroFoundry platform. Start it with:

```bash
mf admin
# HTTP:  http://localhost:8080
# HTTPS: https://admin.cf-local.dev:8443  (after mf setup tls)
```

### Navigation Structure

The sidebar is organized into three sections, with visibility controlled by the authenticated user's role:

**Operations** (visible to all authenticated users):

- **Dashboard** (`/`) — Platform overview: app count, cluster info, quick links
- **Applications** (`/apps`) — View and manage all deployed applications
- **Services** (`/services`) — Provisioned backing services: databases, caches, queues
- **Secrets** (`/secrets`) — Manage application secrets and environment variables

**Settings** (platform-admin only):

- **Users & Orgs** (`/users`) — Workspaces, organizations, Keycloak users, roles, OPA policies, audit log
- **Clusters** (`/clusters`) — Register and manage Kubernetes clusters
- **Service Catalog** (`/catalog`) — Browse service plans, configure provisioning topologies, manage visibility
- **Registry** (`/settings/registry`) — Container registry configuration
- **Webhooks** (`/settings/webhooks`) — HTTP webhook endpoints for platform events
- **SMTP** (`/settings/smtp`) — Email server for alerts and notifications
- **Endpoints** (`/settings/endpoints`) — Override URLs for Prometheus, Grafana, Loki, AlertManager
- **Metrics & Alerts** (`/monitoring`) — Application metrics, cluster resources, alert management
- **Platform** (`/settings/platform`) — DNS, TLS certificates, ingresses, and environment info

**Resources** (visible to all users):

- **Documentation** (`/docs`) — Renders project documentation (User Manual, Admin Guide, Architecture) as browsable HTML pages directly within the admin dashboard

### Platform Settings Page

The **Platform** page (`/settings/platform`) is a read-only diagnostics view that shows:

- **Environment** — Provider type (Docker Desktop, EKS, GKE, AKS), Kubernetes version, node count
- **DNS** — Cluster domain, admin domain, authentication domain
- **TLS** — Certificate details (subject, DNS names, issuer, validity), TLS secret status in app and monitoring namespaces
- **Ingresses** — All ingress routes across the application and monitoring namespaces
- **Hosts File** — Current `/etc/hosts` entries managed by MicroFoundry
- **Platform Services** — Discovered endpoints for Prometheus, Grafana, Loki, AlertManager with override and ingress status

### Documentation Page

The **Docs** page (`/docs`) serves project Markdown files as rendered HTML. It includes a card-grid landing page and individual document views with reading time and word count. Available documents include the User Manual, Admin Guide, and Architecture reference.

---

## Security Tools

### Pre-Commit Hooks

MicroFoundry includes a `.pre-commit-config.yaml` with security and hygiene hooks that run automatically before each commit.

#### Setup

```bash
# Using make
make hooks

# Or manually
pip install pre-commit
pre-commit install
```

#### Run Manually

```bash
pre-commit run --all-files
```

#### Included Hooks

| Hook | Source | Description |
|------|--------|-------------|
| **gitleaks** | `gitleaks/gitleaks` v8.21.2 | Detects hardcoded secrets (API keys, passwords, tokens) in staged changes |
| **trailing-whitespace** | `pre-commit-hooks` v5.0.0 | Removes trailing whitespace |
| **end-of-file-fixer** | `pre-commit-hooks` v5.0.0 | Ensures files end with a newline |
| **check-yaml** | `pre-commit-hooks` v5.0.0 | Validates YAML syntax (allows multi-document files) |
| **check-json** | `pre-commit-hooks` v5.0.0 | Validates JSON syntax |
| **check-merge-conflict** | `pre-commit-hooks` v5.0.0 | Prevents committing merge conflict markers |
| **detect-private-key** | `pre-commit-hooks` v5.0.0 | Flags private key files that should not be committed |

The **gitleaks** hook is the primary security tool — it scans all staged changes for patterns matching API keys, passwords, connection strings, and other secrets before they reach the repository.

---

## CLI Reference

### Application Commands

| Command | Description | Flags |
|---------|-------------|-------|
| `mf push [app]` | Build and deploy an application | `-m` memory, `-i` instances, `-p` path |
| `mf apps` | List all deployed applications | |
| `mf app [name]` | Show application details | |
| `mf logs [name]` | Stream application logs | `--recent` fetch history |
| `mf scale [name]` | Scale instances, memory, and disk | `-i` instances, `-m` memory, `-k` disk, `-f` force |
| `mf delete [name]` | Delete an application | |

### Service Commands

| Command | Description |
|---------|-------------|
| `mf catalog` | Browse the service catalog |
| `mf create-service <type> <plan> <name>` | Provision a service instance |
| `mf services` | List provisioned services |
| `mf service [name]` | Show service details |
| `mf bind-service <app> <svc>` | Bind a service to an app |
| `mf unbind-service <app> <svc>` | Unbind a service |
| `mf delete-service [name]` | Delete a service instance |

### Secret Commands

| Command | Description | Flags |
|---------|-------------|-------|
| `mf secrets` | List all secrets | |
| `mf secret <name>` | Show secret details | `--reveal` show actual values |
| `mf create-secret <name> k=v...` | Create a user secret | |
| `mf delete-secret <name>` | Delete a secret | |

### Authentication Commands

| Command | Description | Flags |
|---------|-------------|-------|
| `mf login` | Authenticate with Keycloak credentials | `-u` username, `-p` password |
| `mf logout` | Clear stored authentication token | |
| `mf whoami` | Display current authenticated user and roles | |

### User Management Commands

The `mf users` command group manages Keycloak users via the Admin API. Requires `auth.admin_base_url` in config.

| Command | Description | Flags |
|---------|-------------|-------|
| `mf users list` | List all Keycloak users | `--search` filter by username/email |
| `mf users create` | Create a new user | `--username`, `--email`, `--first-name`, `--last-name`, `--password` |
| `mf users delete <user-id>` | Delete a user | |
| `mf users toggle <user-id>` | Enable or disable a user | |
| `mf users reset-password <user-id>` | Reset a user's password | `--password`, `--temporary` |
| `mf users roles <user-id>` | List roles assigned to a user | |
| `mf users assign-role <user-id>` | Assign a realm role to a user | `--role-name`, `--role-id` (optional) |
| `mf users remove-role <user-id>` | Remove a realm role from a user | `--role-name`, `--role-id` (optional) |
| `mf users realm-roles` | List all available realm roles | |

### Organization Commands

The `mf orgs` command group manages organizations stored in Kubernetes.

| Command | Description | Flags |
|---------|-------------|-------|
| `mf orgs list` | List all organizations | |
| `mf orgs create` | Create a new organization | `--name`, `--owner`, `--workspace` (optional) |
| `mf orgs delete <org-id>` | Delete an organization | |
| `mf orgs members <org-id>` | List members of an organization | |
| `mf orgs add-member <org-id>` | Add a member to an organization | `--email`, `--name`, `--role` (admin/member/viewer) |
| `mf orgs remove-member <org-id>` | Remove a member | `--email` |
| `mf orgs set-role <org-id>` | Change a member's role | `--email`, `--role` |

### Workspace Commands

The `mf workspaces` command group manages workspaces — the hierarchy level above organizations.

| Command | Description | Flags |
|---------|-------------|-------|
| `mf workspaces list` | List all workspaces | |
| `mf workspaces create` | Create a new workspace | `--name`, `--owner` |
| `mf workspaces delete <ws-id>` | Delete a workspace | |
| `mf workspaces members <ws-id>` | List members of a workspace | |
| `mf workspaces add-member <ws-id>` | Add a member to a workspace | `--email`, `--name`, `--role` (admin/member/viewer) |
| `mf workspaces remove-member <ws-id>` | Remove a member | `--email` |
| `mf workspaces set-role <ws-id>` | Change a member's role | `--email`, `--role` |
| `mf workspaces orgs <ws-id>` | List organizations in a workspace | |

### Platform Commands

| Command | Description | Flags |
|---------|-------------|-------|
| `mf admin` | Start the admin web dashboard | `-p` port (default 8080), `--host` bind address, `--tls-cert`, `--tls-key` |
| `mf setup keycloak` | Deploy Keycloak to the active cluster | `--admin-user`, `--admin-pass`, `--client-secret`, `--port` |
| `mf setup keycloak-realm` | Configure Keycloak realm and client | `--url`, `--admin-user`, `--admin-pass`, `--client-secret`, `--redirect-uri` |
| `mf setup keycloak-idp` | Add an identity provider (google, github, amazon) | `--provider`, `--client-id`, `--client-secret`, `--url` |
| `mf setup tls` | Generate locally-trusted TLS certificates with mkcert | |
| `mf version` | Print version | |

---

## Local Development Tips

### Host Resolution

When deploying locally, `mf push` automatically adds entries to `/etc/hosts`:
```
127.0.0.1 hello-world.cf-local.dev
```

On Windows, the hosts file is at `C:\Windows\System32\drivers\etc\hosts`. You may need to run with admin privileges.

### Ingress Controller

Install an Nginx Ingress Controller for local routing:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/cloud/deploy.yaml
```

### Build Targets

```bash
make build              # Build CLI to bin/mf
make test               # Run all tests
make lint               # Run golangci-lint
make fmt                # Format Go code
make tidy               # go mod tidy
make clean              # Remove build artifacts
make docker-build       # Build Docker image
make install            # Install to GOPATH/bin
make hooks              # Install pre-commit hooks (includes gitleaks)
make monitoring-install # Deploy monitoring stack
```

---

## CloudFoundry Compatibility

MicroFoundry maps CloudFoundry concepts to Kubernetes primitives:

| CF Concept | MicroFoundry Equivalent |
|------------|------------------------|
| `cf push` | `mf push` — builds and deploys to K8s |
| Diego Cell | Kubernetes Pod |
| Gorouter | Ingress Controller (Nginx/Kong) |
| Cloud Controller | MicroFoundry API Server |
| Buildpacks | Cloud Native Buildpacks (Paketo) |
| Service Broker | Built-in K8s-native provisioner |
| Loggregator | Promtail + Loki |
| Doppler/Metrics | Prometheus + Grafana + Beyla eBPF |
| NATS (Alerts) | AlertManager |
| UAA | Keycloak OIDC + OPA + SCIM v2 |
| Org/Space | Workspace → Organization hierarchy |
| VCAP_SERVICES | Same format — injected via K8s Secrets |
| manifest.yml | Supported — same format |
