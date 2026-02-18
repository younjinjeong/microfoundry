package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/younjinjeong/microfoundry/pkg/auth"
	"github.com/younjinjeong/microfoundry/pkg/monitoring"
)

// PageData is the base data passed to every full-page template.
type PageData struct {
	Title           string
	Version         string
	Active          string
	ActiveCluster   string
	Content         any
	User            *auth.UserSession // nil when auth is disabled or not logged in
	AuthEnabled     bool
	TooltipsEnabled bool
}

func (s *Server) pageData(title, active string) PageData {
	return PageData{
		Title:           title,
		Version:         s.version,
		Active:          active,
		ActiveCluster:   s.clientManager.GetActive(),
		AuthEnabled:     s.authEnabled(),
		TooltipsEnabled: s.config.UI.Tooltips,
	}
}

func (s *Server) pageDataWithUser(r *http.Request, title, active string) PageData {
	pd := s.pageData(title, active)
	pd.User = auth.UserFromContext(r.Context())
	return pd
}

func (s *Server) DashboardHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	apps, _ := client.ListApps(ctx)

	data := s.pageData("Dashboard", "dashboard")
	data.Content = map[string]any{
		"AppCount":  len(apps),
		"Domain":    client.Domain,
		"Namespace": client.Namespace,
		"Context":   s.config.Kubernetes.Active,
		"GitHub":    s.config.GitHub.Owner + "/" + s.config.GitHub.Repo,
	}
	s.templates.Render(w, "dashboard.html", data)
}

func (s *Server) AppsListHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	items, err := client.ListAppItems(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Apply state filter if provided
	filter := r.URL.Query().Get("state")
	if filter != "" && filter != "all" {
		var filtered []any
		for _, item := range items {
			if item.State == filter {
				filtered = append(filtered, item)
			}
		}
		data := s.pageData("Applications", "apps")
		data.Content = map[string]any{
			"Apps":   filtered,
			"Filter": filter,
		}
		s.templates.Render(w, "apps.html", data)
		return
	}

	data := s.pageData("Applications", "apps")
	data.Content = map[string]any{
		"Apps":   items,
		"Filter": "all",
	}
	s.templates.Render(w, "apps.html", data)
}

func (s *Server) AppDetailHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	detail, err := client.GetAppDetail(ctx, name)
	if err != nil {
		http.Error(w, "App not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Determine active tab from query parameter
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "overview"
	}

	content := map[string]any{
		"Detail": detail,
		"Tab":    tab,
	}

	// Enrich context for tabs that need additional data on initial page load
	if tab == "logs" {
		var red *monitoring.REDMetrics
		if s.config.Monitoring.BeylaEnabled {
			red, _ = s.prometheus.GetAppREDMetrics(ctx, name)
		}
		content["LogsData"] = map[string]any{
			"Name":         name,
			"RED":          red,
			"BeylaEnabled": s.config.Monitoring.BeylaEnabled,
		}
	}
	if tab == "performance" {
		red, _ := s.prometheus.GetAppREDMetrics(ctx, name)
		namespace := "microfoundry"
		if client != nil {
			namespace = client.Namespace
		}
		end := time.Now()
		start := end.Add(-15 * time.Minute)
		recentLogs, _ := s.loki.QueryLogs(ctx, namespace, name, start, end, 50)

		appParams := map[string]string{"var-app": name}
		content["PerfData"] = map[string]any{
			"Name":                   name,
			"RED":                    red,
			"BeylaEnabled":           s.config.Monitoring.BeylaEnabled,
			"RecentLogs":             recentLogs,
			"GrafanaAppURL":          s.grafana.FullDashboardURL(dashboardPerfUID, appParams),
			"GrafanaReqRatePanelURL": s.grafana.PanelURL(dashboardPerfUID, 16, appParams),
			"GrafanaLatencyPanelURL": s.grafana.PanelURL(dashboardPerfUID, 17, appParams),
			"GrafanaStatusPanelURL":  s.grafana.PanelURL(dashboardPerfUID, 18, appParams),
			"GrafanaHeatmapPanelURL": s.grafana.PanelURL(dashboardPerfUID, 19, appParams),
		}
	}
	if tab == "metrics" {
		appParams := map[string]string{"var-app": name}
		content["MetricsData"] = map[string]any{
			"Name":                   detail.Name,
			"GrafanaAppURL":          s.grafana.FullDashboardURL(dashboardAppUID, appParams),
			"GrafanaCPUPanelURL":     s.grafana.PanelURL(dashboardAppUID, 1, appParams),
			"GrafanaMemPanelURL":     s.grafana.PanelURL(dashboardAppUID, 2, appParams),
			"GrafanaNetPanelURL":     s.grafana.PanelURL(dashboardAppUID, 3, appParams),
			"GrafanaRestartPanelURL": s.grafana.PanelURL(dashboardAppUID, 4, appParams),
			"GrafanaLogPanelURL":     s.grafana.PanelURL(dashboardAppUID, 5, appParams),
		}
	}

	data := s.pageData(name, "apps")
	data.Content = content
	s.templates.Render(w, "app_detail.html", data)
}

