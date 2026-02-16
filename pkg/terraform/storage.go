package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/younjinjeong/microfoundry/pkg/k8s"
	"github.com/younjinjeong/microfoundry/pkg/models"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TopologyStore handles ConfigMap CRUD for topology configs.
type TopologyStore struct {
	k8sClient *k8s.Client
}

// NewTopologyStore creates a new topology store.
func NewTopologyStore(client *k8s.Client) *TopologyStore {
	return &TopologyStore{k8sClient: client}
}

func topologyConfigMapName(serviceType, planID string) string {
	return models.TopologyConfigMapPrefix + serviceType + "-" + planID
}

func topologyLabels(serviceType, planID string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "microfoundry",
		"microfoundry.io/component":    "tf-topology",
		"microfoundry.io/service-type": serviceType,
		"microfoundry.io/plan-id":      planID,
	}
}

// Get returns a topology config for a service-type + plan.
func (s *TopologyStore) Get(ctx context.Context, serviceType, planID string) (*models.TopologyConfig, error) {
	cmName := topologyConfigMapName(serviceType, planID)
	cm, err := s.k8sClient.Clientset.CoreV1().ConfigMaps(s.k8sClient.Namespace).Get(ctx, cmName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("topology for %s/%s not found", serviceType, planID)
		}
		return nil, err
	}

	var meta models.TopologyMetadata
	if metaJSON, ok := cm.Data["metadata"]; ok {
		if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil {
			return nil, fmt.Errorf("unmarshalling topology metadata: %w", err)
		}
	}

	files := make(map[string]string)
	for k, v := range cm.Data {
		if k != "metadata" && strings.HasSuffix(k, ".tf") {
			files[k] = v
		}
	}

	return &models.TopologyConfig{
		ServiceType: serviceType,
		PlanID:      planID,
		Metadata:    meta,
		Files:       files,
	}, nil
}

// Save creates or updates a topology config with version increment.
func (s *TopologyStore) Save(ctx context.Context, config *models.TopologyConfig) error {
	cmName := topologyConfigMapName(config.ServiceType, config.PlanID)
	labels := topologyLabels(config.ServiceType, config.PlanID)
	ns := s.k8sClient.Namespace

	// Try to load existing for version increment
	existing, err := s.k8sClient.Clientset.CoreV1().ConfigMaps(ns).Get(ctx, cmName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		// New topology
		config.Metadata.Version = 1
		config.Metadata.CreatedAt = time.Now()
	} else {
		// Increment version from existing
		var existingMeta models.TopologyMetadata
		if metaJSON, ok := existing.Data["metadata"]; ok {
			_ = json.Unmarshal([]byte(metaJSON), &existingMeta)
		}
		config.Metadata.Version = existingMeta.Version + 1
		config.Metadata.CreatedAt = existingMeta.CreatedAt
	}

	config.Metadata.ServiceType = config.ServiceType
	config.Metadata.PlanID = config.PlanID
	config.Metadata.UpdatedAt = time.Now()

	metaJSON, err := json.Marshal(config.Metadata)
	if err != nil {
		return err
	}

	data := map[string]string{
		"metadata": string(metaJSON),
	}
	for k, v := range config.Files {
		data[k] = v
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: ns,
			Labels:    labels,
		},
		Data: data,
	}

	if errors.IsNotFound(err) {
		_, err = s.k8sClient.Clientset.CoreV1().ConfigMaps(ns).Create(ctx, cm, metav1.CreateOptions{})
	} else {
		cm.ResourceVersion = existing.ResourceVersion
		_, err = s.k8sClient.Clientset.CoreV1().ConfigMaps(ns).Update(ctx, cm, metav1.UpdateOptions{})
	}
	return err
}

// Delete removes a topology config.
func (s *TopologyStore) Delete(ctx context.Context, serviceType, planID string) error {
	cmName := topologyConfigMapName(serviceType, planID)
	err := s.k8sClient.Clientset.CoreV1().ConfigMaps(s.k8sClient.Namespace).Delete(ctx, cmName, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// Exists checks if a topology config exists for a service-type + plan.
func (s *TopologyStore) Exists(ctx context.Context, serviceType, planID string) bool {
	cmName := topologyConfigMapName(serviceType, planID)
	_, err := s.k8sClient.Clientset.CoreV1().ConfigMaps(s.k8sClient.Namespace).Get(ctx, cmName, metav1.GetOptions{})
	return err == nil
}

// List returns all catalog type/plan combos with topology status.
// catalog should be the result of service.Catalog() passed from the caller to avoid import cycles.
func (s *TopologyStore) List(ctx context.Context, catalog []models.ServiceType) ([]models.TopologyListItem, error) {
	// Get all topology ConfigMaps
	cms, err := s.k8sClient.Clientset.CoreV1().ConfigMaps(s.k8sClient.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=microfoundry,microfoundry.io/component=tf-topology",
	})
	if err != nil {
		return nil, fmt.Errorf("listing topology ConfigMaps: %w", err)
	}

	// Index existing topologies by key
	topologyMap := make(map[string]*corev1.ConfigMap)
	for i := range cms.Items {
		cm := &cms.Items[i]
		svcType := cm.Labels["microfoundry.io/service-type"]
		planID := cm.Labels["microfoundry.io/plan-id"]
		if svcType != "" && planID != "" {
			topologyMap[svcType+"/"+planID] = cm
		}
	}

	// Build list from catalog
	var items []models.TopologyListItem
	for _, svcType := range catalog {
		for _, plan := range svcType.Plans {
			item := models.TopologyListItem{
				ServiceType: svcType.ID,
				PlanID:      plan.ID,
				HasTopology: false,
			}

			key := svcType.ID + "/" + plan.ID
			if cm, ok := topologyMap[key]; ok {
				item.HasTopology = true
				var meta models.TopologyMetadata
				if metaJSON, mOk := cm.Data["metadata"]; mOk {
					_ = json.Unmarshal([]byte(metaJSON), &meta)
				}
				item.Version = meta.Version
				item.Description = meta.Description
				item.UpdatedAt = meta.UpdatedAt

				// Count .tf files
				for k := range cm.Data {
					if k != "metadata" && strings.HasSuffix(k, ".tf") {
						item.FileCount++
					}
				}
			}

			items = append(items, item)
		}
	}
	return items, nil
}
