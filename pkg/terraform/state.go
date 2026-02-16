package terraform

import (
	"context"
	"fmt"

	"github.com/younjinjeong/microfoundry/pkg/k8s"
	"github.com/younjinjeong/microfoundry/pkg/models"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TopologyKey identifies the service-type + plan for a terraform-provisioned instance.
type TopologyKey struct {
	ServiceType string
	PlanID      string
}

// StateStore handles ConfigMap CRUD for terraform state per service instance.
type StateStore struct {
	k8sClient *k8s.Client
}

// NewStateStore creates a new terraform state store.
func NewStateStore(client *k8s.Client) *StateStore {
	return &StateStore{k8sClient: client}
}

func stateConfigMapName(instanceName string) string {
	return models.TopologyStateConfigMapPrefix + instanceName
}

// Get returns the terraform state for a service instance.
func (s *StateStore) Get(ctx context.Context, instanceName string) (string, error) {
	cmName := stateConfigMapName(instanceName)
	cm, err := s.k8sClient.Clientset.CoreV1().ConfigMaps(s.k8sClient.Namespace).Get(ctx, cmName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", fmt.Errorf("terraform state for %q not found", instanceName)
		}
		return "", err
	}
	return cm.Data["terraform.tfstate"], nil
}

// SaveWithTopologyKey saves terraform state and records the topology key in labels.
func (s *StateStore) SaveWithTopologyKey(ctx context.Context, instanceName, stateJSON, serviceType, planID string) error {
	cmName := stateConfigMapName(instanceName)
	ns := s.k8sClient.Namespace

	labels := map[string]string{
		"app.kubernetes.io/managed-by":      "microfoundry",
		"microfoundry.io/component":         "tf-state",
		"microfoundry.io/service-instance":  instanceName,
		"microfoundry.io/service-type":      serviceType,
		"microfoundry.io/plan-id":           planID,
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: ns,
			Labels:    labels,
		},
		Data: map[string]string{
			"terraform.tfstate": stateJSON,
		},
	}

	existing, err := s.k8sClient.Clientset.CoreV1().ConfigMaps(ns).Get(ctx, cmName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = s.k8sClient.Clientset.CoreV1().ConfigMaps(ns).Create(ctx, cm, metav1.CreateOptions{})
	} else if err == nil {
		cm.ResourceVersion = existing.ResourceVersion
		_, err = s.k8sClient.Clientset.CoreV1().ConfigMaps(ns).Update(ctx, cm, metav1.UpdateOptions{})
	}
	return err
}

// GetTopologyKey returns the service-type + plan used to provision an instance.
func (s *StateStore) GetTopologyKey(ctx context.Context, instanceName string) (TopologyKey, error) {
	cmName := stateConfigMapName(instanceName)
	cm, err := s.k8sClient.Clientset.CoreV1().ConfigMaps(s.k8sClient.Namespace).Get(ctx, cmName, metav1.GetOptions{})
	if err != nil {
		return TopologyKey{}, err
	}
	return TopologyKey{
		ServiceType: cm.Labels["microfoundry.io/service-type"],
		PlanID:      cm.Labels["microfoundry.io/plan-id"],
	}, nil
}

// Delete removes the terraform state ConfigMap for an instance.
func (s *StateStore) Delete(ctx context.Context, instanceName string) error {
	cmName := stateConfigMapName(instanceName)
	err := s.k8sClient.Clientset.CoreV1().ConfigMaps(s.k8sClient.Namespace).Delete(ctx, cmName, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// Exists checks if terraform state exists for an instance.
func (s *StateStore) Exists(ctx context.Context, instanceName string) bool {
	cmName := stateConfigMapName(instanceName)
	_, err := s.k8sClient.Clientset.CoreV1().ConfigMaps(s.k8sClient.Namespace).Get(ctx, cmName, metav1.GetOptions{})
	return err == nil
}
