package k8s

import (
	"context"
	"fmt"
	"sync"

	"github.com/younjinjeong/microfoundry/pkg/config"
	"github.com/younjinjeong/microfoundry/pkg/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClientManager manages multiple K8s clients, one per registered cluster.
type ClientManager struct {
	clients map[string]*Client
	configs map[string]config.ClusterConfig
	active  string
	mu      sync.RWMutex
}

// NewClientManager creates a manager from the config's cluster map.
func NewClientManager(clusters map[string]config.ClusterConfig, active string) *ClientManager {
	return &ClientManager{
		clients: make(map[string]*Client),
		configs: clusters,
		active:  active,
	}
}

// GetClient returns a cached or newly-created client for the given cluster ID.
func (cm *ClientManager) GetClient(clusterID string) (*Client, error) {
	cm.mu.RLock()
	if client, ok := cm.clients[clusterID]; ok {
		cm.mu.RUnlock()
		return client, nil
	}
	cm.mu.RUnlock()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Double-check after acquiring write lock
	if client, ok := cm.clients[clusterID]; ok {
		return client, nil
	}

	cfg, ok := cm.configs[clusterID]
	if !ok {
		return nil, fmt.Errorf("cluster %q not found", clusterID)
	}

	opts := []ClientOption{WithIngressClass(cfg.IngressClass)}
	if cfg.EKSClusterName != "" {
		opts = append(opts, WithEKSCluster(cfg.EKSClusterName, cfg.Region))
	}
	client, err := NewClient(cfg.Context, cfg.Namespace, cfg.Domain, opts...)
	if err != nil {
		return nil, fmt.Errorf("connecting to cluster %q: %w", clusterID, err)
	}

	cm.clients[clusterID] = client
	return client, nil
}

// GetActiveClient returns the client for the currently active cluster.
func (cm *ClientManager) GetActiveClient() (*Client, error) {
	cm.mu.RLock()
	active := cm.active
	cm.mu.RUnlock()
	return cm.GetClient(active)
}

// SetActive changes the active cluster.
func (cm *ClientManager) SetActive(clusterID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if _, ok := cm.configs[clusterID]; !ok {
		return fmt.Errorf("cluster %q not found", clusterID)
	}
	cm.active = clusterID
	return nil
}

// GetActive returns the currently active cluster ID.
func (cm *ClientManager) GetActive() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.active
}

// ListClusters returns info for all registered clusters.
// Copies config under read-lock, then makes network calls without holding the lock.
func (cm *ClientManager) ListClusters(ctx context.Context) []models.ClusterInfo {
	cm.mu.RLock()
	configs := make(map[string]config.ClusterConfig, len(cm.configs))
	active := cm.active
	for id, cfg := range cm.configs {
		configs[id] = cfg
	}
	cm.mu.RUnlock()

	var infos []models.ClusterInfo
	for id, cfg := range configs {
		gw := NewGatewayProvider(cfg.IngressClass)
		info := models.ClusterInfo{
			ID:           id,
			Name:         cfg.Name,
			Provider:     cfg.Provider,
			Region:       cfg.Region,
			Context:      cfg.Context,
			Namespace:    cfg.Namespace,
			Domain:       cfg.Domain,
			Enabled:      cfg.Enabled,
			IsActive:     id == active,
			Status:       "disconnected",
			IngressClass: gw.IngressClass(),
			Capabilities: gw.Capabilities(),
		}

		// GetClient has its own proper locking for lazy initialization
		if client, err := cm.GetClient(id); err == nil {
			info.Status = "connected"
			if names, err := client.ListApps(ctx); err == nil {
				info.AppCount = len(names)
			}
			if nodes, err := client.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err == nil {
				info.NodeCount = len(nodes.Items)
			}
			if ver, err := client.Clientset.Discovery().ServerVersion(); err == nil {
				info.Version = ver.GitVersion
			}
		} else {
			info.Status = "error"
			info.StatusMsg = err.Error()
		}

		infos = append(infos, info)
	}
	return infos
}

