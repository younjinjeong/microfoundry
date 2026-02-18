package models

const (
	ProviderDockerDesktop = "docker-desktop"
	ProviderEKS           = "eks"
	ProviderGKE           = "gke"
	ProviderAKS           = "aks"
	ProviderNative        = "native"
)

// ClusterInfo is the runtime view of a registered cluster.
type ClusterInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Provider     string   `json:"provider"`
	Region       string   `json:"region,omitempty"`
	Context      string   `json:"context"`
	Namespace    string   `json:"namespace"`
	Domain       string   `json:"domain"`
	Status       string   `json:"status"` // connected, disconnected, error
	Enabled      bool     `json:"enabled"`
	IsActive     bool     `json:"is_active"`
	NodeCount    int      `json:"node_count"`
	AppCount     int      `json:"app_count"`
	Version      string   `json:"version,omitempty"`
	StatusMsg    string   `json:"status_message,omitempty"`
	IngressClass string   `json:"ingress_class,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// ClusterDetail extends ClusterInfo with node and resource data.
type ClusterDetail struct {
	ClusterInfo
	Nodes         []NodeInfo      `json:"nodes"`
	ResourceUsage ResourceSummary `json:"resource_usage"`
}

// NodeInfo represents a K8s node.
type NodeInfo struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Roles   string `json:"roles"`
	Version string `json:"version"`
	OS      string `json:"os"`
	CPU     string `json:"cpu"`
	Memory  string `json:"memory"`
}

// ResourceSummary holds cluster-wide resource usage.
type ResourceSummary struct {
	CPUCapacity    string `json:"cpu_capacity"`
	CPUUsed        string `json:"cpu_used"`
	MemoryCapacity string `json:"memory_capacity"`
	MemoryUsed     string `json:"memory_used"`
	PodCapacity    int    `json:"pod_capacity"`
	PodUsed        int    `json:"pod_used"`
}
