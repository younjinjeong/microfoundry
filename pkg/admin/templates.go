package admin

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

//go:embed static/templates/*.html static/templates/partials/*.html static/templates/tabs/*.html
var templateFiles embed.FS

//go:embed static/css/*
var cssFiles embed.FS

func staticFS() http.FileSystem {
	sub, _ := fs.Sub(cssFiles, "static")
	return http.FS(sub)
}

type TemplateRenderer struct {
	base     *template.Template
	pages    map[string]*template.Template
	partials map[string]*template.Template
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"stateColor": func(state string) string {
			switch strings.ToLower(state) {
			case "started", "running", "connected", "available":
				return "green"
			case "stopped", "down":
				return "red"
			case "starting", "staging", "deploying", "uploading", "creating", "deleting":
				return "yellow"
			case "crashed", "failed", "error":
				return "red"
			default:
				return "gray"
			}
		},
		"stateBg": func(state string) string {
			switch strings.ToLower(state) {
			case "started", "running", "connected", "available":
				return "bg-green-100 text-green-800"
			case "stopped", "down":
				return "bg-red-100 text-red-800"
			case "starting", "staging", "deploying", "uploading", "creating", "deleting":
				return "bg-yellow-100 text-yellow-800"
			case "crashed", "failed", "error":
				return "bg-red-100 text-red-800"
			case "disconnected":
				return "bg-gray-100 text-gray-800"
			default:
				return "bg-gray-100 text-gray-800"
			}
		},
		"lifecycleBadge": func(lcType string) string {
			switch strings.ToLower(lcType) {
			case "docker":
				return "bg-blue-100 text-blue-800"
			case "buildpack":
				return "bg-green-100 text-green-800"
			case "cnb":
				return "bg-purple-100 text-purple-800"
			default:
				return "bg-gray-100 text-gray-800"
			}
		},
		"timeAgo": func(t time.Time) string {
			if t.IsZero() {
				return "-"
			}
			d := time.Since(t)
			switch {
			case d < time.Minute:
				return fmt.Sprintf("%ds ago", int(d.Seconds()))
			case d < time.Hour:
				return fmt.Sprintf("%dm ago", int(d.Minutes()))
			case d < 24*time.Hour:
				return fmt.Sprintf("%dh ago", int(d.Hours()))
			default:
				return fmt.Sprintf("%dd ago", int(d.Hours()/24))
			}
		},
		"join": strings.Join,
		"memFmt": func(mb int) string {
			if mb >= 1024 {
				return fmt.Sprintf("%.1fG", float64(mb)/1024)
			}
			return fmt.Sprintf("%dM", mb)
		},
		"isSensitive": func(key string) bool {
			upper := strings.ToUpper(key)
			for _, s := range []string{"PASSWORD", "SECRET", "KEY", "TOKEN", "CREDENTIALS"} {
				if strings.Contains(upper, s) {
					return true
				}
			}
			return false
		},
		"dict": func(values ...any) map[string]any {
			if len(values)%2 != 0 {
				return nil
			}
			d := make(map[string]any, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					continue
				}
				d[key] = values[i+1]
			}
			return d
		},
	}
}

func NewTemplateRenderer() *TemplateRenderer {
	funcs := templateFuncs()

	// Parse base templates: layout, partials, tabs, and shared inline partials
	// (app_row, app_instances, scale_form don't define "content" so no conflict)
	base := template.Must(
		template.New("").Funcs(funcs).ParseFS(
			templateFiles,
			"static/templates/layout.html",
			"static/templates/app_row.html",
			"static/templates/app_instances.html",
			"static/templates/scale_form.html",
			"static/templates/partials/*.html",
			"static/templates/tabs/*.html",
		),
	)

	// Parse each full-page template by cloning base and adding the page.
	// This ensures each page's {{define "content"}} doesn't conflict.
	pageFiles := []string{
		"dashboard.html",
		"apps.html",
		"app_detail.html",
		"config.html",
		"services.html",
		"secrets.html",
		"users.html",
		"clusters.html",
		"cluster_detail.html",
		"service_detail.html",
		"marketplace.html",
		"secret_detail.html",
		"secret_create.html",
	}

	pages := make(map[string]*template.Template, len(pageFiles)+3)
	for _, name := range pageFiles {
		clone := template.Must(base.Clone())
		pages[name] = template.Must(clone.ParseFS(
			templateFiles,
			"static/templates/"+name,
		))
	}
	// Non-layout partials can also be rendered as pages (for HTMX row updates)
	pages["app_row.html"] = base
	pages["app_instances.html"] = base
	pages["scale_form.html"] = base

	// Partials: tab templates + inline partials are all in base
	partials := map[string]*template.Template{
		"app_row.html":       base,
		"app_instances.html": base,
		"scale_form.html":    base,
		"tab_overview.html":  base,
		"tab_instances.html": base,
		"tab_config.html":    base,
		"tab_services.html":  base,
		"tab_routes.html":    base,
		"tab_logs.html":      base,
	}

	return &TemplateRenderer{base: base, pages: pages, partials: partials}
}

func (tr *TemplateRenderer) Render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, ok := tr.pages[name]
	if !ok {
		http.Error(w, fmt.Sprintf("template not found: %s", name), http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, fmt.Sprintf("template error: %v", err), http.StatusInternalServerError)
	}
}

func (tr *TemplateRenderer) RenderPartial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, ok := tr.partials[name]
	if !ok {
		http.Error(w, fmt.Sprintf("partial not found: %s", name), http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, fmt.Sprintf("template error: %v", err), http.StatusInternalServerError)
	}
}
