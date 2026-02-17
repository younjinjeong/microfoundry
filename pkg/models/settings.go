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

// PlatformSettings is the aggregate of all platform settings stored in ConfigMap.
type PlatformSettings struct {
	Registry RegistryConfig  `json:"registry"`
	Webhooks []WebhookConfig `json:"webhooks"`
	SMTP     SMTPConfig      `json:"smtp"`
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
