package models

import "time"

// Service status constants
const (
	ServiceStatusCreating  = "creating"
	ServiceStatusAvailable = "available"
	ServiceStatusFailed    = "failed"
	ServiceStatusDeleting  = "deleting"
	ServiceStatusDeleted   = "deleted"
)

// ServiceBindingInfo represents a service bound to an app (display only).
type ServiceBindingInfo struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

// SecretInfo represents K8s Secret metadata (no values exposed).
type SecretInfo struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	KeyCount  int    `json:"key_count"`
	CreatedAt string `json:"created_at"`
}

// ServiceType defines a backing service offering in the catalog.
type ServiceType struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Provider    string        `json:"provider"`
	Category    string        `json:"category"`
	Plans       []ServicePlan `json:"plans"`
	Tags        []string      `json:"tags,omitempty"`
}

// ServicePlan defines a sizing tier within a service type.
type ServicePlan struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	InstanceClass string            `json:"instance_class"`
	StorageGB     int               `json:"storage_gb"`
	Replicas      int               `json:"replicas"`
	MultiAZ       bool              `json:"multi_az"`
	CostEstimate  string            `json:"cost_estimate"`
	Customizable  bool              `json:"customizable"`
	Defaults      map[string]string `json:"defaults,omitempty"`
}

// ServiceInstance represents a provisioned backing service.
type ServiceInstance struct {
	Name        string            `json:"name"`
	ServiceType string            `json:"service_type"`
	Plan        string            `json:"plan"`
	Status      string            `json:"status"`
	StatusMsg   string            `json:"status_message,omitempty"`
	ClusterID   string            `json:"cluster_id"`
	Region      string            `json:"region,omitempty"`
	Parameters  map[string]string `json:"parameters,omitempty"`
	Outputs     ServiceOutputs    `json:"outputs,omitempty"`
	Bindings    []ServiceBinding  `json:"bindings,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// ServiceOutputs holds the provisioned resource connection details.
type ServiceOutputs struct {
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Database string `json:"database,omitempty"`
	URI      string `json:"uri,omitempty"`
}

// ServiceBinding represents a binding between an app and a service instance.
type ServiceBinding struct {
	AppName   string    `json:"app_name"`
	SecretRef string    `json:"secret_ref"`
	BoundAt   time.Time `json:"bound_at"`
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
