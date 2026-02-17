# MicroFoundry Admin Dashboard Guide

Complete guide to the MicroFoundry web-based admin dashboard.

---

## Table of Contents

1. [Starting the Dashboard](#starting-the-dashboard)
2. [Navigation](#navigation)
3. [Dashboard Overview](#dashboard-overview)
4. [Applications](#applications)
5. [Services](#services)
6. [Secrets](#secrets)
7. [Metrics & Alerts](#metrics--alerts)
8. [Cluster Management](#cluster-management)
9. [Service Catalog](#service-catalog)
10. [Platform Settings](#platform-settings)
11. [Users & IAM](#users--iam)
12. [SCIM v2 API](#scim-v2-api)
13. [API Reference](#api-reference)

---

## Starting the Dashboard

```bash
# Start on default port 8080
mf admin

# Custom port and host
mf admin -p 9090 --host 0.0.0.0
```

Output:
```
Authentication enabled (Keycloak: http://localhost:8180/realms/microfoundry)
MicroFoundry Admin starting at http://localhost:8080
Active cluster: docker-desktop
```

The dashboard serves both HTML pages (for browser UI) and JSON API endpoints (for programmatic access).

---

## Navigation

The sidebar organizes all pages into two groups:

### Operations (Daily Use)

| Page | URL | Description |
|------|-----|-------------|
| **Dashboard** | `/` | Platform overview with app count, cluster status |
| **Applications** | `/apps` | List, deploy, scale, delete applications |
| **Services** | `/services` | Manage provisioned service instances |
| **Secrets** | `/secrets` | View and manage Kubernetes secrets |
| **Metrics & Alerts** | `/monitoring` | Prometheus alerts, Grafana dashboards |

### Settings (Environment Setup)

| Page | URL | Description |
|------|-----|-------------|
| **Clusters** | `/clusters` | Multi-cluster management |
| **Service Catalog** | `/catalog` | Browse and configure service plans |
| **Registry** | `/settings/registry` | Container registry configuration |
| **Webhooks** | `/settings/webhooks` | Event webhook management |
| **SMTP** | `/settings/smtp` | Email notification configuration |
| **Users & Orgs** | `/users` | Organization and member management |
| **Platform** | `/config` | Platform configuration view |

---

## Dashboard Overview

**URL**: `/`

The dashboard provides a quick overview of the platform state:
- Total deployed applications
- Active cluster information
- Recent deployment activity
- System health indicators

---

## Applications

### App List

**URL**: `/apps`

Displays a table of all deployed applications with columns:
- **Name** — application name (clickable for details)
- **Org** — Kubernetes namespace
- **Owner** — who deployed (from annotation)
- **State** — STARTED / STOPPED / DEPLOYING
- **Instances** — running/total count
- **Memory** — allocated memory
- **Type** — build type (docker/buildpack/cnb)
- **Routes** — ingress URL
- **Created** — relative timestamp
- **Actions** — scale and delete buttons

**Filtering**: Use the state dropdown to filter by All / Started / Stopped.

Responsive design hides Org, Owner, and Type columns on smaller screens.

### App Detail

**URL**: `/apps/{name}`

The detail page has 6 tabs loaded via HTMX partial updates:

#### Overview Tab
**URL**: `/apps/{name}?tab=overview`

Information cards showing:
- State, owner, organization, created date
- Resources: instance count, memory, CPU, disk
- Routes with full URLs
- Build info: lifecycle type, image reference

#### Instances Tab
**URL**: `/apps/{name}?tab=instances`

Live table of running pods:
- Instance number, state, uptime
- Restart count, node name, pod name
- Scale form with instance count input
- Auto-refreshes every 5 seconds

#### Config Tab
**URL**: `/apps/{name}?tab=config`

Environment variables table:
- Key/value display with sensitive value masking
- Toggle show/hide for secrets (PASSWORD, SECRET, KEY, TOKEN, CREDENTIALS)
- Resource limits (CPU, memory)
- Labels and annotations

#### Services Tab
**URL**: `/apps/{name}?tab=services`

Bound services and associated secrets:
- Service name, type, status
- Related secrets
- Empty state guidance for binding services

#### Routes Tab
**URL**: `/apps/{name}?tab=routes`

Full route table:
- Host, domain, path, protocol
- Clickable URL for each route
- Empty state when no routes configured

#### Logs Tab
**URL**: `/apps/{name}?tab=logs`

Log viewer with two modes:
- **Live streaming**: SSE-based real-time log feed (start/stop toggle)
- **Historical**: Query Loki for past logs

#### Performance Tab
**URL**: `/apps/{name}?tab=performance`

RED metrics dashboard:
- Request rate, error rate stat cards
- Latency percentiles (p50, p95, p99)
- Grafana panel embeds
- Correlated log entries

### HTMX Tab Loading

Tab content is loaded dynamically via HTMX:
- URL: `/apps/{name}/tab/{tab}` — returns HTML partial
- URL persistence: `hx-push-url` updates the browser URL
- No full page reload when switching tabs

### App Actions

| Action | Method | URL | Description |
|--------|--------|-----|-------------|
| Scale | POST | `/apps/{name}/scale` | Update replica count |
| Delete | DELETE | `/apps/{name}` | Remove app + all resources |

---

## Services

### Service List

**URL**: `/services`

Table of all provisioned service instances:
- Name, type, plan, status
- Bound applications
- Actions (bind, unbind, delete)

### Service Detail

**URL**: `/services/{name}`

Detailed view of a single service:
- Type, plan, status
- Resource allocations
- Credentials (masked)
- Bound applications list

### Service Actions

| Action | Method | URL |
|--------|--------|-----|
| Create | POST | `/services/create` |
| Bind to app | POST | `/services/{name}/bind` |
| Unbind from app | POST | `/services/{name}/unbind` |
| Delete | DELETE | `/services/{name}` |

---

## Secrets

### Secret List

**URL**: `/secrets`

Lists all secrets in the namespace:
- Secret name, type (service/user), key count
- Created timestamp
- Actions: view, reveal, delete

### Secret Detail

**URL**: `/secrets/{name}`

Shows secret keys with masked values. Click individual keys to reveal:
- **Reveal endpoint**: `GET /secrets/{name}/reveal/{key}` — returns unmasked value via HTMX

### Create Secret

**URL**: `GET /secrets/new` (form) → `POST /secrets` (submit)

Form for creating user secrets with key-value pairs.

---

## Metrics & Alerts

### Monitoring Page

**URL**: `/monitoring`

Displays:
- Platform-level Grafana dashboard embeds
- Links to Prometheus and AlertManager

### Alerts

**URL**: `/monitoring/alerts`

Live alert list from AlertManager:
- Alert name, severity, state (firing/pending/resolved)
- Labels, annotations, duration
- Auto-refreshes

---

## Cluster Management

### Cluster List

**URL**: `/clusters`

Table of registered Kubernetes clusters:
- Name, provider, region, status
- Kubernetes version, node count, app count
- Active cluster indicator
- Actions: activate, view details, remove

### Cluster Detail

**URL**: `/clusters/{id}`

Detailed cluster information:
- Connection status, K8s version
- Node table: name, roles, status, CPU, memory, version, OS
- Resource usage: pod count, CPU/memory capacity
- App count in namespace

### Cluster Actions

| Action | Method | URL |
|--------|--------|-----|
| Add cluster | POST | `/clusters` |
| Set active | POST | `/clusters/{id}/activate` |
| Remove | DELETE | `/clusters/{id}` |

---

## Service Catalog

### Browse Catalog

**URL**: `/catalog`

Service types grouped by category with cards showing:
- Service name, description, provider
- Available plans with resources
- Tags
- Visibility toggle (admin only)

### Topology Management

**URL**: `/topologies/{type}/{plan}`

Terraform topology viewer for service plans:
- View HCL template
- Upload custom topology
- Preview rendered template
- Delete topology

### Visibility Control

Toggle service/plan visibility for the marketplace:
- `POST /catalog/{type}/visibility` — toggle entire service type
- `POST /catalog/{type}/{plan}/visibility` — toggle individual plan

---

## Platform Settings

### Registry Configuration

**URL**: `/settings/registry`

Configure a container registry for image storage:

| Field | Description |
|-------|-------------|
| Registry URL | e.g., `harbor.local:30003` |
| Project | Registry project/namespace |
| Username | Registry username |
| Password | Leave blank to keep existing |
| Skip TLS | For self-signed certificates |
| Enable Push | Master toggle for registry integration |

**Test Connection**: `POST /settings/registry/test` — verifies registry connectivity.

Settings are stored in K8s ConfigMap (`mf-platform-settings`) and Secret (`mf-platform-credentials`).

### Webhooks

**URL**: `/settings/webhooks`

Configure HTTP webhooks for platform events:

| Field | Description |
|-------|-------------|
| Name | Webhook display name |
| URL | HTTP endpoint to call |
| Events | Multi-select from available event types |
| Enabled | Toggle per webhook |

Available event types:
- `app.deployed`, `app.crashed`, `app.scaled`, `app.deleted`
- `alert.fired`, `alert.resolved`
- `service.created`, `service.deleted`

Actions:
- **Add**: `POST /settings/webhooks`
- **Test**: `POST /settings/webhooks/{id}/test` — sends test payload
- **Delete**: `DELETE /settings/webhooks/{id}`

### SMTP Configuration

**URL**: `/settings/smtp`

Configure SMTP for email notifications:

| Field | Description |
|-------|-------------|
| SMTP Host | e.g., `smtp.gmail.com` |
| Port | Default: 587 |
| Username | SMTP username |
| Password | Leave blank to keep existing |
| From Address | Sender email address |
| Use TLS | Enable TLS encryption |
| Enable SMTP | Master toggle |

**Test Connection**: `POST /settings/smtp/test` — verifies SMTP connectivity.

---

## Users & IAM

**URL**: `/users`

The Users & IAM page provides complete identity and access management through a 4-tab interface. Tabs load dynamically via HTMX (`hx-get="/users/tab/{tab}"`, `hx-target="#tab-content"`, `hx-push-url="/users?tab={tab}"`).

### Orgs Tab

**URL**: `/users?tab=orgs`

Organization management with a two-column layout:

- **Left panel** — list of user's organizations, create org form
- **Right panel** — selected org detail, members table, invite form

| Action | Method | URL |
|--------|--------|-----|
| Create org | POST | `/users/orgs` |
| Delete org | DELETE | `/users/orgs/{id}` |
| Invite member | POST | `/users/orgs/{id}/members` |
| Remove member | DELETE | `/users/orgs/{id}/members/{email}` |
| Set role | POST | `/users/orgs/{id}/members/{email}/role` |
| Switch active | POST | `/users/orgs/{id}/activate` |

Roles: **admin** (full org access), **member** (apps/services/secrets), **viewer** (read-only).

### Users Tab

**URL**: `/users?tab=users`

Keycloak user management (requires Keycloak admin credentials in config):

- User table: Username, Email, Name, Roles (as badges), Enabled/Disabled status
- Search bar with real-time filtering by username or email
- Create user form: username, email, first/last name, password
- Per-user actions: toggle enable/disable, assign realm roles, delete

| Action | Method | URL |
|--------|--------|-----|
| Create user | POST | `/users/keycloak` |
| Toggle enable/disable | POST | `/users/keycloak/{id}/toggle` |
| Assign role | POST | `/users/keycloak/{id}/roles` |
| Delete user | DELETE | `/users/keycloak/{id}` |

Realm roles: **platform-admin** (full platform access), **org-admin**, **org-member**, **viewer**.

### Policies Tab

**URL**: `/users?tab=policies`

View and manage OPA authorization policies:

- Read-only display of the embedded default policy (`authz.rego`)
- Custom policy editor with name and Rego source fields
- Save button validates Rego syntax before applying (copy-on-write — invalid Rego never corrupts live policies)
- Error messages displayed for syntax errors

| Action | Method | URL |
|--------|--------|-----|
| Save policy | POST | `/users/policies` |

### Audit Tab

**URL**: `/users?tab=audit`

Authorization decision log (in-memory ring buffer, last 1000 entries):

- Table columns: Timestamp, User Email, Action, Resource, Path, Method, Allowed/Denied
- Filter dropdowns: by user, by resource, by action
- Allow (green) / Deny (red) status badges
- Entry count indicator showing total logged decisions

---

## SCIM v2 API

MicroFoundry implements SCIM v2 (RFC 7643/7644) endpoints for standard identity provisioning. All SCIM endpoints require the `platform-admin` role and use `Content-Type: application/scim+json`.

### Endpoints

| Method | Endpoint | Description | Status Codes |
|--------|----------|-------------|-------------|
| GET | `/scim/v2/Users` | List users with pagination & filtering | 200, 400, 503 |
| POST | `/scim/v2/Users` | Create user | 201, 400, 409, 503 |
| GET | `/scim/v2/Users/{id}` | Get user by ID | 200, 400, 404, 503 |
| PUT | `/scim/v2/Users/{id}` | Replace user (full update) | 200, 400, 404, 503 |
| PATCH | `/scim/v2/Users/{id}` | Partial update (PatchOp) | 200, 400, 404, 503 |
| DELETE | `/scim/v2/Users/{id}` | Delete user | 204, 400, 503 |
| GET | `/scim/v2/ServiceProviderConfig` | Provider capabilities | 200 |
| GET | `/scim/v2/ResourceTypes` | Supported resource types | 200 |
| GET | `/scim/v2/Schemas` | Schema definitions | 200 |

### Query Parameters (List Users)

| Parameter | Default | Description |
|-----------|---------|-------------|
| `startIndex` | 1 | SCIM 1-based pagination start |
| `count` | 20 | Results per page (max: 100) |
| `filter` | — | SCIM filter expression |

Supported filter operators: `eq`, `co`, `sw`. Supported attributes: `userName`, `emails.value`. Compound filters (`and`/`or`) are not supported.

Example: `GET /scim/v2/Users?filter=userName eq "admin"&count=10`

### Request/Response Examples

**List Users:**

```bash
curl -H "Authorization: Bearer TOKEN" \
  "http://localhost:8080/scim/v2/Users?startIndex=1&count=20"
```

```json
{
  "schemas": ["urn:ietf:params:scim:api:messages:2.0:ListResponse"],
  "totalResults": 3,
  "itemsPerPage": 20,
  "startIndex": 1,
  "Resources": [
    {
      "schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "userName": "admin",
      "name": { "givenName": "Admin", "familyName": "User" },
      "emails": [{ "value": "admin@example.com", "type": "work", "primary": true }],
      "active": true,
      "meta": {
        "resourceType": "User",
        "created": "2025-01-15T10:00:00Z",
        "location": "http://localhost:8080/scim/v2/Users/550e8400-e29b-41d4-a716-446655440000"
      }
    }
  ]
}
```

**Create User:**

```bash
curl -X POST -H "Content-Type: application/scim+json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"newuser","name":{"givenName":"New","familyName":"User"},"emails":[{"value":"new@example.com","primary":true}],"active":true}' \
  "http://localhost:8080/scim/v2/Users"
```

Returns `201 Created` with `Location` header pointing to the new user resource.

**Patch User (disable):**

```bash
curl -X PATCH -H "Content-Type: application/scim+json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"active","value":false}]}' \
  "http://localhost:8080/scim/v2/Users/550e8400-e29b-41d4-a716-446655440000"
```

### Validation

All `{id}` path parameters are validated as UUIDs. Non-UUID values return a `400` SCIM error:

```json
{
  "schemas": ["urn:ietf:params:scim:api:messages:2.0:Error"],
  "detail": "invalid user ID format",
  "status": "400"
}
```

---

## API Reference

All API endpoints return JSON and are available at `/api/...`.

### Application APIs

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/apps` | List all applications |
| GET | `/api/apps/{name}` | Get application detail |
| GET | `/api/apps/{name}/logs/history` | Query historical logs |
| POST | `/api/apps/{name}/scale` | Scale application |
| DELETE | `/api/apps/{name}` | Delete application |
| GET | `/api/apps/{name}/red-metrics` | Get RED metrics |
| GET | `/api/apps/{name}/health` | Get app health status |
| GET | `/api/apps/{name}/observability` | Combined observability data |

### Service APIs

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/services` | List provisioned services |
| GET | `/api/services/{name}` | Get service detail |
| GET | `/api/catalog` | Get full service catalog |
| GET | `/api/catalog/visible` | Get visible catalog only |

### Secret APIs

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/secrets` | List all secrets |
| GET | `/api/secrets/{name}` | Get secret detail |
| POST | `/api/secrets` | Create a secret |

### Cluster APIs

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/clusters` | List clusters |
| GET | `/api/clusters/{id}/health` | Check cluster health |

### Settings APIs

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/settings` | Get all platform settings |
| PUT | `/api/settings/registry` | Save registry config |
| GET | `/api/settings/webhooks` | List webhooks |
| POST | `/api/settings/webhooks` | Create webhook |
| DELETE | `/api/settings/webhooks/{id}` | Delete webhook |
| PUT | `/api/settings/smtp` | Save SMTP config |

### Topology APIs

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/topologies` | List all topologies |
| GET | `/api/topologies/{type}/{plan}` | Get topology detail |
| PUT | `/api/topologies/{type}/{plan}` | Save topology |

### Monitoring APIs

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/monitoring/alerts` | Get active alerts |

### Organization APIs

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/orgs` | List organizations |
| GET | `/api/orgs/{id}` | Get organization detail |
| POST | `/api/orgs` | Create organization |
| GET | `/api/orgs/{id}/members` | List members |

### IAM APIs

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/users` | List Keycloak users (with `?search=` filter) |
| GET | `/api/policies` | Get all OPA policies (name → Rego source) |
| PUT | `/api/policies` | Update an OPA policy (JSON: `name`, `source`) |
| GET | `/api/audit` | Query audit log (`?user=`, `?resource=`, `?action=`) |

### Config API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/config` | Get platform configuration |
