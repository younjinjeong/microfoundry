package models

import "time"

const TopologyConfigMapPrefix = "mf-tf-topology-"
const TopologyStateConfigMapPrefix = "mf-tf-state-"

// TopologyMetadata holds versioning and descriptive info for a topology config.
type TopologyMetadata struct {
	ServiceType string    `json:"service_type"`
	PlanID      string    `json:"plan"`
	Version     int       `json:"version"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedBy   string    `json:"created_by,omitempty"`
}

// TopologyConfig stores Terraform files for a service-type + plan combination.
type TopologyConfig struct {
	ServiceType string            `json:"service_type"`
	PlanID      string            `json:"plan"`
	Metadata    TopologyMetadata  `json:"metadata"`
	Files       map[string]string `json:"files"` // e.g., "main.tf" → content
}

// TopologyListItem is a summary view for the topology list page.
type TopologyListItem struct {
	ServiceType string    `json:"service_type"`
	PlanID      string    `json:"plan"`
	Description string    `json:"description,omitempty"`
	Version     int       `json:"version"`
	FileCount   int       `json:"file_count"`
	HasTopology bool      `json:"has_topology"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TopologyPlanResult holds the output of a terraform plan preview.
type TopologyPlanResult struct {
	ServiceType string `json:"service_type"`
	PlanID      string `json:"plan"`
	Output      string `json:"output"`
	HasChanges  bool   `json:"has_changes"`
	Error       string `json:"error,omitempty"`
}
