package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for MicroFoundry.
type Config struct {
	GitHub     GitHubConfig     `mapstructure:"github"`
	Kubernetes KubernetesConfig `mapstructure:"kubernetes"`
	Admin      AdminConfig      `mapstructure:"admin"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
	Registry   RegistryConfig   `mapstructure:"registry"`
	SMTP       SMTPConfig       `mapstructure:"smtp"`
	Auth       AuthConfig       `mapstructure:"auth"`
	UI         UIConfig         `mapstructure:"ui"`
}

// AdminConfig holds admin UI server settings.
type AdminConfig struct {
	Port    int    `mapstructure:"port"`
	Host    string `mapstructure:"host"`
	Domain  string `mapstructure:"domain"`
	TLS     bool   `mapstructure:"tls"`
	TLSPort int    `mapstructure:"tls_port"`
	TLSCert string `mapstructure:"tls_cert"`
	TLSKey  string `mapstructure:"tls_key"`
}

// UIConfig holds admin UI feature toggles.
type UIConfig struct {
	Tooltips bool `mapstructure:"tooltips"`
}

// RegistryConfig provides file-based defaults for container registry settings.
// Runtime settings from the admin UI (stored in K8s ConfigMap) take precedence.
type RegistryConfig struct {
	URL      string `mapstructure:"url"`
	Project  string `mapstructure:"project"`
	Username string `mapstructure:"username"`
	Insecure bool   `mapstructure:"insecure"`
}

// SMTPConfig provides file-based defaults for SMTP settings.
type SMTPConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	FromAddr string `mapstructure:"from_addr"`
	TLS      bool   `mapstructure:"tls"`
}

// AuthConfig holds OIDC/Keycloak authentication settings.
type AuthConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	IssuerURL    string `mapstructure:"issuer_url"`
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
	SessionKey   string `mapstructure:"session_key"`

	// Keycloak Admin API (for user CRUD and SCIM)
	AdminBaseURL      string `mapstructure:"admin_base_url"`
	AdminClientID     string `mapstructure:"admin_client_id"`
	AdminClientSecret string `mapstructure:"admin_client_secret"`
	Realm             string `mapstructure:"realm"`
}

// MonitoringConfig holds endpoints for the monitoring stack.
type MonitoringConfig struct {
	GrafanaURL      string `mapstructure:"grafana_url"`
	LokiURL         string `mapstructure:"loki_url"`
	AlertmanagerURL string `mapstructure:"alertmanager_url"`
	PrometheusURL   string `mapstructure:"prometheus_url"`
	BeylaEnabled    bool   `mapstructure:"beyla_enabled"`
}

type GitHubConfig struct {
	Owner string `mapstructure:"owner"`
	Repo  string `mapstructure:"repo"`
}

// ClusterConfig represents a registered Kubernetes cluster.
type ClusterConfig struct {
	Name      string `mapstructure:"name"      json:"name"`
	Context   string `mapstructure:"context"   json:"context"`
	Namespace string `mapstructure:"namespace" json:"namespace"`
	Domain    string `mapstructure:"domain"    json:"domain"`
	Provider  string `mapstructure:"provider"  json:"provider"`
	Region    string `mapstructure:"region"    json:"region,omitempty"`
	Enabled   bool   `mapstructure:"enabled"   json:"enabled"`
	TLS       bool   `mapstructure:"tls"       json:"tls"`
}

type KubernetesConfig struct {
	// Multi-cluster support
	Clusters map[string]ClusterConfig `mapstructure:"clusters"`
	Active   string                   `mapstructure:"active"`

	// Legacy single-cluster fields (for migration)
	Context   string `mapstructure:"context"`
	Namespace string `mapstructure:"namespace"`
}

// Migrate converts legacy single-cluster config to multi-cluster format.
func (k *KubernetesConfig) Migrate() {
	if len(k.Clusters) > 0 {
		return // Already new format
	}

	ctx := k.Context
	if ctx == "" {
		ctx = "docker-desktop"
	}
	ns := k.Namespace
	if ns == "" {
		ns = "microfoundry"
	}

	k.Clusters = map[string]ClusterConfig{
		ctx: {
			Name:      ctx,
			Context:   ctx,
			Namespace: ns,
			Domain:    "cf-local.dev",
			Provider:  DetectProvider(ctx),
			Enabled:   true,
		},
	}
	k.Active = ctx
}

// GetActiveCluster returns the active cluster config.
func (k *KubernetesConfig) GetActiveCluster() (string, ClusterConfig, bool) {
	if k.Active == "" {
		return "", ClusterConfig{}, false
	}
	cfg, ok := k.Clusters[k.Active]
	return k.Active, cfg, ok
}

// DetectProvider guesses the provider from a kubeconfig context name.
func DetectProvider(context string) string {
	lower := strings.ToLower(context)
	switch {
	case strings.Contains(lower, "docker-desktop"):
		return "docker-desktop"
	case strings.Contains(lower, "eks") || strings.Contains(lower, "aws"):
		return "eks"
	case strings.Contains(lower, "gke") || strings.Contains(lower, "gke_"):
		return "gke"
	case strings.Contains(lower, "aks") || strings.Contains(lower, "azure"):
		return "aks"
	default:
		return "native"
	}
}

// Load reads configuration from files and env vars and returns a Config.
func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("mf")
	v.SetConfigType("yaml")

	// Search paths: project dir, then user home
	v.AddConfigPath("./configs")
	if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(filepath.Join(home, ".mf"))
	}

	// Environment variable bindings
	v.SetEnvPrefix("MF")
	v.AutomaticEnv()

	// Defaults (legacy format)
	v.SetDefault("github.owner", "younjinjeong")
	v.SetDefault("github.repo", "microfoundry")
	v.SetDefault("kubernetes.context", "docker-desktop")
	v.SetDefault("kubernetes.namespace", "microfoundry")
	v.SetDefault("monitoring.grafana_url", "http://localhost:3000")
	v.SetDefault("monitoring.loki_url", "http://loki.monitoring.svc.cluster.local:3100")
	v.SetDefault("monitoring.alertmanager_url", "http://kube-prometheus-kube-prome-alertmanager.monitoring.svc.cluster.local:9093")
	v.SetDefault("monitoring.prometheus_url", "http://kube-prometheus-kube-prome-prometheus.monitoring.svc.cluster.local:9090")
	v.SetDefault("monitoring.beyla_enabled", true)
	v.SetDefault("ui.tooltips", true)
	v.SetDefault("admin.port", 8080)
	v.SetDefault("admin.host", "0.0.0.0")
	v.SetDefault("admin.domain", "")
	v.SetDefault("admin.tls", false)
	v.SetDefault("admin.tls_port", 8443)

	// Read config file (optional — not an error if missing)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	// Auto-migrate legacy config to multi-cluster format
	cfg.Kubernetes.Migrate()

	return &cfg, nil
}

// AdminURL returns the base URL for the admin UI (e.g., "https://admin.cf-local.dev:8443").
func (a *AdminConfig) AdminURL() string {
	if a.TLS && a.Domain != "" {
		port := a.TLSPort
		if port == 0 {
			port = 8443
		}
		if port == 443 {
			return fmt.Sprintf("https://%s", a.Domain)
		}
		return fmt.Sprintf("https://%s:%d", a.Domain, port)
	}
	port := a.Port
	if port == 0 {
		port = 8080
	}
	if a.Domain != "" && !a.TLS {
		// Domain set but no TLS — still show domain (HTTP)
		if port == 80 {
			return fmt.Sprintf("http://%s", a.Domain)
		}
		return fmt.Sprintf("http://%s:%d", a.Domain, port)
	}
	if port == 80 {
		return "http://localhost"
	}
	return fmt.Sprintf("http://localhost:%d", port)
}

// ConfigDir returns the path to the user's mf config directory.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".mf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}
