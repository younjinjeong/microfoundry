package admin

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/younjinjeong/microfoundry/pkg/config"
	"github.com/younjinjeong/microfoundry/pkg/k8s"
	"github.com/younjinjeong/microfoundry/pkg/monitoring"
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
}

func NewServer(clientManager *k8s.ClientManager, cfg *config.Config, version string) *Server {
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
	s.registerRoutes()
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

func (s *Server) registerRoutes() {
	// Static CSS files
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(staticFS())))

	// Prometheus metrics endpoint
	s.mux.Handle("GET /metrics", monitoring.Handler())

	// Page routes
	s.mux.HandleFunc("GET /{$}", s.DashboardHandler)
	s.mux.HandleFunc("GET /apps", s.AppsListHandler)
	s.mux.HandleFunc("GET /apps/{name}", s.AppDetailHandler)
	s.mux.HandleFunc("GET /apps/{name}/tab/{tab}", s.AppTabHandler)
	s.mux.HandleFunc("GET /apps/{name}/instances", s.AppInstancesHandler)
	s.mux.HandleFunc("GET /apps/{name}/logs/stream", s.LogStreamHandler)
	s.mux.HandleFunc("GET /apps/{name}/logs/history", s.LogHistoryHandler)
	s.mux.HandleFunc("GET /config", s.ConfigHandler)
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
	s.mux.HandleFunc("GET /users", s.UsersHandler)

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
}

func (s *Server) ListenAndServe(addr string) error {
	handler := monitoring.InstrumentHandler(s.metrics, s.mux)
	return http.ListenAndServe(addr, handler)
}
