package admin

import (
	"net/http"

	"github.com/younjinjeong/microfoundry/pkg/config"
	"github.com/younjinjeong/microfoundry/pkg/k8s"
)

type Server struct {
	clientManager *k8s.ClientManager
	config        *config.Config
	version       string
	templates     *TemplateRenderer
	mux           *http.ServeMux
}

func NewServer(clientManager *k8s.ClientManager, cfg *config.Config, version string) *Server {
	s := &Server{
		clientManager: clientManager,
		config:        cfg,
		version:       version,
		templates:     NewTemplateRenderer(),
		mux:           http.NewServeMux(),
	}
	s.registerRoutes()
	return s
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

	// Page routes
	s.mux.HandleFunc("GET /{$}", s.DashboardHandler)
	s.mux.HandleFunc("GET /apps", s.AppsListHandler)
	s.mux.HandleFunc("GET /apps/{name}", s.AppDetailHandler)
	s.mux.HandleFunc("GET /apps/{name}/tab/{tab}", s.AppTabHandler)
	s.mux.HandleFunc("GET /apps/{name}/instances", s.AppInstancesHandler)
	s.mux.HandleFunc("GET /apps/{name}/logs/stream", s.LogStreamHandler)
	s.mux.HandleFunc("GET /config", s.ConfigHandler)
	s.mux.HandleFunc("GET /services", s.ServicesListHandler)
	s.mux.HandleFunc("GET /services/{name}", s.ServiceDetailHandler)
	s.mux.HandleFunc("GET /marketplace", s.MarketplaceHandler)
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
	s.mux.HandleFunc("GET /api/config", s.APIGetConfigHandler)
	s.mux.HandleFunc("POST /api/apps/{name}/scale", s.APIScaleAppHandler)
	s.mux.HandleFunc("DELETE /api/apps/{name}", s.APIDeleteAppHandler)
	s.mux.HandleFunc("GET /api/clusters", s.APIClustersListHandler)
	s.mux.HandleFunc("GET /api/clusters/{id}/health", s.APIClusterHealthHandler)
	s.mux.HandleFunc("GET /api/services", s.APIServicesListHandler)
	s.mux.HandleFunc("GET /api/services/{name}", s.APIServiceDetailHandler)
	s.mux.HandleFunc("GET /api/marketplace", s.APIMarketplaceHandler)
	s.mux.HandleFunc("GET /api/secrets", s.APISecretsListHandler)
	s.mux.HandleFunc("GET /api/secrets/{name}", s.APISecretDetailHandler)
	s.mux.HandleFunc("POST /api/secrets", s.APICreateSecretHandler)
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}
