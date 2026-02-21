package admin

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/younjinjeong/microfoundry/pkg/auth"
	"github.com/younjinjeong/microfoundry/pkg/config"
	"github.com/younjinjeong/microfoundry/pkg/csp"
	"github.com/younjinjeong/microfoundry/pkg/k8s"
	"github.com/younjinjeong/microfoundry/pkg/models"
	"github.com/younjinjeong/microfoundry/pkg/monitoring"
	"github.com/younjinjeong/microfoundry/pkg/settings"
)

type Server struct {
	clientManager *k8s.ClientManager
	config        *config.Config
	version       string
	templates     *TemplateRenderer
	mux           *http.ServeMux
	metrics       *monitoring.Metrics
	grafana       *monitoring.GrafanaConfig
	loki          *monitoring.LokiClient
	alertmanager  *monitoring.AlertmanagerClient
	prometheus    *monitoring.PrometheusClient
	// Auth (nil when auth is disabled)
	oidcAuth  *auth.OIDCAuthenticator
	sessions  *auth.SessionManager
	orgStore_ *auth.OrgStore
	wsStore   *auth.WorkspaceStore
	// IAM (nil when not configured)
	keycloakAdmin *auth.KeycloakAdminClient
	opa           *auth.OPAEngine
	auditLog      *auth.AuditLog
	// TLS (empty when TLS disabled)
	tlsCertFile string
	tlsKeyFile  string
	// Admin domain (e.g., "admin.cf-local.dev")
	adminDomain string
	tlsEnabled  bool
	// CSP OIDC credential manager (nil when auth disabled)
	cspManager *csp.Manager
	// Endpoint URL hot-swap
	endpointMu sync.RWMutex
}

// ServerOption configures optional Server features.
type ServerOption func(*Server)

// WithAuth enables OIDC authentication on the server.
func WithAuth(oidc *auth.OIDCAuthenticator, sessions *auth.SessionManager, orgStore *auth.OrgStore) ServerOption {
	return func(s *Server) {
		s.oidcAuth = oidc
		s.sessions = sessions
		s.orgStore_ = orgStore
	}
}

// WithWorkspaceStore sets the workspace store.
func WithWorkspaceStore(wsStore *auth.WorkspaceStore) ServerOption {
	return func(s *Server) {
		s.wsStore = wsStore
	}
}

// WithOrgStore sets the org store without full OIDC (e.g. for API use).
func WithOrgStore(orgStore *auth.OrgStore) ServerOption {
	return func(s *Server) {
		s.orgStore_ = orgStore
	}
}

// WithKeycloakAdmin enables Keycloak user management.
func WithKeycloakAdmin(kc *auth.KeycloakAdminClient) ServerOption {
	return func(s *Server) {
		s.keycloakAdmin = kc
	}
}

// WithOPA enables OPA authorization policy engine.
func WithOPA(opa *auth.OPAEngine) ServerOption {
	return func(s *Server) {
		s.opa = opa
	}
}

// WithAuditLog enables authorization audit logging.
func WithAuditLog(al *auth.AuditLog) ServerOption {
	return func(s *Server) {
		s.auditLog = al
	}
}

// WithTLS enables HTTPS on the admin server.
func WithTLS(certFile, keyFile string) ServerOption {
	return func(s *Server) {
		s.tlsCertFile = certFile
		s.tlsKeyFile = keyFile
	}
}

// WithCSPManager sets the CSP OIDC credential manager.
func WithCSPManager(mgr *csp.Manager) ServerOption {
	return func(s *Server) {
		s.cspManager = mgr
	}
}

// WithDomain sets the admin domain name for display in the UI.
func WithDomain(domain string, tlsEnabled bool) ServerOption {
	return func(s *Server) {
		s.adminDomain = domain
		s.tlsEnabled = tlsEnabled
	}
}

