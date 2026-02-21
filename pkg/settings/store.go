package settings

import (
	"context"
	"encoding/json"

	"github.com/younjinjeong/microfoundry/pkg/k8s"
	"github.com/younjinjeong/microfoundry/pkg/models"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Store provides ConfigMap/Secret-backed persistence for platform settings.
type Store struct {
	k8sClient *k8s.Client
}

// NewStore creates a new settings store.
func NewStore(client *k8s.Client) *Store {
	return &Store{k8sClient: client}
}

// Get returns the current platform settings from the ConfigMap.
func (s *Store) Get(ctx context.Context) (*models.PlatformSettings, error) {
	cmAPI := s.k8sClient.Clientset.CoreV1().ConfigMaps(s.k8sClient.Namespace)
	cm, err := cmAPI.Get(ctx, models.PlatformSettingsConfigMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return &models.PlatformSettings{}, nil
		}
		return nil, err
	}

	settings := &models.PlatformSettings{}
	if data, ok := cm.Data["settings"]; ok {
		if err := json.Unmarshal([]byte(data), settings); err != nil {
			return &models.PlatformSettings{}, nil
		}
	}
	return settings, nil
}

// SaveRegistry saves registry configuration. Password goes to Secret, rest to ConfigMap.
func (s *Store) SaveRegistry(ctx context.Context, cfg models.RegistryConfig, password string) error {
	if err := s.update(ctx, func(settings *models.PlatformSettings) {
		settings.Registry = cfg
	}); err != nil {
		return err
	}

	if password != "" {
		return s.updateSecret(ctx, "registry-password", password)
	}
	return nil
}

// SaveSMTP saves SMTP configuration. Password goes to Secret, rest to ConfigMap.
func (s *Store) SaveSMTP(ctx context.Context, cfg models.SMTPConfig, password string) error {
	if err := s.update(ctx, func(settings *models.PlatformSettings) {
		settings.SMTP = cfg
	}); err != nil {
		return err
	}

	if password != "" {
		return s.updateSecret(ctx, "smtp-password", password)
	}
	return nil
}

// AddWebhook adds a single webhook to the list.
func (s *Store) AddWebhook(ctx context.Context, wh models.WebhookConfig) error {
	return s.update(ctx, func(settings *models.PlatformSettings) {
		settings.Webhooks = append(settings.Webhooks, wh)
	})
}

// DeleteWebhook removes a webhook by ID.
func (s *Store) DeleteWebhook(ctx context.Context, id string) error {
	return s.update(ctx, func(settings *models.PlatformSettings) {
		filtered := make([]models.WebhookConfig, 0, len(settings.Webhooks))
		for _, wh := range settings.Webhooks {
			if wh.ID != id {
				filtered = append(filtered, wh)
			}
		}
		settings.Webhooks = filtered
	})
}

// SaveEndpoints saves service endpoint URL overrides to the ConfigMap.
func (s *Store) SaveEndpoints(ctx context.Context, cfg models.EndpointsConfig) error {
	return s.update(ctx, func(settings *models.PlatformSettings) {
		settings.Endpoints = cfg
	})
}

// SaveCloudProviders saves cloud provider configuration. Secrets go to Secret, rest to ConfigMap.
func (s *Store) SaveCloudProviders(ctx context.Context, cfg models.CloudProviderConfig, secrets map[string]string) error {
	if err := s.update(ctx, func(settings *models.PlatformSettings) {
		settings.CloudProviders = cfg
	}); err != nil {
		return err
	}

	// Store sensitive credentials in K8s Secret
	for key, value := range secrets {
		if value != "" {
			if err := s.updateSecret(ctx, key, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetCredential retrieves a single credential from the platform Secret.
func (s *Store) GetCredential(ctx context.Context, key string) (string, error) {
	secAPI := s.k8sClient.Clientset.CoreV1().Secrets(s.k8sClient.Namespace)
	sec, err := secAPI.Get(ctx, models.PlatformCredentialsSecretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}
	return string(sec.Data[key]), nil
}

// update applies a mutation function to the platform settings with optimistic concurrency.
func (s *Store) update(ctx context.Context, mutate func(*models.PlatformSettings)) error {
	cmAPI := s.k8sClient.Clientset.CoreV1().ConfigMaps(s.k8sClient.Namespace)

	cm, err := cmAPI.Get(ctx, models.PlatformSettingsConfigMapName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		// Create new ConfigMap
		settings := &models.PlatformSettings{}
		mutate(settings)
		data, _ := json.Marshal(settings)
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      models.PlatformSettingsConfigMapName,
				Namespace: s.k8sClient.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "microfoundry",
					"microfoundry.io/component":    "platform-settings",
				},
			},
			Data: map[string]string{
				"settings": string(data),
			},
		}
		_, err = cmAPI.Create(ctx, cm, metav1.CreateOptions{})
		return err
	}

	// Update existing ConfigMap
	settings := &models.PlatformSettings{}
	if raw, ok := cm.Data["settings"]; ok {
		json.Unmarshal([]byte(raw), settings)
	}
	mutate(settings)
	data, _ := json.Marshal(settings)
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data["settings"] = string(data)
	_, err = cmAPI.Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

// updateSecret creates or updates a key in the platform credentials Secret.
func (s *Store) updateSecret(ctx context.Context, key, value string) error {
	secAPI := s.k8sClient.Clientset.CoreV1().Secrets(s.k8sClient.Namespace)

	sec, err := secAPI.Get(ctx, models.PlatformCredentialsSecretName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		// Create new Secret
		sec = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      models.PlatformCredentialsSecretName,
				Namespace: s.k8sClient.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "microfoundry",
					"microfoundry.io/component":    "platform-credentials",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				key: []byte(value),
			},
		}
		_, err = secAPI.Create(ctx, sec, metav1.CreateOptions{})
		return err
	}

	// Update existing Secret
	if sec.Data == nil {
		sec.Data = make(map[string][]byte)
	}
	sec.Data[key] = []byte(value)
	_, err = secAPI.Update(ctx, sec, metav1.UpdateOptions{})
	return err
}
