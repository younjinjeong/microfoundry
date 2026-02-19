# MicroFoundry — AI Agent Navigator

> Quick-start guide for AI tools and agents working in this repository.
> Read this first. It tells you what this project is, how it's structured, and how to do things.

## What Is This?

MicroFoundry is a **micro CloudFoundry for Kubernetes** — a single Go binary that gives developers `cf push`-style deployments on any K8s cluster. It includes a CLI (`mf`), a web admin dashboard, an MCP server for AI tool integration, and a full observability stack.

**Module**: `github.com/younjinjeong/microfoundry`
**Language**: Go 1.25
**Entry point**: `cmd/mf/main.go`

## Quick Commands

```bash
# Build
go build ./...            # compile everything
go build -o bin/mf ./cmd/mf  # build the binary

# Verify
go vet ./...              # lint
go test ./... -v -count=1 # test

# Run
./bin/mf push <app>       # deploy an app
./bin/mf admin            # start admin dashboard (default :8443 with TLS)
./bin/mf catalog          # list available backing services
```

## Project Map

```
cmd/mf/           CLI commands (Cobra). One file per command.
                   main.go is the root. push.go, apps.go, admin.go, etc.

pkg/admin/         Web dashboard. HTTP handlers + Go templates + HTMX.
                   server.go = routes. handlers.go = page handlers.
                   static/templates/ = HTML pages. static/css/ = styles.

pkg/k8s/           Kubernetes client. client.go wraps client-go.
                   manager.go = multi-cluster ClientManager.
                   app.go = deploy/scale/delete. ingress.go = routes.

pkg/auth/          Authentication & authorization.
                   oidc.go = Keycloak OIDC. opa.go = policy engine.
                   workspace.go = multi-tenant hierarchy.

pkg/service/       Backing service broker.
                   catalog.go = 10 service types. provisioner.go = K8s provisioning.
                   binder.go = VCAP_SERVICES injection.

pkg/config/        Multi-cluster configuration (Viper + YAML).
pkg/build/         Source-to-image (Dockerfile + Cloud Native Buildpacks).
pkg/monitoring/    Prometheus, Grafana, Loki, Beyla integration.
pkg/models/        Shared data types.
pkg/hosts/         /etc/hosts management for local dev.
pkg/tls/           mkcert TLS certificate generation.
pkg/secrets/       K8s Secret management.
pkg/settings/      Platform settings (ConfigMap/Secret store).
pkg/terraform/     Terraform topology management.

docs/              Markdown documentation (embedded in admin UI via embed.FS).
deploy/            K8s manifests, Helm chart, monitoring stack, cloud packages.
test/              Playwright E2E tests.
configs/           Example YAML configuration.
```

## Key Patterns

**Templates**: Go `html/template` with `embed.FS`. Clone pattern — each page gets its own template clone. See `pkg/admin/templates.go`.

**Routes**: All registered in `pkg/admin/server.go` using `mux.HandleFunc("METHOD /path", handler)`.

**K8s storage**: No database. All state lives in Kubernetes objects (ConfigMaps, Secrets, Deployments, Ingresses).

**Settings**: Stored in K8s ConfigMaps/Secrets via `pkg/settings/`. Never file-based.

**Auth flow**: Keycloak OIDC -> session cookie -> OPA middleware -> handler. See `pkg/auth/`.

**Service provisioning**: `mf create-service` -> `pkg/service/provisioner.go` -> creates StatefulSet + PVC + Service + Secret in K8s.

## Configuration

Primary config file: `configs/mf.yaml` (see `configs/mf.example.yaml` for schema).

```yaml
kubernetes:
  active: "docker-desktop"
  clusters:
    docker-desktop:
      context: "docker-desktop"
      namespace: "microfoundry"
      domain: "cf-local.dev"
      provider: "docker-desktop"
```

## CLI Commands (19 files in cmd/mf/)

| File | Commands |
|------|----------|
| `push.go` | `mf push` — build + deploy app |
| `apps.go` | `mf apps`, `mf app <name>` |
| `logs.go` | `mf logs <app>` |
| `scale.go` | `mf scale <app> -i N` |
| `delete.go` | `mf delete <app>` |
| `catalog.go` | `mf catalog` |
| `create_service.go` | `mf create-service <type> <plan> <name>` |
| `services.go` | `mf services`, `mf service <name>` |
| `bind_service.go` | `mf bind-service <app> <svc>` |
| `unbind_service.go` | `mf unbind-service <app> <svc>` |
| `delete_service.go` | `mf delete-service <name>` |
| `secrets.go` | `mf secrets`, `mf create-secret`, `mf delete-secret` |
| `admin.go` | `mf admin` — start web dashboard |
| `setup.go` | `mf setup keycloak/keycloak-realm/keycloak-idp` |
| `users.go` | `mf users`, `mf create-user` |
| `orgs.go` | `mf orgs`, `mf create-org` |
| `workspaces.go` | `mf workspaces`, `mf create-workspace` |
| `login.go` | `mf auth login` |
| `main.go` | Root command + version |

