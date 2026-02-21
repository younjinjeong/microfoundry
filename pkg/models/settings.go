package models

import "strings"

const (
	PlatformSettingsConfigMapName = "mf-platform-settings"
	PlatformCredentialsSecretName = "mf-platform-credentials"
)

// RegistryConfig holds Docker/OCI container registry settings.
type RegistryConfig struct {
	URL      string `json:"url"      mapstructure:"url"`
	Project  string `json:"project"  mapstructure:"project"`
	Username string `json:"username" mapstructure:"username"`
	Insecure bool   `json:"insecure" mapstructure:"insecure"`
	Enabled  bool   `json:"enabled"  mapstructure:"enabled"`
}

// ImagePrefix returns the prefix for image tags (e.g., "harbor.local:30003/microfoundry/").
// Falls back to "microfoundry/" when registry is not configured.
func (r *RegistryConfig) ImagePrefix() string {
	if !r.Enabled || r.URL == "" {
		return "microfoundry/"
	}
	// Strip protocol scheme if present
	u := r.URL
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimRight(u, "/")

	if r.Project != "" {
		return u + "/" + r.Project + "/"
	}
	return u + "/"
}

// WebhookConfig holds a single webhook endpoint configuration.
type WebhookConfig struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	Enabled   bool     `json:"enabled"`
	CreatedAt string   `json:"created_at"`
}

// SMTPConfig holds SMTP server settings for email notifications.
type SMTPConfig struct {
	Host     string `json:"host"      mapstructure:"host"`
	Port     int    `json:"port"      mapstructure:"port"`
	Username string `json:"username"  mapstructure:"username"`
	FromAddr string `json:"from_addr" mapstructure:"from_addr"`
	TLS      bool   `json:"tls"       mapstructure:"tls"`
	Enabled  bool   `json:"enabled"   mapstructure:"enabled"`
}

// ServiceEndpoint holds the resolved configuration for a single platform service.
type ServiceEndpoint struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Namespace   string `json:"namespace"`
	ServiceName string `json:"service_name"`
	ServicePort int32  `json:"service_port"`
	HealthPath  string `json:"health_path"`
	InternalURL string `json:"internal_url"`
	IngressHost string `json:"ingress_host,omitempty"`
	OverrideURL string `json:"override_url,omitempty"`
	HasIngress  bool   `json:"has_ingress"`
}

// ResolvedURL returns the effective URL for this endpoint.
// Priority: OverrideURL > IngressHost (https) > InternalURL.
func (e *ServiceEndpoint) ResolvedURL() string {
	if e.OverrideURL != "" {
		return e.OverrideURL
	}
	if e.IngressHost != "" {
		return "https://" + e.IngressHost
	}
	return e.InternalURL
}

// EndpointsConfig holds user-provided URL overrides for platform services.
type EndpointsConfig struct {
	Overrides map[string]string `json:"overrides,omitempty"`
}

// PlatformServices defines the well-known platform services that MicroFoundry manages.
var PlatformServices = []ServiceEndpoint{
	{Name: "grafana", DisplayName: "Grafana", Namespace: "monitoring",
		ServiceName: "kube-prometheus-grafana", ServicePort: 80, HealthPath: "/api/health"},
	{Name: "prometheus", DisplayName: "Prometheus", Namespace: "monitoring",
		ServiceName: "kube-prometheus-kube-prome-prometheus", ServicePort: 9090, HealthPath: "/-/healthy"},
	{Name: "alertmanager", DisplayName: "Alertmanager", Namespace: "monitoring",
		ServiceName: "kube-prometheus-kube-prome-alertmanager", ServicePort: 9093, HealthPath: "/-/healthy"},
	{Name: "loki", DisplayName: "Loki", Namespace: "monitoring",
		ServiceName: "loki", ServicePort: 3100, HealthPath: "/ready"},
	{Name: "keycloak", DisplayName: "Keycloak", Namespace: "",
		ServiceName: "keycloak", ServicePort: 8180, HealthPath: "/health/ready"},
}

// PlatformSettings is the aggregate of all platform settings stored in ConfigMap.
type PlatformSettings struct {
	Registry       RegistryConfig      `json:"registry"`
	Webhooks       []WebhookConfig     `json:"webhooks"`
	SMTP           SMTPConfig          `json:"smtp"`
	Endpoints      EndpointsConfig     `json:"endpoints,omitempty"`
	CloudProviders CloudProviderConfig `json:"cloud_providers,omitempty"`
}

// CloudProviderConfig holds credentials and settings for cloud service providers.
type CloudProviderConfig struct {
	AWS   AWSConfig   `json:"aws"`
	GCP   GCPConfig   `json:"gcp"`
	Azure AzureConfig `json:"azure"`
}

// AWSConfig holds AWS authentication settings.
type AWSConfig struct {
	Enabled     bool   `json:"enabled"`
	Region      string `json:"region"`
	AccessKeyID string `json:"access_key_id"`
	AssumeRole  string `json:"assume_role,omitempty"`
}

// GCPConfig holds GCP authentication settings.
type GCPConfig struct {
	Enabled   bool   `json:"enabled"`
	ProjectID string `json:"project_id"`
	Region    string `json:"region"`
}

// AzureConfig holds Azure authentication settings.
type AzureConfig struct {
	Enabled        bool   `json:"enabled"`
	TenantID       string `json:"tenant_id"`
	ClientID       string `json:"client_id"`
	SubscriptionID string `json:"subscription_id"`
	ResourceGroup  string `json:"resource_group"`
	Location       string `json:"location"`
}

// WebhookEventTypes lists all supported event types for webhook subscriptions.
var WebhookEventTypes = []string{
	"app.deployed",
	"app.crashed",
	"app.scaled",
	"app.deleted",
	"alert.fired",
	"alert.resolved",
	"service.created",
	"service.deleted",
}