func NewServer(clientManager *k8s.ClientManager, cfg *config.Config, version string, opts ...ServerOption) *Server {
	metrics := monitoring.NewMetrics(prometheus.DefaultRegisterer)

	s := &Server{
		clientManager: clientManager,
		config:        cfg,
		version:       version,
		templates:     NewTemplateRenderer(),
		mux:           http.NewServeMux(),
		metrics:       metrics,
		grafana: &monitoring.GrafanaConfig{
			BaseURL: cfg.Monitoring.GrafanaURL,
		},
		loki: &monitoring.LokiClient{
			BaseURL:    cfg.Monitoring.LokiURL,
			HTTPClient: &http.Client{Timeout: 10 * time.Second},
		},
		alertmanager: &monitoring.AlertmanagerClient{
			BaseURL:    cfg.Monitoring.AlertmanagerURL,
			HTTPClient: &http.Client{Timeout: 10 * time.Second},
		},
		prometheus: monitoring.NewPrometheusClient(cfg.Monitoring.PrometheusURL),
	}

	for _, opt := range opts {
		opt(s)
	}

	s.registerRoutes()
	go s.initEndpointsFromK8s()
	return s
}

// GetMetrics returns the metrics instance for external use (e.g., background collector).
func (s *Server) GetMetrics() *monitoring.Metrics {
	return s.metrics
}

// getClient resolves the active K8s client for the current request.
// Priority: cookie "mf-cluster" → config active cluster.
func (s *Server) getClient(r *http.Request) (*k8s.Client, error) {
	if cookie, err := r.Cookie("mf-cluster"); err == nil && cookie.Value != "" {
		return s.clientManager.GetClient(cookie.Value)
	}
	return s.clientManager.GetActiveClient()
}

// UpdateEndpointURLs updates monitoring client BaseURLs from discovered endpoints.
// Only overrides config-file URLs when an explicit override or ingress is found;
// internal cluster DNS is not used because the admin server runs on the host.
func (s *Server) UpdateEndpointURLs(endpoints []models.ServiceEndpoint) {
	s.endpointMu.Lock()
	defer s.endpointMu.Unlock()
	for _, ep := range endpoints {
		// Preserve config-file URLs unless there's a better alternative
		if ep.OverrideURL == "" && !ep.HasIngress {
			continue
		}
		url := ep.ResolvedURL()
		if url == "" {
			continue
		}
		switch ep.Name {
		case "grafana":
			s.grafana.BaseURL = url
		case "loki":
			s.loki.BaseURL = url
		case "alertmanager":
			s.alertmanager.BaseURL = url
		case "prometheus":
			s.prometheus.BaseURL = url
		}
	}
}

// initEndpointsFromK8s resolves platform service endpoints from K8s on startup.
func (s *Server) initEndpointsFromK8s() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := s.clientManager.GetActiveClient()
	if err != nil {
		log.Printf("endpoint discovery: no active client: %v", err)
		return
	}

	store := settings.NewStore(client)
	ps, _ := store.Get(ctx)

	endpoints := client.DiscoverPlatformServices(ctx, client.Domain, ps.Endpoints.Overrides)
	s.UpdateEndpointURLs(endpoints)
	active := 0
	for _, ep := range endpoints {
		if ep.OverrideURL != "" || ep.HasIngress {
			active++
			log.Printf("endpoint discovery: %s → %s", ep.Name, ep.ResolvedURL())
		}
	}
	log.Printf("endpoint discovery: %d/%d services have overrides or ingresses (others use config-file URLs)", active, len(endpoints))
}

// authEnabled returns true if OIDC authentication is configured.
func (s *Server) authEnabled() bool {
	return s.oidcAuth != nil
}

