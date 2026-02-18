package manifest

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/younjinjeong/microfoundry/pkg/models"
	"gopkg.in/yaml.v3"
)

// Manifest represents a CF-compatible manifest.yml file.
type Manifest struct {
	Applications []ManifestApp `yaml:"applications"`
}

type ManifestApp struct {
	Name        string            `yaml:"name"`
	Path        string            `yaml:"path,omitempty"`
	Memory      string            `yaml:"memory,omitempty"`
	DiskQuota   string            `yaml:"disk-quota,omitempty"`
	Instances   *int              `yaml:"instances,omitempty"`
	Buildpacks  []string          `yaml:"buildpacks,omitempty"`
	Stack       string            `yaml:"stack,omitempty"`
	Command     string            `yaml:"command,omitempty"`
	Env         map[string]string `yaml:"env,omitempty"`
	Routes      []ManifestRoute   `yaml:"routes,omitempty"`
	NoRoute     bool              `yaml:"no-route,omitempty"`
	RandomRoute bool              `yaml:"random-route,omitempty"`
	Docker      *ManifestDocker   `yaml:"docker,omitempty"`
	Services    []string          `yaml:"services,omitempty"`

	HealthCheckType     string `yaml:"health-check-type,omitempty"`
	HealthCheckEndpoint string `yaml:"health-check-http-endpoint,omitempty"`
	HealthCheckTimeout  int    `yaml:"timeout,omitempty"`
}

type ManifestRoute struct {
	Route    string `yaml:"route"`
	Protocol string `yaml:"protocol,omitempty"` // "websocket", "grpc", or "" (HTTP default)
}

type ManifestDocker struct {
	Image    string `yaml:"image"`
	Username string `yaml:"username,omitempty"`
}

// ReadFile parses a manifest.yml file.
func ReadFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	return Parse(data)
}

// Parse parses manifest YAML bytes.
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	if len(m.Applications) == 0 {
		return nil, fmt.Errorf("manifest must contain at least one application")
	}
	return &m, nil
}

// ToApp converts a ManifestApp to a models.App with defaults applied.
func (ma ManifestApp) ToApp(domain string) models.App {
	app := models.AppDefaults(ma.Name)

	if ma.Memory != "" {
		app.MemoryMB = ParseMemoryMB(ma.Memory)
	}
	if ma.DiskQuota != "" {
		app.DiskMB = ParseMemoryMB(ma.DiskQuota)
	}
	if ma.Instances != nil {
		app.Instances = *ma.Instances
	}
	if ma.Command != "" {
		app.Command = ma.Command
	}
	if len(ma.Buildpacks) > 0 {
		app.Buildpacks = ma.Buildpacks
		app.LifecycleType = models.LifecycleBuildpack
	}
	if ma.Docker != nil && ma.Docker.Image != "" {
		app.DockerImage = ma.Docker.Image
		app.LifecycleType = models.LifecycleDocker
	}
	if ma.Env != nil {
		app.Env = ma.Env
	}
	if ma.HealthCheckType != "" {
		app.HealthCheck.Type = models.HealthCheckType(ma.HealthCheckType)
	}
	if ma.HealthCheckEndpoint != "" {
		app.HealthCheck.Endpoint = ma.HealthCheckEndpoint
	}
	if ma.HealthCheckTimeout > 0 {
		app.HealthCheck.Timeout = ma.HealthCheckTimeout
	}

	return app
}

// ParseRoutes extracts Route models from manifest route entries.
func (ma ManifestApp) ParseRoutes(domain string) []models.Route {
	if ma.NoRoute {
		return nil
	}
	if len(ma.Routes) > 0 {
		var routes []models.Route
		for _, mr := range ma.Routes {
			r := parseRouteString(mr.Route)
			r.Protocol = normalizeProtocol(mr.Protocol)
			routes = append(routes, r)
		}
		return routes
	}
	// Default route: <app-name>.<domain>
	return []models.Route{
		{Host: ma.Name, Domain: domain},
	}
}

// parseRouteString parses "host.domain/path" into a Route.
func parseRouteString(s string) models.Route {
	r := models.Route{}
	// Split path
	if idx := strings.Index(s, "/"); idx != -1 {
		r.Path = s[idx:]
		s = s[:idx]
	}
	// Split host.domain (assume first segment is host, rest is domain)
	parts := strings.SplitN(s, ".", 2)
	if len(parts) == 2 {
		r.Host = parts[0]
		r.Domain = parts[1]
	} else {
		r.Host = s
		r.Domain = "cf-local.dev"
	}
	return r
}

// normalizeProtocol maps user-friendly protocol names to internal constants.
func normalizeProtocol(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "websocket", "ws", "wss":
		return "websocket"
	case "grpc", "grpcs":
		return "grpc"
	case "tcp":
		return "tcp"
	default:
		return "" // HTTP default
	}
}

// ParseMemoryMB converts CF-style memory string to megabytes.
// "256M" -> 256, "1G" -> 1024, "512m" -> 512
func ParseMemoryMB(s string) int {
	s = strings.TrimSpace(strings.ToUpper(s))
	if strings.HasSuffix(s, "G") {
		n, _ := strconv.Atoi(strings.TrimSuffix(s, "G"))
		return n * 1024
	}
	if strings.HasSuffix(s, "M") {
		n, _ := strconv.Atoi(strings.TrimSuffix(s, "M"))
		return n
	}
	n, _ := strconv.Atoi(s)
	return n
}
