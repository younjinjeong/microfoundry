package admin

import (
	"net/http"
	"strconv"
)

// PageData is the base data passed to every full-page template.
type PageData struct {
	Title         string
	Version       string
	Active        string
	ActiveCluster string
	Content       any
}

func (s *Server) pageData(title, active string) PageData {
	return PageData{
		Title:         title,
		Version:       s.version,
		Active:        active,
		ActiveCluster: s.clientManager.GetActive(),
	}
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

	data := s.pageData(name, "apps")
	data.Content = map[string]any{
		"Detail": detail,
		"Tab":    tab,
	}
	s.templates.Render(w, "app_detail.html", data)
}

// validTabs is the allowlist of valid tab names for the app detail view.
var validTabs = map[string]bool{
	"overview": true, "instances": true, "config": true,
	"services": true, "routes": true, "logs": true,
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

func (s *Server) UsersHandler(w http.ResponseWriter, r *http.Request) {
	data := s.pageData("Users & Organizations", "users")
	s.templates.Render(w, "users.html", data)
}