func (s *Server) registerRoutes() {
	// Static CSS files (always public)
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(staticFS())))

	// Prometheus metrics endpoint (always public)
	s.mux.Handle("GET /metrics", monitoring.Handler())

	// Auth routes (always public)
	s.mux.HandleFunc("GET /login", s.LoginPageHandler)
	s.mux.HandleFunc("GET /auth/login", s.AuthLoginHandler)
	s.mux.HandleFunc("GET /auth/callback", s.AuthCallbackHandler)
	s.mux.HandleFunc("GET /auth/logout", s.AuthLogoutHandler)
	s.mux.HandleFunc("GET /denied", s.DeniedHandler)

	// Page routes
	s.mux.HandleFunc("GET /{$}", s.DashboardHandler)
	s.mux.HandleFunc("GET /apps", s.AppsListHandler)
	s.mux.HandleFunc("GET /apps/{name}", s.AppDetailHandler)
	s.mux.HandleFunc("GET /apps/{name}/tab/{tab}", s.AppTabHandler)
	s.mux.HandleFunc("GET /apps/{name}/instances", s.AppInstancesHandler)
	s.mux.HandleFunc("GET /apps/{name}/logs/stream", s.LogStreamHandler)
	s.mux.HandleFunc("GET /apps/{name}/logs/history", s.LogHistoryHandler)
	s.mux.HandleFunc("GET /config", s.ConfigHandler)
	s.mux.HandleFunc("GET /docs", s.DocsHandler)
	s.mux.HandleFunc("GET /services", s.ServicesListHandler)
	s.mux.HandleFunc("GET /services/{name}", s.ServiceDetailHandler)
	s.mux.HandleFunc("GET /catalog", s.CatalogHandler)
	s.mux.HandleFunc("POST /services/create", s.CreateServiceHandler)
	s.mux.HandleFunc("POST /services/{name}/bind", s.BindServiceHandler)
	s.mux.HandleFunc("POST /services/{name}/unbind", s.UnbindServiceHandler)
	s.mux.HandleFunc("DELETE /services/{name}", s.DeleteServiceHandler)
	// Secret routes
	s.mux.HandleFunc("GET /secrets", s.SecretsListHandler)
	s.mux.HandleFunc("GET /secrets/new", s.CreateSecretFormHandler)
	s.mux.HandleFunc("GET /secrets/{name}", s.SecretDetailHandler)
	s.mux.HandleFunc("GET /secrets/{name}/reveal/{key}", s.SecretRevealHandler)
	s.mux.HandleFunc("POST /secrets", s.CreateSecretHandler)
	s.mux.HandleFunc("DELETE /secrets/{name}", s.DeleteSecretHandler)

	// Users & Orgs routes
	s.mux.HandleFunc("GET /users", s.OrgsPageHandler)
	s.mux.HandleFunc("GET /users/tab/{tab}", s.IAMTabHandler)
	s.mux.HandleFunc("POST /users/orgs", s.CreateOrgHandler)
	s.mux.HandleFunc("DELETE /users/orgs/{id}", s.DeleteOrgHandler)
	s.mux.HandleFunc("POST /users/orgs/{id}/members", s.InviteMemberHandler)
	s.mux.HandleFunc("DELETE /users/orgs/{id}/members/{email}", s.RemoveMemberHandler)
	s.mux.HandleFunc("POST /users/orgs/{id}/members/{email}/role", s.SetMemberRoleHandler)
	s.mux.HandleFunc("POST /users/orgs/{id}/activate", s.SwitchOrgHandler)
	// IAM — Keycloak user management
	s.mux.HandleFunc("POST /users/keycloak", s.CreateKeycloakUserHandler)
	s.mux.HandleFunc("DELETE /users/keycloak/{id}", s.DeleteKeycloakUserHandler)
	s.mux.HandleFunc("POST /users/keycloak/{id}/toggle", s.ToggleKeycloakUserHandler)
	s.mux.HandleFunc("POST /users/keycloak/{id}/roles", s.AssignKeycloakRoleHandler)
	// IAM — Workspace management (within /users tab)
	s.mux.HandleFunc("POST /users/workspaces", s.IAMCreateWorkspaceHandler)
	s.mux.HandleFunc("DELETE /users/workspaces/{id}", s.IAMDeleteWorkspaceHandler)
	s.mux.HandleFunc("POST /users/workspaces/{id}/members", s.IAMInviteWorkspaceMemberHandler)
	s.mux.HandleFunc("DELETE /users/workspaces/{id}/members/{email}", s.IAMRemoveWorkspaceMemberHandler)
	s.mux.HandleFunc("POST /users/workspaces/{id}/members/{email}/role", s.IAMSetWorkspaceMemberRoleHandler)
	s.mux.HandleFunc("POST /users/workspaces/{id}/activate", s.IAMSwitchWorkspaceHandler)
	// IAM — OPA policies
	s.mux.HandleFunc("POST /users/policies", s.SavePolicyHandler)

	// Monitoring routes
	s.mux.HandleFunc("GET /monitoring", s.MonitoringHandler)
	s.mux.HandleFunc("GET /monitoring/alerts", s.AlertsListHandler)

	// Settings routes — Registry
	s.mux.HandleFunc("GET /settings/registry", s.RegistrySettingsHandler)
	s.mux.HandleFunc("POST /settings/registry", s.SaveRegistryHandler)
	s.mux.HandleFunc("POST /settings/registry/test", s.TestRegistryHandler)
	// Settings routes — Webhooks
	s.mux.HandleFunc("GET /settings/webhooks", s.WebhooksSettingsHandler)
	s.mux.HandleFunc("POST /settings/webhooks", s.CreateWebhookHandler)
	s.mux.HandleFunc("DELETE /settings/webhooks/{id}", s.DeleteWebhookHandler)
	s.mux.HandleFunc("POST /settings/webhooks/{id}/test", s.TestWebhookHandler)
	// Settings routes — SMTP
	s.mux.HandleFunc("GET /settings/smtp", s.SMTPSettingsHandler)
	s.mux.HandleFunc("POST /settings/smtp", s.SaveSMTPHandler)
	s.mux.HandleFunc("POST /settings/smtp/test", s.TestSMTPHandler)
	// Settings routes — Service Endpoints
	s.mux.HandleFunc("GET /settings/endpoints", s.EndpointsSettingsHandler)
	s.mux.HandleFunc("POST /settings/endpoints", s.SaveEndpointsHandler)
	s.mux.HandleFunc("POST /settings/endpoints/{name}/ingress", s.CreateEndpointIngressHandler)
	s.mux.HandleFunc("POST /settings/endpoints/{name}/test", s.TestEndpointHandler)
	// Settings routes — Cloud Providers
	s.mux.HandleFunc("GET /settings/cloud-providers", s.CloudProvidersSettingsHandler)
	s.mux.HandleFunc("POST /settings/cloud-providers", s.SaveCloudProvidersHandler)
	s.mux.HandleFunc("POST /settings/cloud-providers/{provider}/test", s.TestCloudProviderHandler)
	// Settings routes — Platform (DNS, TLS, Environment)
	s.mux.HandleFunc("GET /settings/platform", s.PlatformSettingsHandler)

	// Catalog visibility routes
	s.mux.HandleFunc("POST /catalog/{type}/{plan}/visibility", s.TogglePlanVisibilityHandler)
	s.mux.HandleFunc("POST /catalog/{type}/visibility", s.ToggleServiceVisibilityHandler)

	// Topology routes (accessed from catalog page)
	s.mux.HandleFunc("GET /topologies", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/catalog", http.StatusMovedPermanently)
	})
	s.mux.HandleFunc("GET /topologies/upload", s.TopologyUploadHandler)
	s.mux.HandleFunc("GET /topologies/{type}/{plan}", s.TopologyDetailHandler)
	s.mux.HandleFunc("POST /topologies/{type}/{plan}", s.SaveTopologyHandler)
	s.mux.HandleFunc("POST /topologies/{type}/{plan}/preview", s.TopologyPreviewHandler)
	s.mux.HandleFunc("POST /topologies/upload", s.UploadTopologyHandler)
	s.mux.HandleFunc("DELETE /topologies/{type}/{plan}", s.DeleteTopologyHandler)

	// Cluster routes
	s.mux.HandleFunc("GET /clusters", s.ClustersListHandler)
	s.mux.HandleFunc("GET /clusters/{id}", s.ClusterDetailHandler)
	s.mux.HandleFunc("POST /clusters", s.AddClusterHandler)
	s.mux.HandleFunc("POST /clusters/{id}/activate", s.SetActiveClusterHandler)
	s.mux.HandleFunc("DELETE /clusters/{id}", s.RemoveClusterHandler)

	// HTMX action routes
	s.mux.HandleFunc("POST /apps/{name}/scale", s.ScaleAppHandler)
	s.mux.HandleFunc("DELETE /apps/{name}", s.DeleteAppHandler)

	// JSON API routes
	s.mux.HandleFunc("GET /api/apps", s.APIListAppsHandler)
	s.mux.HandleFunc("GET /api/apps/{name}", s.APIGetAppHandler)
	s.mux.HandleFunc("GET /api/apps/{name}/logs/history", s.APILogHistoryHandler)
	s.mux.HandleFunc("GET /api/config", s.APIGetConfigHandler)
	s.mux.HandleFunc("POST /api/apps/{name}/scale", s.APIScaleAppHandler)
	s.mux.HandleFunc("DELETE /api/apps/{name}", s.APIDeleteAppHandler)
	s.mux.HandleFunc("GET /api/clusters", s.APIClustersListHandler)
	s.mux.HandleFunc("GET /api/clusters/{id}/health", s.APIClusterHealthHandler)
	s.mux.HandleFunc("GET /api/services", s.APIServicesListHandler)
	s.mux.HandleFunc("GET /api/services/{name}", s.APIServiceDetailHandler)
	s.mux.HandleFunc("GET /api/catalog", s.APICatalogHandler)
	s.mux.HandleFunc("GET /api/catalog/visible", s.APIVisibleCatalogHandler)
	s.mux.HandleFunc("GET /api/secrets", s.APISecretsListHandler)
	s.mux.HandleFunc("GET /api/secrets/{name}", s.APISecretDetailHandler)
	s.mux.HandleFunc("POST /api/secrets", s.APICreateSecretHandler)
	s.mux.HandleFunc("GET /api/topologies", s.APITopologiesListHandler)
	s.mux.HandleFunc("GET /api/topologies/{type}/{plan}", s.APITopologyDetailHandler)
	s.mux.HandleFunc("PUT /api/topologies/{type}/{plan}", s.APISaveTopologyHandler)
	s.mux.HandleFunc("GET /api/monitoring/alerts", s.APIAlertsHandler)
	s.mux.HandleFunc("GET /api/monitoring/health", s.APIMonitoringHealthHandler)

	// Beyla RED metrics API routes
	s.mux.HandleFunc("GET /api/apps/{name}/red-metrics", s.APIAppREDMetricsHandler)
	s.mux.HandleFunc("GET /api/apps/{name}/health", s.APIAppHealthHandler)
	s.mux.HandleFunc("GET /api/apps/{name}/observability", s.APIAppObservabilityHandler)

	// Settings API routes
	s.mux.HandleFunc("GET /api/settings", s.APIGetSettingsHandler)
	s.mux.HandleFunc("PUT /api/settings/registry", s.APISaveRegistryHandler)
	s.mux.HandleFunc("GET /api/settings/webhooks", s.APIGetWebhooksHandler)
	s.mux.HandleFunc("POST /api/settings/webhooks", s.APICreateWebhookHandler)
	s.mux.HandleFunc("DELETE /api/settings/webhooks/{id}", s.APIDeleteWebhookHandler)
	s.mux.HandleFunc("PUT /api/settings/smtp", s.APISaveSMTPHandler)
	s.mux.HandleFunc("GET /api/settings/endpoints", s.APIGetEndpointsHandler)
	s.mux.HandleFunc("PUT /api/settings/endpoints", s.APISaveEndpointsHandler)
	s.mux.HandleFunc("GET /api/settings/cloud-providers", s.APIGetSettingsHandler) // reuse existing
	s.mux.HandleFunc("PUT /api/settings/cloud-providers", s.SaveCloudProvidersHandler)

	// Workspace routes
	s.mux.HandleFunc("GET /workspaces", s.WorkspacesPageHandler)
	s.mux.HandleFunc("POST /workspaces", s.CreateWorkspaceHandler)
	s.mux.HandleFunc("DELETE /workspaces/{id}", s.DeleteWorkspaceHandler)
	s.mux.HandleFunc("POST /workspaces/{id}/members", s.InviteWorkspaceMemberHandler)
	s.mux.HandleFunc("DELETE /workspaces/{id}/members/{email}", s.RemoveWorkspaceMemberHandler)
	s.mux.HandleFunc("POST /workspaces/{id}/members/{email}/role", s.SetWorkspaceMemberRoleHandler)
	s.mux.HandleFunc("POST /workspaces/{id}/activate", s.SwitchWorkspaceHandler)

	// Workspace API routes
	s.mux.HandleFunc("GET /api/workspaces", s.APIListWorkspacesHandler)
	s.mux.HandleFunc("GET /api/workspaces/{id}", s.APIGetWorkspaceHandler)
	s.mux.HandleFunc("POST /api/workspaces", s.APICreateWorkspaceHandler)
	s.mux.HandleFunc("DELETE /api/workspaces/{id}", s.APIDeleteWorkspaceHandler)
	s.mux.HandleFunc("GET /api/workspaces/{id}/members", s.APIListWorkspaceMembersHandler)
	s.mux.HandleFunc("POST /api/workspaces/{id}/members", s.APIAddWorkspaceMemberHandler)
	s.mux.HandleFunc("DELETE /api/workspaces/{id}/members/{email}", s.APIRemoveWorkspaceMemberHandler)
	s.mux.HandleFunc("GET /api/workspaces/{id}/orgs", s.APIListWorkspaceOrgsHandler)

	// Org API routes
	s.mux.HandleFunc("GET /api/orgs", s.APIListOrgsHandler)
	s.mux.HandleFunc("GET /api/orgs/{id}", s.APIGetOrgHandler)
	s.mux.HandleFunc("POST /api/orgs", s.APICreateOrgHandler)
	s.mux.HandleFunc("DELETE /api/orgs/{id}", s.APIDeleteOrgHandler)
	s.mux.HandleFunc("GET /api/orgs/{id}/members", s.APIListMembersHandler)
	s.mux.HandleFunc("POST /api/orgs/{id}/members", s.APIAddMemberHandler)
	s.mux.HandleFunc("DELETE /api/orgs/{id}/members/{email}", s.APIRemoveMemberHandler)

	// IAM API routes
	s.mux.HandleFunc("GET /api/audit", s.APIAuditLogHandler)
	s.mux.HandleFunc("GET /api/users", s.APIKeycloakUsersHandler)
	s.mux.HandleFunc("POST /api/users", s.APICreateUserHandler)
	s.mux.HandleFunc("DELETE /api/users/{id}", s.APIDeleteUserHandler)
	s.mux.HandleFunc("POST /api/users/{id}/toggle", s.APIToggleUserHandler)
	s.mux.HandleFunc("POST /api/users/{id}/roles", s.APIAssignRoleHandler)
	s.mux.HandleFunc("DELETE /api/users/{id}/roles", s.APIRemoveRoleHandler)
	s.mux.HandleFunc("POST /api/users/{id}/password", s.APIResetPasswordHandler)
	s.mux.HandleFunc("GET /api/policies", s.APIPoliciesHandler)
	s.mux.HandleFunc("PUT /api/policies", s.APISavePolicyHandler)

	// SCIM v2 routes
	s.mux.HandleFunc("GET /scim/v2/Users", s.SCIMListUsersHandler)
	s.mux.HandleFunc("POST /scim/v2/Users", s.SCIMCreateUserHandler)
	s.mux.HandleFunc("GET /scim/v2/Users/{id}", s.SCIMGetUserHandler)
	s.mux.HandleFunc("PUT /scim/v2/Users/{id}", s.SCIMUpdateUserHandler)
	s.mux.HandleFunc("PATCH /scim/v2/Users/{id}", s.SCIMPatchUserHandler)
	s.mux.HandleFunc("DELETE /scim/v2/Users/{id}", s.SCIMDeleteUserHandler)
	s.mux.HandleFunc("GET /scim/v2/ServiceProviderConfig", s.SCIMServiceProviderConfigHandler)
	s.mux.HandleFunc("GET /scim/v2/ResourceTypes", s.SCIMResourceTypesHandler)
	s.mux.HandleFunc("GET /scim/v2/Schemas", s.SCIMSchemasHandler)
}

