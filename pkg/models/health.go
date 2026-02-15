package models

type HealthCheckType string

const (
	HealthCheckHTTP    HealthCheckType = "http"
	HealthCheckPort    HealthCheckType = "port"
	HealthCheckProcess HealthCheckType = "process"
)

// HealthCheck maps to K8s readiness/liveness probes.
type HealthCheck struct {
	Type     HealthCheckType `json:"type"`
	Endpoint string          `json:"endpoint,omitempty"`
	Timeout  int             `json:"timeout,omitempty"`
}
