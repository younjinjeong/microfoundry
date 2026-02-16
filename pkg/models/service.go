package models

import (
	"regexp"
	"time"
)

// Service status constants
const (
	ServiceStatusCreating  = "creating"
	ServiceStatusAvailable = "available"
	ServiceStatusFailed    = "failed"
	ServiceStatusDeleting  = "deleting"
	ServiceStatusDeleted   = "deleted"
)

// ServiceSecretPrefix is the K8s Secret name prefix for service credentials.
const ServiceSecretPrefix = "mf-svc-"

// ValidServiceName matches valid service instance names (lowercase alphanumeric + hyphens, 2-42 chars).
var ValidServiceName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,40}[a-z0-9]$`)

// ServiceBindingInfo represents a service bound to an app (display only).
type ServiceBindingInfo struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

// ServiceType defines a backing service offering in the catalog.
type ServiceType struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Provider    string        `json:"provider"`
	Category    string        `json:"category"`
	Label       string        `json:"label"` // VCAP_SERVICES label (e.g., "mariadb")
	Plans       []ServicePlan `json:"plans"`
	Tags        []string      `json:"tags,omitempty"`
}

// ServicePlan defines a sizing tier within a service type.
type ServicePlan struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Resources    PlanResources `json:"resources"`
	CostEstimate string        `json:"cost_estimate"`
}

// PlanResources defines K8s resource allocation for a service plan.
type PlanResources struct {
	MemoryMB  int `json:"memory_mb"`  // container memory limit in MiB
	CPUMillis int `json:"cpu_millis"` // container CPU request in millicores
	StorageGB int `json:"storage_gb"` // PVC size in GiB, 0 = no PVC
}

// ServiceInstance represents a provisioned backing service.
type ServiceInstance struct {
	Name            string            `json:"name"`
	ServiceType     string            `json:"service_type"`
	Plan            string            `json:"plan"`
	Status          string            `json:"status"`
	StatusMsg       string            `json:"status_message,omitempty"`
	ClusterID       string            `json:"cluster_id"`
	Region          string            `json:"region,omitempty"`
	ProvisionMethod string            `json:"provision_method,omitempty"` // "k8s-native" or "terraform"
	Parameters      map[string]string `json:"parameters,omitempty"`
	Outputs         ServiceOutputs    `json:"outputs,omitempty"`
	Bindings        []ServiceBinding  `json:"bindings,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// Redacted returns a copy with sensitive fields masked.
func (si *ServiceInstance) Redacted() ServiceInstance {
	out := *si
	out.Outputs = si.Outputs.Redacted()
	return out
}

// ServiceOutputs holds the provisioned resource connection details.
type ServiceOutputs struct {
	Host     string            `json:"host,omitempty"`
	Port     int               `json:"port,omitempty"`
	Username string            `json:"username,omitempty"`
	Password string            `json:"password,omitempty"`
	Database string            `json:"database,omitempty"`
	URI      string            `json:"uri,omitempty"`
	Extra    map[string]string `json:"extra,omitempty"` // service-specific: access_key, bucket, mgmt_url, etc.
}

// Redacted returns a copy with password and URI masked.
func (o ServiceOutputs) Redacted() ServiceOutputs {
	out := o
	if out.Password != "" {
		out.Password = "********"
	}
	if out.URI != "" {
		out.URI = "********"
	}
	return out
}

// ServiceBinding represents a binding between an app and a service instance.
type ServiceBinding struct {
	AppName   string    `json:"app_name"`
	SecretRef string    `json:"secret_ref"`
	BoundAt   time.Time `json:"bound_at"`
}

// GatewayRoute represents a single proxy route in a gateway's configuration.
type GatewayRoute struct {
	AppName  string `json:"app_name"`
	Path     string `json:"path"`     // e.g., "/hello-world"
	Upstream string `json:"upstream"` // e.g., "http://hello-world.microfoundry.svc.cluster.local:80"
}

// ServiceListItem is a summary view for list pages.
type ServiceListItem struct {
	Name        string    `json:"name"`
	ServiceType string    `json:"service_type"`
	Plan        string    `json:"plan"`
	Status      string    `json:"status"`
	ClusterID   string    `json:"cluster_id"`
	BoundApps   int       `json:"bound_apps"`
	CreatedAt   time.Time `json:"created_at"`
}