func (s *Server) ListenAndServe(addr string) error {
	var handler http.Handler = s.mux

	// OPA authorization (needs user from context → must wrap AFTER InjectUser)
	if s.opa != nil {
		handler = auth.OPAMiddleware(s.opa, s.sessions, s.orgStore_, s.wsStore, s.auditLog)(handler)
	}

	// InjectUser populates user in context (wraps OUTSIDE OPA so it runs first)
	if s.authEnabled() {
		handler = auth.InjectUser(s.sessions)(handler)
	}

	handler = monitoring.InstrumentHandler(s.metrics, handler)

	if s.tlsCertFile != "" && s.tlsKeyFile != "" {
		return http.ListenAndServeTLS(addr, s.tlsCertFile, s.tlsKeyFile, handler)
	}
	return http.ListenAndServe(addr, handler)
}

// Auth route handlers — delegate to OIDC authenticator or show login page

func (s *Server) LoginPageHandler(w http.ResponseWriter, r *http.Request) {
	if !s.authEnabled() {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	s.templates.Render(w, "login.html", nil)
}

func (s *Server) AuthLoginHandler(w http.ResponseWriter, r *http.Request) {
	if !s.authEnabled() {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	s.oidcAuth.LoginHandler(w, r)
}

func (s *Server) AuthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	if !s.authEnabled() {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	s.oidcAuth.CallbackHandler(w, r)
}

func (s *Server) AuthLogoutHandler(w http.ResponseWriter, r *http.Request) {
	if !s.authEnabled() {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	s.oidcAuth.LogoutHandler(w, r)
}
