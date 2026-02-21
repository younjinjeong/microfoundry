# MicroFoundry

[![CI](https://github.com/younjinjeong/microfoundry/actions/workflows/ci.yml/badge.svg?branch=rc)](https://github.com/younjinjeong/microfoundry/actions/workflows/ci.yml)
[![Release](https://github.com/younjinjeong/microfoundry/actions/workflows/release.yml/badge.svg)](https://github.com/younjinjeong/microfoundry/releases)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Release](https://img.shields.io/badge/Release-v0.2.0-blue.svg)](https://github.com/younjinjeong/microfoundry/releases/tag/v0.2.0)

**A micro CloudFoundry for Kubernetes** — lightweight PaaS that preserves the CloudFoundry developer experience while running on cloud-native infrastructure.

MicroFoundry replaces the heavyweight BOSH/Diego runtime with Kubernetes, managed cloud services, and modern observability. The result: `cf push`-style deployments, service binding, and logging — all backed by Kubernetes, Prometheus, Loki, and Grafana Beyla.

> **Built with AI** — Developed through a structured Human-AI workflow using [Claude Code](https://claude.ai/claude-code) with a 7-agent review process. See [Development Workflow](docs/development-workflow.md) for details. AI agents can read [`ai/AGENTS.md`](ai/AGENTS.md) for fast onboarding.

---

## Why MicroFoundry?

| Problem | Solution |
|---|---|
| **CF is too heavy** — BOSH + Diego + 20+ VMs | **Single Go binary** on any K8s cluster |
| **Losing CF developer experience** when moving to K8s | **`mf push` works like `cf push`** — same workflow, K8s underneath |
| **Observability requires code changes** | **Zero-code eBPF metrics** via Grafana Beyla |
| **No platform visibility** without multiple tools | **Built-in admin dashboard** — 48 templates, all server-rendered |
| **IAM is bolted on** — separate auth/authz systems | **Keycloak + OPA + SCIM v2** with OIDC federation to AWS/GCP/Azure |
| **Multi-cluster is hard** | **One control plane** — Docker Desktop, EKS, GKE, AKS |

**Key features:**

- **56 backing services** — 10 local K8s + 21 AWS + 12 GCP + 13 Azure, each with 3 plans
- **OIDC CSP federation** — Keycloak-brokered temporary credentials for AWS STS, GCP WIF, Azure FIC
- **5-tier RBAC** — platform-admin → workspace-admin → org-admin → member → viewer
- **MCP Server** — AI tools (Claude, Cursor) deploy and manage apps directly
- **Cloud deployment packages** — Terraform blueprints for EKS, ECS Fargate, GKE, AKS, local K8s
- **Cross-platform release** — GoReleaser for Linux/macOS/Windows, multi-arch Docker, Helm OCI

---

## Quick Start

### Prerequisites

- Go 1.25+, Docker Desktop with Kubernetes, kubectl, Helm 3

### Build & Deploy

```bash
make build                               # Build to bin/mf
make monitoring-install                  # Prometheus + Grafana + Loki + Beyla
mf push hello-world                      # Deploy from source
mf create-service postgresql small my-db # Provision a database
mf bind-service hello-world my-db        # Bind DB → app (VCAP_SERVICES)
mf admin                                 # Open dashboard at :8443
```

### Authentication (Optional)

```bash
mf setup keycloak                        # Deploy Keycloak
mf setup keycloak-realm --url http://localhost:8180
# Add auth config to configs/mf.yaml, restart mf admin
```

### Configuration

```bash
cp configs/mf.example.yaml configs/mf.yaml
```

```yaml
kubernetes:
  active: "docker-desktop"
  clusters:
    docker-desktop:
      context: "docker-desktop"
      namespace: "microfoundry"
      domain: "cf-local.dev"
      provider: "docker-desktop"
    eks-prod:
      context: "arn:aws:eks:us-west-2:..."
      namespace: "microfoundry"
      domain: "apps.example.com"
      provider: "eks"
```

---

## Admin Dashboard

The built-in dashboard (`mf admin`) provides application lifecycle, service catalog, multi-cluster management, observability, secrets, IAM, and platform settings in one interface.

<p align="center">
  <img src="docs/images/dashboard-walkthrough.gif" alt="MicroFoundry Admin Dashboard Walkthrough" width="900">
</p>

<details>
<summary><strong>Screenshots</strong> (click to expand)</summary>
<br>

| Dashboard | Applications | Service Catalog |
|:---------:|:------------:|:---------------:|
| <img src="docs/images/dashboard.png" alt="Dashboard" width="280"> | <img src="docs/images/apps-list.png" alt="Applications" width="280"> | <img src="docs/images/catalog.png" alt="Catalog" width="280"> |

| Users & IAM | Workspaces | Clusters |
|:-----------:|:----------:|:--------:|
| <img src="docs/images/users-iam.png" alt="Users & IAM" width="280"> | <img src="docs/images/workspaces.png" alt="Workspaces" width="280"> | <img src="docs/images/clusters.png" alt="Clusters" width="280"> |

| Monitoring & Alerts | Services | Secrets |
|:-------------------:|:--------:|:-------:|
| <img src="docs/images/monitoring.png" alt="Monitoring" width="280"> | <img src="docs/images/services.png" alt="Services" width="280"> | <img src="docs/images/secrets.png" alt="Secrets" width="280"> |

| Service Endpoints | Registry Settings | Platform Config |
|:-----------------:|:-----------------:|:---------------:|
| <img src="docs/images/settings-endpoints.png" alt="Endpoints" width="280"> | <img src="docs/images/settings-registry.png" alt="Registry" width="280"> | <img src="docs/images/config.png" alt="Config" width="280"> |

</details>

**Admin pages:** Dashboard, Applications, Services, Secrets, Users & Orgs (5-tab IAM), Clusters, Service Catalog, Registry, Webhooks, SMTP, Endpoints, Cloud Providers, Metrics & Alerts, Platform, Documentation

---

## Architecture

```
                          ┌─────────────────────────────────┐
                          │         Developer / AI          │
                          └──────────┬──────────┬───────────┘
                                     │          │
                              ┌──────▽──┐  ┌────▽──────┐
                              │  CLI    │  │  MCP      │
                              │ mf push │  │  Server   │
                              └──────┬──┘  └────┬──────┘
                                     │          │
                              ┌──────▽──────────▽──────┐
                              │  MicroFoundry Admin    │
                              │  API + Dashboard       │
                              └─────────────┬──────────┘
                                            │
          ┌─────────────────────────────────┼─────────────────────────────────┐
          │                                 │                                 │
  ┌───────▽────────┐              ┌─────────▽──────────┐            ┌────────▽────────┐
  │  Build System  │              │  Kubernetes API     │            │  Observability  │
  │  Dockerfile    │              │  Deployments        │            │  Prometheus     │
  │  CNB/Paketo    │              │  Services/Ingress   │            │  Grafana + Loki │
  └────────────────┘              │  Secrets/ConfigMaps │            │  Beyla (eBPF)   │
                                  └─────────┬──────────┘            └─────────────────┘
                                            │
                    ┌───────────────────────┼───────────────────────┐
                    │                       │                       │
            ┌───────▽───────┐       ┌───────▽───────┐      ┌───────▽───────┐
            │ API Gateway   │       │ Backing       │      │ IAM           │
            │ Kong/Nginx/   │       │ Services      │      │ Keycloak OIDC │
            │ AWS API GW    │       │ 56 types      │      │ OPA + SCIM v2 │
            └───────────────┘       └───────────────┘      └───────────────┘
```

For detailed architecture, multi-cluster runtime, and component design, see [Architecture](docs/architecture.md).

---

## Service Catalog

56 backing services across 4 providers, each with 3 plans (small / medium / large):

| Provider | Count | Services |
|----------|-------|----------|
| **Local K8s** | 10 | MariaDB, PostgreSQL, ClickHouse, Redis, Memcached, RabbitMQ, ActiveMQ, MinIO, Kong, Nginx |
| **AWS** | 21 | RDS (PostgreSQL, MySQL, MariaDB), Aurora, DynamoDB, DocumentDB, Redshift, ElastiCache, SQS, SNS, MQ, MSK, Kinesis, S3, OpenSearch, Bedrock, SageMaker, MediaConvert, IVS |
| **GCP** | 12 | Cloud SQL (PostgreSQL, MySQL), AlloyDB, Spanner, Firestore, Bigtable, BigQuery, Memorystore, Pub/Sub, Cloud Storage, Vertex AI, Transcoder |
| **Azure** | 13 | Database (PostgreSQL, MySQL), SQL Database, Cosmos DB, Synapse, Cache for Redis, Service Bus, Event Hubs, Blob Storage, AI Search, Azure OpenAI, ML, Media Services |

```bash
mf catalog                                # List all services
mf create-service postgresql small my-db  # Provision
mf bind-service hello-world my-db         # Bind → VCAP_SERVICES
```

Cloud providers support **OIDC federation** via Keycloak — no static credentials needed. See [Admin Guide](docs/admin-guide.md) for setup.

---

## CLI Commands

| Command | Description |
|---|---|
| `mf push [app]` | Build + deploy from source (Dockerfile or CNB) |
| `mf apps` / `mf app [name]` | List apps / show app details |
| `mf logs [app]` | Stream or fetch application logs |
| `mf scale [app] -i N` | Scale application instances |
| `mf delete [app]` | Delete app and clean up routes |
| `mf catalog` | List available services by provider |
| `mf create-service` / `mf delete-service` | Provision / delete a backing service |
| `mf services` / `mf bind-service` / `mf unbind-service` | List / bind / unbind services |
| `mf secrets` / `mf create-secret` / `mf delete-secret` | Manage secrets |
| `mf admin` | Start web dashboard |
| `mf setup keycloak` / `keycloak-realm` / `keycloak-idp` | Authentication setup |
| `mf users` / `mf create-user` | Keycloak user management |
| `mf orgs` / `mf create-org` | Organization management |
| `mf auth login` | OIDC authentication |

### MCP Server

9 tools for AI integration: `mf_push`, `mf_apps`, `mf_logs`, `mf_scale`, `mf_delete`, `mf_create_service`, `mf_bind_service`, `mf_routes`, `mf_env`

---

## Cloud Deployment

Terraform deployment packages for all major providers. Cost-optimized defaults — if MicroFoundry goes down, existing apps keep running on K8s.

| Platform | Architecture | Estimated Cost |
|---|---|---|
| **AWS ECS Fargate + EKS** | Fargate control plane + EKS workloads | ~$108/month |
| **AWS ECS Fargate only** | Connect to existing EKS cluster | ~$20/month |
| **AWS EKS** | Helm install on EKS | Depends on cluster |
| **GCP GKE** | Helm install on GKE Autopilot/Standard | Depends on cluster |
| **Azure AKS** | Helm install on AKS | Depends on cluster |
| **Local K8s** | Docker Desktop / kind / minikube | Free |

See [deploy/packages/](deploy/packages/) for detailed setup guides per provider.

---

## Tech Stack

| Layer | Technology | Purpose |
|---|---|---|
| **Language** | Go 1.25 | API server, CLI, MCP server |
| **CLI** | Cobra + Viper | Commands + configuration |
| **Runtime** | Kubernetes | Scheduling and orchestration |
| **Build** | Cloud Native Buildpacks | Source-to-container builds |
| **Ingress** | Kong / Nginx / Traefik / AWS API GW | Pluggable gateway with WebSocket/gRPC |
| **TLS** | mkcert | Local HTTPS with `.dev` domains |
| **Metrics** | Prometheus + Grafana + Beyla (eBPF) | Zero-code auto-instrumented metrics |
| **Logs** | Promtail + Loki | Log aggregation |
| **Auth** | Keycloak + go-oidc + OPA | OIDC + Rego policies + SCIM v2 |
| **UI** | Go templates + HTMX + Tailwind CSS | Server-rendered, no JS build step |
| **IaC** | Terraform | Cloud resource provisioning |
| **AI** | Model Context Protocol (MCP) | AI tool integration |
| **CSP** | AWS STS + GCP WIF + Azure FIC | OIDC credential federation |
| **Security** | gitleaks + pre-commit | Secret detection |

---

## Documentation

All docs are also available through the **in-app docs viewer** at `/docs` in the admin dashboard.

| Document | Description |
|----------|-------------|
| [User Manual](docs/user-manual.md) | Deploying and managing applications |
| [Architecture](docs/architecture.md) | Technical design and project structure |
| [Admin Guide](docs/admin-guide.md) | Dashboard pages and API reference (100+ endpoints) |
| [Development Workflow](docs/development-workflow.md) | Human-AI collaborative development process |
| [CF vs MicroFoundry](docs/cloudfoundry-vs-microfoundry.md) | Component-by-component comparison |
| [CF Architecture](docs/cloudfoundry-architecture.md) | CloudFoundry reference architecture |
| [Observability & Capacity](docs/observability-capacity.md) | Monitoring and capacity planning |
| [Changelog](CHANGELOG.md) | Development history and releases |

---

## Contributing

1. Fork the repository
2. Create a feature branch from `rc` (not `main`)
3. Make your changes
4. Submit a PR targeting `rc`

This project uses a structured AI-assisted workflow. See [CLAUDE.md](CLAUDE.md) for branch strategy and conventions, and [Development Workflow](docs/development-workflow.md) for the full process.

```bash
make hooks              # Install pre-commit hooks
go build ./...          # Must pass
go vet ./...            # Must pass
go test ./...           # Run tests
```

---

## Release

| Version | Tag | Description |
|---------|-----|-------------|
| **v0.2.0** | [`v0.2.0`](https://github.com/younjinjeong/microfoundry/releases/tag/v0.2.0) | Multi-cloud service catalog (56 services), OIDC CSP federation, README redesign |
| **v0.1.0** | [`v0.1.0`](https://github.com/younjinjeong/microfoundry/releases/tag/v0.1.0) | First release — GoReleaser, multi-arch Docker, Helm OCI, 5 cloud deployment packages |

---

## License

See [LICENSE](LICENSE).
