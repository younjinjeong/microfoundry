package secrets

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/younjinjeong/microfoundry/pkg/k8s"
	"github.com/younjinjeong/microfoundry/pkg/models"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	labelManagedBy       = "microfoundry"
	labelManagedByKey    = "app.kubernetes.io/managed-by"
	labelSecretType      = "microfoundry.io/secret-type"
	labelServiceInstance = "microfoundry.io/service-instance"
	annotationCreatedBy  = "microfoundry.io/created-by"
	annotationDesc       = "microfoundry.io/description"
)

// Manager provides a vault-like interface for managing K8s Secrets.
type Manager struct {
	k8sClient *k8s.Client
}

// NewManager creates a new secrets manager.
func NewManager(client *k8s.Client) *Manager {
	return &Manager{k8sClient: client}
}

// List returns all MicroFoundry-managed secrets.
func (m *Manager) List(ctx context.Context) ([]models.SecretListItem, error) {
	secrets, err := m.k8sClient.Clientset.CoreV1().Secrets(m.k8sClient.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelManagedByKey + "=" + labelManagedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("listing secrets: %w", err)
	}

	var items []models.SecretListItem
	for _, s := range secrets.Items {
		// Only include MicroFoundry-managed secrets (mf-svc-* or mf-secret-*)
		if !strings.HasPrefix(s.Name, models.ServiceSecretPrefix) &&
			!strings.HasPrefix(s.Name, models.UserSecretPrefix) {
			continue
		}

		item := models.SecretListItem{
			K8sName:  s.Name,
			KeyCount: len(s.Data),
		}

		// Determine logical name and type
		if strings.HasPrefix(s.Name, models.ServiceSecretPrefix) {
			item.Name = strings.TrimPrefix(s.Name, models.ServiceSecretPrefix)
			item.Type = models.SecretTypeService
			item.ServiceInstance = s.Labels[labelServiceInstance]
		} else {
			item.Name = strings.TrimPrefix(s.Name, models.UserSecretPrefix)
			item.Type = models.SecretTypeUser
		}

		// Override type from label if present (for forward compatibility)
		if t, ok := s.Labels[labelSecretType]; ok {
			item.Type = t
		}

		// Read annotations
		item.Description = s.Annotations[annotationDesc]

		// CreatedAt from K8s metadata
		item.CreatedAt = s.CreationTimestamp.Time

		// Count bound apps
		boundApps, err := m.GetBoundApps(ctx, s.Name)
		if err != nil {
			log.Printf("warning: could not get bound apps for secret %q: %v", s.Name, err)
		}
		item.BoundApps = len(boundApps)

		items = append(items, item)
	}

	// Sort by creation time descending (newest first)
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	return items, nil
}

// Get returns secret metadata, key names (no values), and bound apps.
func (m *Manager) Get(ctx context.Context, name string) (*models.SecretDetail, error) {
	k8sName, err := m.resolveK8sName(ctx, name)
	if err != nil {
		return nil, err
	}

	secret, err := m.k8sClient.Clientset.CoreV1().Secrets(m.k8sClient.Namespace).Get(ctx, k8sName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("secret %q not found", name)
		}
		return nil, err
	}

	detail := &models.SecretDetail{
		K8sName:   secret.Name,
		KeyCount:  len(secret.Data),
		CreatedAt: secret.CreationTimestamp.Time,
	}

	// Determine logical name and type
	if strings.HasPrefix(secret.Name, models.ServiceSecretPrefix) {
		detail.Name = strings.TrimPrefix(secret.Name, models.ServiceSecretPrefix)
		detail.Type = models.SecretTypeService
		detail.ServiceInstance = secret.Labels[labelServiceInstance]
	} else if strings.HasPrefix(secret.Name, models.UserSecretPrefix) {
		detail.Name = strings.TrimPrefix(secret.Name, models.UserSecretPrefix)
		detail.Type = models.SecretTypeUser
	} else {
		detail.Name = secret.Name
		detail.Type = models.SecretTypeUser
	}

	// Override type from label if present
	if t, ok := secret.Labels[labelSecretType]; ok {
		detail.Type = t
	}

	detail.Description = secret.Annotations[annotationDesc]

	// Collect key names (sorted)
	keys := make([]string, 0, len(secret.Data))
	for k := range secret.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	detail.Keys = keys

	// Bound apps
	boundApps, err := m.GetBoundApps(ctx, secret.Name)
	if err != nil {
		log.Printf("warning: could not get bound apps for secret %q: %v", secret.Name, err)
	}
	detail.BoundApps = boundApps

	return detail, nil
}

