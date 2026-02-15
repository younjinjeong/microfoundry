package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/younjinjeong/microfoundry/pkg/k8s"
	"github.com/younjinjeong/microfoundry/pkg/models"
	"github.com/younjinjeong/microfoundry/pkg/secrets"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	serviceConfigMapPrefix = "mf-svc-meta-"
	labelManagedBy         = "microfoundry"
	labelServiceInstance   = "microfoundry.io/service-instance"
	maxRetries             = 3
)

// SecretName returns the K8s Secret name for a service instance.
func SecretName(instanceName string) string {
	return models.ServiceSecretPrefix + instanceName
}

// Manager handles service instance lifecycle using K8s as the backing store.
type Manager struct {
	k8sClient *k8s.Client
	secrets   *secrets.Manager
}

// NewManager creates a new service manager.
func NewManager(client *k8s.Client) *Manager {
	return &Manager{
		k8sClient: client,
		secrets:   secrets.NewManager(client),
	}
}

// List returns all service instances.
func (m *Manager) List(ctx context.Context) ([]models.ServiceListItem, error) {
	cms, err := m.k8sClient.Clientset.CoreV1().ConfigMaps(m.k8sClient.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=" + labelManagedBy + "," + labelServiceInstance,
	})
	if err != nil {
		return nil, fmt.Errorf("listing service instances: %w", err)
	}

	var items []models.ServiceListItem
	for _, cm := range cms.Items {
		var inst models.ServiceInstance
		if data, ok := cm.Data["instance"]; ok {
			if err := json.Unmarshal([]byte(data), &inst); err != nil {
				log.Printf("warning: skipping corrupted service ConfigMap %q: %v", cm.Name, err)
				continue
			}
		} else {
			log.Printf("warning: service ConfigMap %q missing 'instance' key", cm.Name)
			continue
		}
		items = append(items, models.ServiceListItem{
			Name:        inst.Name,
			ServiceType: inst.ServiceType,
			Plan:        inst.Plan,
			Status:      inst.Status,
			ClusterID:   inst.ClusterID,
			BoundApps:   len(inst.Bindings),
			CreatedAt:   inst.CreatedAt,
		})
	}
	return items, nil
}

// Get returns a single service instance by name.
func (m *Manager) Get(ctx context.Context, name string) (*models.ServiceInstance, error) {
	cm, err := m.k8sClient.Clientset.CoreV1().ConfigMaps(m.k8sClient.Namespace).Get(ctx, serviceConfigMapPrefix+name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("service instance %q not found", name)
		}
		return nil, err
	}

	data, ok := cm.Data["instance"]
	if !ok {
		return nil, fmt.Errorf("service instance %q has corrupted metadata (missing 'instance' key)", name)
	}

	var inst models.ServiceInstance
	if err := json.Unmarshal([]byte(data), &inst); err != nil {
		return nil, fmt.Errorf("unmarshalling service instance: %w", err)
	}

	// Load outputs from secret if available
	secret, err := m.k8sClient.Clientset.CoreV1().Secrets(m.k8sClient.Namespace).Get(ctx, SecretName(name), metav1.GetOptions{})
	if err == nil {
		inst.Outputs = models.ServiceOutputs{
			Host:     string(secret.Data["host"]),
			Username: string(secret.Data["username"]),
			Password: string(secret.Data["password"]),
			Database: string(secret.Data["database"]),
			URI:      string(secret.Data["uri"]),
		}
		if p, ok := secret.Data["port"]; ok {
			var port int
			if _, err := fmt.Sscanf(string(p), "%d", &port); err == nil {
				inst.Outputs.Port = port
			}
		}
	}

	return &inst, nil
}

// Create stores a new service instance metadata.
func (m *Manager) Create(ctx context.Context, inst *models.ServiceInstance) error {
	inst.CreatedAt = time.Now()
	inst.UpdatedAt = time.Now()
	inst.Status = models.ServiceStatusCreating

	data, err := json.Marshal(inst)
	if err != nil {
		return err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceConfigMapPrefix + inst.Name,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": labelManagedBy,
				labelServiceInstance:           inst.Name,
			},
		},
		Data: map[string]string{
			"instance": string(data),
		},
	}

	_, err = m.k8sClient.Clientset.CoreV1().ConfigMaps(m.k8sClient.Namespace).Create(ctx, cm, metav1.CreateOptions{})
	return err
}

