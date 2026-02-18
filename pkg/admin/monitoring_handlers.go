package admin

import (
	"context"
	"net/http"
	"sync"
	"time"
)

const (
	dashboardOverviewUID = "mf-overview"
	dashboardAppUID      = "mf-app-detail"
	dashboardClusterUID  = "mf-cluster"
)

// ComponentStatus represents the health of a monitoring component.
type ComponentStatus struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Connected bool   `json:"connected"`
	Error     string `json:"error,omitempty"`
}

// MonitoringHandler renders the Metrics & Alerts page with embedded Grafana dashboards.
func (s *Server) MonitoringHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Run health checks in parallel
	components := s.checkMonitoringHealth(ctx)

	// Build embeddable Grafana iframe URLs
	overviewURL := s.grafana.DashboardURL(dashboardOverviewUID, nil)
	clusterURL := s.grafana.DashboardURL(dashboardClusterUID, nil)
	fullOverviewURL := s.grafana.FullDashboardURL(dashboardOverviewUID, nil)
	fullClusterURL := s.grafana.FullDashboardURL(dashboardClusterUID, nil)

	// Query Alertmanager for active alerts
	alerts, _ := s.alertmanager.ListAlerts(ctx)

	firingCount := 0
	for _, a := range alerts {
		if a.Status == "firing" {
			firingCount++
		}
	}

	allConnected := true
	for _, c := range components {
		if !c.Connected {
			allConnected = false
			break
		}
	}

	data := s.pageData("Metrics & Alerts", "monitoring")
	data.Content = map[string]any{
		"OverviewIframeURL": overviewURL,
		"ClusterIframeURL":  clusterURL,
		"FullOverviewURL":   fullOverviewURL,
		"FullClusterURL":    fullClusterURL,
		"GrafanaBaseURL":    s.grafana.BaseURL,
		"Alerts":            alerts,
		"FiringCount":       firingCount,
		"Components":        components,
		"AllConnected":      allConnected,
	}
	s.templates.Render(w, "monitoring.html", data)
}

// checkMonitoringHealth runs health checks on all monitoring components in parallel.
func (s *Server) checkMonitoringHealth(ctx context.Context) []ComponentStatus {
	type result struct {
		idx    int
		status ComponentStatus
	}

	checks := []struct {
		name string
		url  string
		fn   func(ctx context.Context) error
	}{
		{"Grafana", s.grafana.BaseURL, s.grafana.HealthCheck},
		{"Prometheus", s.prometheus.BaseURL, s.prometheus.HealthCheck},
		{"Alertmanager", s.alertmanager.BaseURL, s.alertmanager.HealthCheck},
		{"Loki", s.loki.BaseURL, s.loki.HealthCheck},
	}

	results := make([]ComponentStatus, len(checks))
	var wg sync.WaitGroup

	for i, check := range checks {
		wg.Add(1)
		go func(idx int, name, url string, fn func(context.Context) error) {
			defer wg.Done()
			cs := ComponentStatus{Name: name, URL: url, Connected: true}
			if err := fn(ctx); err != nil {
				cs.Connected = false
				cs.Error = err.Error()
			}
			results[idx] = cs
		}(i, check.name, check.url, check.fn)
	}

	wg.Wait()
	return results
}

// AlertsListHandler returns the alerts partial for HTMX polling.
func (s *Server) AlertsListHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	alerts, _ := s.alertmanager.ListAlerts(ctx)

	s.templates.RenderPartial(w, "alerts_list.html", map[string]any{
		"Alerts": alerts,
	})
}

// LogHistoryHandler returns historical logs from Loki as an HTML partial.
func (s *Server) LogHistoryHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	duration := r.URL.Query().Get("duration")
	if duration == "" {
		duration = "1h"
	}
	d, _ := time.ParseDuration(duration)
	if d == 0 {
		d = time.Hour
	}
	end := time.Now()
	start := end.Add(-d)

	entries, err := s.loki.QueryLogs(ctx, client.Namespace, name, start, end, 500)
	if err != nil {
		http.Error(w, "Querying logs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.templates.RenderPartial(w, "log_history.html", map[string]any{
		"Entries":  entries,
		"AppName":  name,
		"Duration": duration,
	})
}

// APIMonitoringHealthHandler returns the connectivity status of all monitoring components.
func (s *Server) APIMonitoringHealthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	components := s.checkMonitoringHealth(ctx)
	allOK := true
	for _, c := range components {
		if !c.Connected {
			allOK = false
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"healthy":    allOK,
		"components": components,
	})
}

// APIAlertsHandler returns alerts as JSON.
func (s *Server) APIAlertsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	alerts, err := s.alertmanager.ListAlerts(ctx)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, alerts)
}

// APILogHistoryHandler returns historical logs from Loki as JSON.
func (s *Server) APILogHistoryHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")

	client, err := s.getClient(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	duration := r.URL.Query().Get("duration")
	if duration == "" {
		duration = "1h"
	}
	d, _ := time.ParseDuration(duration)
	if d == 0 {
		d = time.Hour
	}
	end := time.Now()
	start := end.Add(-d)

	entries, err := s.loki.QueryLogs(ctx, client.Namespace, name, start, end, 500)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, entries)
}
