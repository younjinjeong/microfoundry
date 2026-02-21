# Changelog

All notable changes to MicroFoundry are documented here.

## [v0.2.0] — 2026-02-21

### Added
- Multi-cloud service catalog — 56 services across 4 providers (10 local K8s + 21 AWS + 12 GCP + 13 Azure)
- OIDC federation for CSP authentication via Keycloak (AWS STS, GCP WIF, Azure FIC)
- Cloud Provider settings page with dual auth mode (static credentials + OIDC federation)
- ECS Fargate deployment package with multi-cluster control plane
- README redesign following best practices

## [v0.1.0] — 2026-02-18

### Added
- First release — GoReleaser cross-compilation, multi-arch Docker images, Helm OCI chart
- 5 cloud deployment packages (AWS EKS, AWS ECS Fargate, GCP GKE, Azure AKS, local K8s)

---

## Development History

MicroFoundry has been built incrementally through a series of Epics:

| PR | Epic | Description |
|----|------|-------------|
| [#2](https://github.com/younjinjeong/microfoundry/pull/2) | Local K8s Runtime | `mf push`, `mf apps`, `mf logs`, `mf scale`, `mf delete` — core CLI |
| [#4](https://github.com/younjinjeong/microfoundry/pull/4) | Admin Dashboard | Web admin interface with HTMX, dashboard, app management |
| [#6](https://github.com/younjinjeong/microfoundry/pull/6) | App Detail View | 8-tab application detail view (Overview → Performance) |
| [#8](https://github.com/younjinjeong/microfoundry/pull/8) | Multi-Cluster | Multi-cluster Kubernetes management with ClientManager |
| [#10](https://github.com/younjinjeong/microfoundry/pull/10) | Backing Services | Service catalog, provisioning, binding, VCAP_SERVICES |
| [#11](https://github.com/younjinjeong/microfoundry/pull/11) | Real K8s Provisioning | StatefulSet + PVC provisioning for all 10 service types |
| [#13](https://github.com/younjinjeong/microfoundry/pull/13) | Terraform Topologies | Terraform-based service topology management |
| [#15](https://github.com/younjinjeong/microfoundry/pull/15) | Catalog & Visibility | Service catalog browser with plan visibility controls |
| [#17](https://github.com/younjinjeong/microfoundry/pull/17) | Monitoring & Logging | Prometheus + Loki + Grafana + AlertManager integration |
| [#20](https://github.com/younjinjeong/microfoundry/pull/20) | Beyla eBPF | Netflix Atlas-inspired auto-instrumentation with Beyla |
| [#22](https://github.com/younjinjeong/microfoundry/pull/22) | Observability Hardening | Security, resilience & capacity for monitoring stack |
| [#24](https://github.com/younjinjeong/microfoundry/pull/24) | Platform Config | Registry, webhooks, SMTP settings via admin UI |
| [#26](https://github.com/younjinjeong/microfoundry/pull/26) | Keycloak UAA | OIDC authentication with Keycloak, sessions, org management |
| [#31](https://github.com/younjinjeong/microfoundry/pull/31) | E2E Testing | Playwright E2E test suite (82 test cases, 8 suites) |
| [#34](https://github.com/younjinjeong/microfoundry/pull/34) | IAM & SCIM | Keycloak user CRUD, SCIM v2, OPA authorization, audit log |
| [#37](https://github.com/younjinjeong/microfoundry/pull/37) | IAM Hardening | Authz bypass fix, error handling, SCIM compliance, OPA atomicity |
| [#40](https://github.com/younjinjeong/microfoundry/pull/40) | Docs Sync #3 | Documentation sync for IAM, SCIM v2, OPA & Audit |
| [#42](https://github.com/younjinjeong/microfoundry/pull/42) | Local TLS | mkcert TLS for `.dev` HTTPS access |
| [#44](https://github.com/younjinjeong/microfoundry/pull/44) | Contextual Tooltips | Tooltips across all admin UI pages |
| [#46](https://github.com/younjinjeong/microfoundry/pull/46) | Admin Domain | Configurable domain name with auto-TLS |
| [#48](https://github.com/younjinjeong/microfoundry/pull/48) | Pluggable Gateway | nginx/kong/traefik/AWS API Gateway support |
| [#50](https://github.com/younjinjeong/microfoundry/pull/50) | Protocol Support | WebSocket and gRPC protocol support for routes |
| [#54](https://github.com/younjinjeong/microfoundry/pull/54) | Service Creation UI | Service creation form + bind/unbind UI |
| [#58](https://github.com/younjinjeong/microfoundry/pull/58) | User/Org CLI | User and organization management CLI commands |
| [#61](https://github.com/younjinjeong/microfoundry/pull/61) | Service Endpoints | Configurable service endpoints with K8s auto-discovery |
| [#64](https://github.com/younjinjeong/microfoundry/pull/64) | Workspace RBAC | Workspace hierarchy, 5-tier RBAC, CLI auth |
| [#65](https://github.com/younjinjeong/microfoundry/pull/65) | Workspace IAM Tab | Workspaces tab in Users & Organizations page |
| [#83](https://github.com/younjinjeong/microfoundry/pull/83) | Multi-Cloud Catalog | 46 cloud services across AWS, GCP, Azure |
| [#85](https://github.com/younjinjeong/microfoundry/pull/85) | OIDC CSP Federation | Keycloak-brokered OIDC for AWS STS, GCP WIF, Azure FIC |

### External Contributions

| PR | Author | Description |
|----|--------|-------------|
| [#29](https://github.com/younjinjeong/microfoundry/pull/29) | [@byunjuneseok](https://github.com/byunjuneseok) | Fix: add EnsureNamespace to create-service command |