// UpdateStatus updates the status of a service instance with conflict retry.
func (m *Manager) UpdateStatus(ctx context.Context, name, status, msg string) error {
	return m.retryOnConflict(ctx, name, func(inst *models.ServiceInstance) {
		inst.Status = status
		inst.StatusMsg = msg
	})
}

// SaveOutputs stores service outputs in a K8s Secret via the secrets manager.
func (m *Manager) SaveOutputs(ctx context.Context, name string, outputs models.ServiceOutputs) error {
	data := map[string]string{
		"host":     outputs.Host,
		"port":     fmt.Sprintf("%d", outputs.Port),
		"username": outputs.Username,
		"password": outputs.Password,
		"database": outputs.Database,
		"uri":      outputs.URI,
	}
	return m.secrets.CreateServiceSecret(ctx, name, data)
}

// Delete removes a service instance and its secret.
func (m *Manager) Delete(ctx context.Context, name string) error {
	cmErr := m.k8sClient.Clientset.CoreV1().ConfigMaps(m.k8sClient.Namespace).Delete(ctx, serviceConfigMapPrefix+name, metav1.DeleteOptions{})
	if cmErr != nil && !errors.IsNotFound(cmErr) {
		return fmt.Errorf("deleting service ConfigMap: %w", cmErr)
	}
	if secErr := m.secrets.Delete(ctx, name); secErr != nil {
		return fmt.Errorf("deleting service Secret: %w", secErr)
	}
	return nil
}

// AddBinding adds a binding to a service instance with conflict retry.
func (m *Manager) AddBinding(ctx context.Context, serviceName, appName string) error {
	return m.retryOnConflict(ctx, serviceName, func(inst *models.ServiceInstance) {
		inst.Bindings = append(inst.Bindings, models.ServiceBinding{
			AppName:   appName,
			SecretRef: SecretName(serviceName),
			BoundAt:   time.Now(),
		})
	})
}

// RemoveBinding removes a binding from a service instance with conflict retry.
func (m *Manager) RemoveBinding(ctx context.Context, serviceName, appName string) error {
	return m.retryOnConflict(ctx, serviceName, func(inst *models.ServiceInstance) {
		var updated []models.ServiceBinding
		for _, b := range inst.Bindings {
			if b.AppName != appName {
				updated = append(updated, b)
			}
		}
		inst.Bindings = updated
	})
}

// retryOnConflict performs a read-modify-write with resourceVersion conflict retry.
func (m *Manager) retryOnConflict(ctx context.Context, name string, mutate func(*models.ServiceInstance)) error {
	for i := 0; i < maxRetries; i++ {
		cm, err := m.k8sClient.Clientset.CoreV1().ConfigMaps(m.k8sClient.Namespace).Get(ctx, serviceConfigMapPrefix+name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		var inst models.ServiceInstance
		if data, ok := cm.Data["instance"]; ok {
			if err := json.Unmarshal([]byte(data), &inst); err != nil {
				return fmt.Errorf("unmarshalling service instance: %w", err)
			}
		} else {
			return fmt.Errorf("service instance %q has corrupted metadata", name)
		}

		mutate(&inst)
		inst.UpdatedAt = time.Now()

		data, err := json.Marshal(&inst)
		if err != nil {
			return err
		}
		cm.Data["instance"] = string(data)

		// Update with resourceVersion — K8s rejects if another write happened since our Get
		_, err = m.k8sClient.Clientset.CoreV1().ConfigMaps(m.k8sClient.Namespace).Update(ctx, cm, metav1.UpdateOptions{})
		if err == nil {
			return nil
		}
		if !errors.IsConflict(err) {
			return err
		}
		// Conflict — retry with fresh read
	}
	return fmt.Errorf("conflict: too many retries updating service %q", name)
}
