package admin

import (
	"net/http"
	"regexp"

	"github.com/younjinjeong/microfoundry/pkg/config"
)

// validClusterID restricts cluster names to safe alphanumeric characters.
var validClusterID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,62}$`)

// ClustersListHandler renders the clusters page with all registered clusters.
func (s *Server) ClustersListHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clusters := s.clientManager.ListClusters(ctx)

	data := s.pageData("Clusters", "clusters")
	data.Content = map[string]any{
		"Clusters":      clusters,
		"ActiveCluster": s.clientManager.GetActive(),
	}
	s.templates.Render(w, "clusters.html", data)
}

// ClusterDetailHandler renders the detail page for a single cluster.
func (s *Server) ClusterDetailHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	detail, err := s.clientManager.GetClusterDetail(ctx, id)
	if err != nil {
		http.Error(w, "Cluster not found: "+err.Error(), http.StatusNotFound)
		return
	}

	data := s.pageData(detail.Name, "clusters")
	data.Content = detail
	s.templates.Render(w, "cluster_detail.html", data)
}

// SetActiveClusterHandler sets the active cluster and redirects back to the clusters page.
func (s *Server) SetActiveClusterHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.clientManager.SetActive(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "mf-cluster",
		Value:    id,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60, // 30 days
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/clusters")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/clusters", http.StatusSeeOther)
}

// AddClusterHandler adds a new cluster from form data.
func (s *Server) AddClusterHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	id := r.FormValue("name")
	if id == "" {
		http.Error(w, "Cluster name is required", http.StatusBadRequest)
		return
	}
	if !validClusterID.MatchString(id) {
		http.Error(w, "Invalid cluster name: must be alphanumeric with .-_: allowed, max 63 chars", http.StatusBadRequest)
		return
	}

	provider := r.FormValue("provider")
	if provider == "" {
		provider = config.DetectProvider(r.FormValue("context"))
	}

	cfg := config.ClusterConfig{
		Name:         id,
		Provider:     provider,
		Context:      r.FormValue("context"),
		Namespace:    r.FormValue("namespace"),
		Domain:       r.FormValue("domain"),
		Region:       r.FormValue("region"),
		IngressClass: r.FormValue("ingress_class"),
		Enabled:      true,
	}

	if err := s.clientManager.AddCluster(id, cfg); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/clusters")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/clusters", http.StatusSeeOther)
}

// RemoveClusterHandler removes a cluster by ID.
func (s *Server) RemoveClusterHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.clientManager.RemoveCluster(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// APIClustersListHandler returns all clusters as JSON.
func (s *Server) APIClustersListHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clusters := s.clientManager.ListClusters(ctx)

	writeJSON(w, http.StatusOK, map[string]any{
		"clusters": clusters,
		"active":   s.clientManager.GetActive(),
	})
}

// APIClusterHealthHandler returns the health status of a single cluster as JSON.
func (s *Server) APIClusterHealthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	status, err := s.clientManager.CheckHealth(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"cluster": id,
			"status":  status,
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"cluster": id,
		"status":  status,
	})
}