## Admin Dashboard Pages

16+ pages served at `:8443`. Key routes:

| Route | Handler File | Purpose |
|-------|-------------|---------|
| `/` | `handlers.go` | Dashboard |
| `/apps` | `handlers.go` | App list |
| `/apps/{name}` | `handlers.go` | App detail (8 tabs) |
| `/services` | `service_handlers.go` | Service list |
| `/secrets` | `secret_handlers.go` | Secret management |
| `/users` | `iam_handlers.go` | Users & Orgs (5 tabs) |
| `/clusters` | `cluster_handlers.go` | Multi-cluster |
| `/catalog` | `handlers.go` | Service catalog |
| `/monitoring` | `monitoring_handlers.go` | Metrics & alerts |
| `/docs` | `handlers.go` | In-app documentation |
| `/settings/platform` | `settings_handlers.go` | DNS, TLS, environment |
| `/settings/registry` | `settings_handlers.go` | Container registry |
| `/settings/endpoints` | `settings_handlers.go` | Service URLs |
| `/settings/webhooks` | `settings_handlers.go` | Webhook config |
| `/settings/smtp` | `settings_handlers.go` | Email config |
| `/workspaces` | `workspace_handlers.go` | Workspace hierarchy |

## Branch Rules (IMPORTANT)

Read `CLAUDE.md` before making changes. Summary:

- **Never push to `main` directly.** A pre-push hook blocks this.
- All PRs target `rc` branch, never `main`.
- Feature branches: `epic/<name>` created from `rc`.
- Flow: `epic/* -> PR -> rc -> validate -> PR -> main`

## Adding a New Feature (Checklist)

1. Branch from `rc`: `git checkout rc && git checkout -b epic/my-feature`
2. Add CLI command in `cmd/mf/` (Cobra)
3. Add business logic in `pkg/`
4. Add admin UI page: handler in `pkg/admin/`, template in `pkg/admin/static/templates/`
5. Register route in `pkg/admin/server.go`
6. Register template in `pkg/admin/templates.go`
7. Verify: `go build ./... && go vet ./...`
8. Create PR targeting `rc`: `gh pr create --base rc`

## Adding a New Admin Page (Checklist)

1. Create handler function in appropriate `*_handlers.go` file
2. Create template in `pkg/admin/static/templates/<name>.html`
3. Add template filename to `pageFiles` slice in `pkg/admin/templates.go`
4. Add route in `pkg/admin/server.go`
5. Add nav link in `pkg/admin/static/templates/partials/nav.html`

## Dependencies

Minimal. Prefer stdlib. Key external deps:

- `k8s.io/client-go` — Kubernetes API
- `github.com/spf13/cobra` + `viper` — CLI + config
- `github.com/coreos/go-oidc/v3` — OIDC auth
- `github.com/open-policy-agent/opa` — Authorization
- `github.com/gorilla/sessions` — HTTP sessions
- `github.com/yuin/goldmark` — Markdown rendering
- `github.com/goreleaser/goreleaser` — Release builds

## MCP Server

MicroFoundry exposes an MCP server for AI tool integration. Tools: `mf_push`, `mf_apps`, `mf_logs`, `mf_scale`, `mf_delete`, `mf_create_service`, `mf_bind_service`, `mf_routes`, `mf_env`.

## Files You Probably Need

| Task | Start Here |
|------|-----------|
| Fix a CLI bug | `cmd/mf/<command>.go` |
| Fix a dashboard bug | `pkg/admin/*_handlers.go` + `static/templates/*.html` |
| Add K8s feature | `pkg/k8s/` |
| Change auth behavior | `pkg/auth/` |
| Modify service catalog | `pkg/service/catalog.go` |
| Update monitoring | `pkg/monitoring/` |
| Change build process | `pkg/build/` |
| Edit deployment manifests | `deploy/` |
| Update documentation | `docs/*.md` |
| Modify CI/CD | `.github/workflows/` |