// GetValue returns the decoded value of a single key in a secret.
func (m *Manager) GetValue(ctx context.Context, name, key string) (string, error) {
	k8sName, err := m.resolveK8sName(ctx, name)
	if err != nil {
		return "", err
	}

	secret, err := m.k8sClient.Clientset.CoreV1().Secrets(m.k8sClient.Namespace).Get(ctx, k8sName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting secret %q: %w", name, err)
	}

	val, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret %q", key, name)
	}

	return string(val), nil
}

// CreateServiceSecret creates a K8s Secret for a service instance with proper labels.
func (m *Manager) CreateServiceSecret(ctx context.Context, serviceName string, data map[string]string) error {
	k8sName := models.ServiceSecretPrefix + serviceName

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: k8sName,
			Labels: map[string]string{
				labelManagedByKey:    labelManagedBy,
				labelSecretType:     models.SecretTypeService,
				labelServiceInstance: serviceName,
			},
			Annotations: map[string]string{
				annotationCreatedBy: "create-service",
			},
		},
		StringData: data,
	}

	existing, err := m.k8sClient.Clientset.CoreV1().Secrets(m.k8sClient.Namespace).Get(ctx, k8sName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = m.k8sClient.Clientset.CoreV1().Secrets(m.k8sClient.Namespace).Create(ctx, secret, metav1.CreateOptions{})
			return err
		}
		return err
	}

	// Update existing secret
	existing.StringData = data
	// Ensure labels are set (for backward compatibility migration)
	if existing.Labels == nil {
		existing.Labels = make(map[string]string)
	}
	existing.Labels[labelManagedByKey] = labelManagedBy
	existing.Labels[labelSecretType] = models.SecretTypeService
	existing.Labels[labelServiceInstance] = serviceName
	if existing.Annotations == nil {
		existing.Annotations = make(map[string]string)
	}
	existing.Annotations[annotationCreatedBy] = "create-service"

	_, err = m.k8sClient.Clientset.CoreV1().Secrets(m.k8sClient.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

// CreateUserSecret creates a user-defined K8s Secret.
func (m *Manager) CreateUserSecret(ctx context.Context, name, description string, data map[string]string) error {
	k8sName := models.UserSecretPrefix + name

	labels := map[string]string{
		labelManagedByKey: labelManagedBy,
		labelSecretType:  models.SecretTypeUser,
	}

	annotations := map[string]string{
		annotationCreatedBy: "admin-ui",
	}
	if description != "" {
		annotations[annotationDesc] = description
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        k8sName,
			Labels:      labels,
			Annotations: annotations,
		},
		StringData: data,
	}

	_, err := m.k8sClient.Clientset.CoreV1().Secrets(m.k8sClient.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return fmt.Errorf("secret %q already exists", name)
		}
		return err
	}
	return nil
}

// Delete removes a K8s Secret by its logical name.
func (m *Manager) Delete(ctx context.Context, name string) error {
	k8sName, err := m.resolveK8sName(ctx, name)
	if err != nil {
		return err
	}

	err = m.k8sClient.Clientset.CoreV1().Secrets(m.k8sClient.Namespace).Delete(ctx, k8sName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("deleting secret %q: %w", name, err)
	}
	return nil
}

// GetBoundApps lists Deployments whose envFrom references the given K8s Secret name.
func (m *Manager) GetBoundApps(ctx context.Context, k8sSecretName string) ([]string, error) {
	deployments, err := m.k8sClient.Clientset.AppsV1().Deployments(m.k8sClient.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing deployments: %w", err)
	}

	var apps []string
	for _, d := range deployments.Items {
		for _, c := range d.Spec.Template.Spec.Containers {
			for _, ef := range c.EnvFrom {
				if ef.SecretRef != nil && ef.SecretRef.Name == k8sSecretName {
					apps = append(apps, d.Name)
					break
				}
			}
		}
	}

	sort.Strings(apps)
	return apps, nil
}

// resolveK8sName maps a logical name to the actual K8s Secret name.
// It tries the service prefix first, then user prefix, then the raw name.
func (m *Manager) resolveK8sName(ctx context.Context, name string) (string, error) {
	// If name already has a known prefix, use it directly
	if strings.HasPrefix(name, models.ServiceSecretPrefix) || strings.HasPrefix(name, models.UserSecretPrefix) {
		return name, nil
	}

	// Try service secret first
	svcName := models.ServiceSecretPrefix + name
	_, err := m.k8sClient.Clientset.CoreV1().Secrets(m.k8sClient.Namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err == nil {
		return svcName, nil
	}

	// Try user-defined secret
	userName := models.UserSecretPrefix + name
	_, err = m.k8sClient.Clientset.CoreV1().Secrets(m.k8sClient.Namespace).Get(ctx, userName, metav1.GetOptions{})
	if err == nil {
		return userName, nil
	}

	return "", fmt.Errorf("secret %q not found (tried %s and %s)", name, svcName, userName)
}
