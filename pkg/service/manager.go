package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/younjinjeong/microfoundry/pkg/k8s"
	"github.com/younjinjeong/microfoundry/pkg/models"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	serviceConfigMapPrefix = "mf-svc-meta-"
	serviceSecretPrefix    = "mf-svc-"
	labelManagedBy         = "microfoundry"
	labelServiceInstance   = "microfoundry.io/service-instance"
)

// Manager handles service instance lifecycle using K8s as the backing store.
type Manager struct {
	k8sClient *k8s.Client
}

// NewManager creates a new service manager.
func NewManager(client *k8s.Client) *Manager {
	return &Manager{k8sClient: client}
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
				continue
			}
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

	var inst models.ServiceInstance
	if data, ok := cm.Data["instance"]; ok {
		if err := json.Unmarshal([]byte(data), &inst); err != nil {
			return nil, fmt.Errorf("unmarshalling service instance: %w", err)
		}
	}

	// Load outputs from secret if available
	secret, err := m.k8sClient.Clientset.CoreV1().Secrets(m.k8sClient.Namespace).Get(ctx, serviceSecretPrefix+name, metav1.GetOptions{})
	if err == nil {
		inst.Outputs = models.ServiceOutputs{
			Host:     string(secret.Data["host"]),
			Username: string(secret.Data["username"]),
			Password: string(secret.Data["password"]),
			Database: string(secret.Data["database"]),
			URI:      string(secret.Data["uri"]),
		}
		if p, ok := secret.Data["port"]; ok {
			fmt.Sscanf(string(p), "%d", &inst.Outputs.Port)
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

// UpdateStatus updates the status of a service instance.
func (m *Manager) UpdateStatus(ctx context.Context, name, status, msg string) error {
	inst, err := m.Get(ctx, name)
	if err != nil {
		return err
	}
	inst.Status = status
	inst.StatusMsg = msg
	inst.UpdatedAt = time.Now()
	return m.save(ctx, inst)
}

// SaveOutputs stores service outputs in a K8s Secret.
func (m *Manager) SaveOutputs(ctx context.Context, name string, outputs models.ServiceOutputs) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceSecretPrefix + name,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": labelManagedBy,
				labelServiceInstance:           name,
			},
		},
		StringData: map[string]string{
			"host":     outputs.Host,
			"port":     fmt.Sprintf("%d", outputs.Port),
			"username": outputs.Username,
			"password": outputs.Password,
			"database": outputs.Database,
			"uri":      outputs.URI,
		},
	}

	existing, err := m.k8sClient.Clientset.CoreV1().Secrets(m.k8sClient.Namespace).Get(ctx, serviceSecretPrefix+name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = m.k8sClient.Clientset.CoreV1().Secrets(m.k8sClient.Namespace).Create(ctx, secret, metav1.CreateOptions{})
			return err
		}
		return err
	}
	existing.StringData = secret.StringData
	_, err = m.k8sClient.Clientset.CoreV1().Secrets(m.k8sClient.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

// Delete removes a service instance and its secret.
func (m *Manager) Delete(ctx context.Context, name string) error {
	// Delete ConfigMap
	_ = m.k8sClient.Clientset.CoreV1().ConfigMaps(m.k8sClient.Namespace).Delete(ctx, serviceConfigMapPrefix+name, metav1.DeleteOptions{})
	// Delete Secret
	_ = m.k8sClient.Clientset.CoreV1().Secrets(m.k8sClient.Namespace).Delete(ctx, serviceSecretPrefix+name, metav1.DeleteOptions{})
	return nil
}

// AddBinding adds a binding to a service instance.
func (m *Manager) AddBinding(ctx context.Context, serviceName, appName string) error {
	inst, err := m.Get(ctx, serviceName)
	if err != nil {
		return err
	}

	// Check not already bound
	for _, b := range inst.Bindings {
		if b.AppName == appName {
			return fmt.Errorf("app %q already bound to service %q", appName, serviceName)
		}
	}

	inst.Bindings = append(inst.Bindings, models.ServiceBinding{
		AppName:   appName,
		SecretRef: serviceSecretPrefix + serviceName,
		BoundAt:   time.Now(),
	})
	inst.UpdatedAt = time.Now()
	return m.save(ctx, inst)
}

// RemoveBinding removes a binding from a service instance.
func (m *Manager) RemoveBinding(ctx context.Context, serviceName, appName string) error {
	inst, err := m.Get(ctx, serviceName)
	if err != nil {
		return err
	}

	var updated []models.ServiceBinding
	found := false
	for _, b := range inst.Bindings {
		if b.AppName == appName {
			found = true
			continue
		}
		updated = append(updated, b)
	}
	if !found {
		return fmt.Errorf("app %q not bound to service %q", appName, serviceName)
	}

	inst.Bindings = updated
	inst.UpdatedAt = time.Now()
	return m.save(ctx, inst)
}

func (m *Manager) save(ctx context.Context, inst *models.ServiceInstance) error {
	data, err := json.Marshal(inst)
	if err != nil {
		return err
	}

	cm, err := m.k8sClient.Clientset.CoreV1().ConfigMaps(m.k8sClient.Namespace).Get(ctx, serviceConfigMapPrefix+inst.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	cm.Data["instance"] = string(data)
	_, err = m.k8sClient.Clientset.CoreV1().ConfigMaps(m.k8sClient.Namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}