// GetClusterDetail returns detailed info for a single cluster.
func (cm *ClientManager) GetClusterDetail(ctx context.Context, clusterID string) (*models.ClusterDetail, error) {
	cm.mu.RLock()
	cfg, ok := cm.configs[clusterID]
	isActive := clusterID == cm.active
	cm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("cluster %q not found", clusterID)
	}

	client, err := cm.GetClient(clusterID)
	if err != nil {
		return nil, err
	}

	gw := NewGatewayProvider(cfg.IngressClass)
	detail := &models.ClusterDetail{
		ClusterInfo: models.ClusterInfo{
			ID:           clusterID,
			Name:         cfg.Name,
			Provider:     cfg.Provider,
			Region:       cfg.Region,
			Context:      cfg.Context,
			Namespace:    cfg.Namespace,
			Domain:       cfg.Domain,
			Enabled:      cfg.Enabled,
			IsActive:     isActive,
			Status:       "connected",
			IngressClass: gw.IngressClass(),
			Capabilities: gw.Capabilities(),
		},
	}

	// K8s server version
	if ver, err := client.Clientset.Discovery().ServerVersion(); err == nil {
		detail.Version = ver.GitVersion
	}

	// Node info
	nodes, err := client.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err == nil {
		detail.NodeCount = len(nodes.Items)
		var totalCPUMillis int64
		var totalMemBytes int64
		for _, node := range nodes.Items {
			roles := "worker"
			if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
				roles = "control-plane"
			}
			ni := models.NodeInfo{
				Name:    node.Name,
				Version: node.Status.NodeInfo.KubeletVersion,
				OS:      node.Status.NodeInfo.OperatingSystem + "/" + node.Status.NodeInfo.Architecture,
			}
			ni.Roles = roles
			for _, cond := range node.Status.Conditions {
				if cond.Type == "Ready" {
					if cond.Status == "True" {
						ni.Status = "Ready"
					} else {
						ni.Status = "NotReady"
					}
				}
			}
			alloc := node.Status.Allocatable
			if cpu, ok := alloc["cpu"]; ok {
				ni.CPU = cpu.String()
			}
			if mem, ok := alloc["memory"]; ok {
				ni.Memory = mem.String()
			}
			detail.Nodes = append(detail.Nodes, ni)

			// Aggregate resources across all nodes
			cap := node.Status.Capacity
			if cpu, ok := cap["cpu"]; ok {
				totalCPUMillis += cpu.MilliValue()
			}
			if mem, ok := cap["memory"]; ok {
				totalMemBytes += mem.Value()
			}
			if pods, ok := cap["pods"]; ok {
				detail.ResourceUsage.PodCapacity += int(pods.Value())
			}
		}
		// Format aggregated totals
		if totalCPUMillis > 0 {
			detail.ResourceUsage.CPUCapacity = fmt.Sprintf("%dm", totalCPUMillis)
		}
		if totalMemBytes > 0 {
			detail.ResourceUsage.MemoryCapacity = fmt.Sprintf("%dMi", totalMemBytes/(1024*1024))
		}
	}

	// App count
	if names, err := client.ListApps(ctx); err == nil {
		detail.AppCount = len(names)
	}

	// Pod count for resource usage
	if pods, err := client.Clientset.CoreV1().Pods(cfg.Namespace).List(ctx, metav1.ListOptions{}); err == nil {
		detail.ResourceUsage.PodUsed = len(pods.Items)
	}

	return detail, nil
}

// AddCluster registers a new cluster.
func (cm *ClientManager) AddCluster(id string, cfg config.ClusterConfig) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if _, ok := cm.configs[id]; ok {
		return fmt.Errorf("cluster %q already exists", id)
	}
	cm.configs[id] = cfg
	return nil
}

// RemoveCluster unregisters a cluster.
func (cm *ClientManager) RemoveCluster(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if _, ok := cm.configs[id]; !ok {
		return fmt.Errorf("cluster %q not found", id)
	}
	if id == cm.active {
		return fmt.Errorf("cannot remove active cluster %q", id)
	}
	delete(cm.configs, id)
	delete(cm.clients, id)
	return nil
}

// CheckHealth verifies connectivity to a cluster.
func (cm *ClientManager) CheckHealth(ctx context.Context, clusterID string) (string, error) {
	client, err := cm.GetClient(clusterID)
	if err != nil {
		return "error", err
	}
	_, err = client.Clientset.Discovery().ServerVersion()
	if err != nil {
		return "disconnected", err
	}
	return "connected", nil
}