// validTabs is the allowlist of valid tab names for the app detail view.
var validTabs = map[string]bool{
	"overview": true, "instances": true, "config": true,
	"services": true, "routes": true, "logs": true,
	"metrics": true, "performance": true,
}

// AppTabHandler serves individual tab content as HTMX partials.
func (s *Server) AppTabHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")
	tab := r.PathValue("tab")

	if !validTabs[tab] {
		http.Error(w, "invalid tab", http.StatusBadRequest)
		return
	}

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	detail, err := client.GetAppDetail(ctx, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Performance tab uses dedicated handler with RED metrics + logs
	if tab == "performance" {
		s.AppPerformanceTabHandler(w, r, name)
		return
	}

	// Logs tab uses dedicated handler with RED metrics summary
	if tab == "logs" {
		s.AppLogsTabHandler(w, r, name)
		return
	}


	// Metrics tab needs Grafana panel URLs
	if tab == "metrics" {
		appParams := map[string]string{"var-app": name}
		metricsData := map[string]any{
			"Name":                   detail.Name,
			"GrafanaAppURL":          s.grafana.FullDashboardURL(dashboardAppUID, appParams),
			"GrafanaCPUPanelURL":     s.grafana.PanelURL(dashboardAppUID, 1, appParams),
			"GrafanaMemPanelURL":     s.grafana.PanelURL(dashboardAppUID, 2, appParams),
			"GrafanaNetPanelURL":     s.grafana.PanelURL(dashboardAppUID, 3, appParams),
			"GrafanaRestartPanelURL": s.grafana.PanelURL(dashboardAppUID, 4, appParams),
			"GrafanaLogPanelURL":     s.grafana.PanelURL(dashboardAppUID, 5, appParams),
		}
		s.templates.RenderPartial(w, "tab_metrics.html", metricsData)
		return
	}

	templateName := "tab_" + tab + ".html"
	s.templates.RenderPartial(w, templateName, detail)
}

func (s *Server) AppInstancesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	status, err := client.GetAppStatus(ctx, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.templates.RenderPartial(w, "app_instances.html", map[string]any{
		"Instances":    status.Instances,
		"RunningCount": status.RunningCount,
		"TotalCount":   status.TotalCount,
	})
}

func (s *Server) ScaleAppHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")

	instancesStr := r.FormValue("instances")
	instances, err := strconv.Atoi(instancesStr)
	if err != nil || instances < 0 || instances > 20 {
		http.Error(w, "instances must be between 0 and 20", http.StatusBadRequest)
		return
	}

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	if err := client.ScaleApp(ctx, name, instances); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.metrics.ScaleEvents.WithLabelValues(s.clientManager.GetActive(), name).Inc()

	// Return updated app row via ListAppItems
	items, _ := client.ListAppItems(ctx)
	for _, item := range items {
		if item.Name == name {
			s.templates.RenderPartial(w, "app_row.html", item)
			return
		}
	}

	// Fallback
	http.Redirect(w, r, "/apps", http.StatusSeeOther)
}

func (s *Server) DeleteAppHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	if err := client.DeleteApp(ctx, name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) ConfigHandler(w http.ResponseWriter, r *http.Request) {
	client, _ := s.getClient(r)

	domain := ""
	namespace := ""
	if client != nil {
		domain = client.Domain
		namespace = client.Namespace
	}

	data := s.pageData("Configuration", "config")
	data.Content = map[string]any{
		"Domain":    domain,
		"Namespace": namespace,
		"Context":   s.config.Kubernetes.Active,
		"GitHub":    s.config.GitHub.Owner + "/" + s.config.GitHub.Repo,
		"Owner":     s.config.GitHub.Owner,
		"Repo":      s.config.GitHub.Repo,
	}
	s.templates.Render(w, "config.html", data)
}

// UsersHandler is now replaced by OrgsPageHandler in org_handlers.go
