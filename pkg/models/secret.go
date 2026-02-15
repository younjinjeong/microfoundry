package models

import (
	"regexp"
	"time"
)

// UserSecretPrefix is the K8s Secret name prefix for user-defined secrets.
const UserSecretPrefix = "mf-secret-"

// ValidSecretName matches valid secret names (lowercase alphanumeric + hyphens, 2-42 chars).
var ValidSecretName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,40}[a-z0-9]$`)

// Secret type constants.
const (
	SecretTypeService = "service"
	SecretTypeUser    = "user-defined"
)

// SecretInfo represents K8s Secret metadata (no values exposed).
// Used in app detail views to show bound secrets.
type SecretInfo struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	KeyCount  int    `json:"key_count"`
	CreatedAt string `json:"created_at"`
}

// SecretListItem is the summary for secrets list views.
type SecretListItem struct {
	Name            string    `json:"name"`
	K8sName         string    `json:"k8s_name"`
	Type            string    `json:"type"`
	ServiceInstance string    `json:"service_instance,omitempty"`
	KeyCount        int       `json:"key_count"`
	BoundApps       int       `json:"bound_apps"`
	Description     string    `json:"description,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// SecretDetail is the full view for secret detail pages (values NOT included).
type SecretDetail struct {
	Name            string    `json:"name"`
	K8sName         string    `json:"k8s_name"`
	Type            string    `json:"type"`
	ServiceInstance string    `json:"service_instance,omitempty"`
	Keys            []string  `json:"keys"`
	KeyCount        int       `json:"key_count"`
	BoundApps       []string  `json:"bound_apps"`
	Description     string    `json:"description,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}
