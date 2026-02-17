package k8s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const imagePullSecretName = "mf-registry-pull"

// EnsureImagePullSecret creates or updates the Docker registry pull secret
// in the current namespace. This is required for K8s to pull images from
// private registries like Harbor.
func (c *Client) EnsureImagePullSecret(ctx context.Context, registry, username, password string) error {
	// Build .dockerconfigjson payload
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	dockerConfig := map[string]any{
		"auths": map[string]any{
			registry: map[string]string{
				"username": username,
				"password": password,
				"auth":     auth,
			},
		},
	}

	configJSON, err := json.Marshal(dockerConfig)
	if err != nil {
		return fmt.Errorf("encoding docker config: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      imagePullSecretName,
			Namespace: c.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "microfoundry",
				"microfoundry.io/component":    "registry-pull-secret",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: configJSON,
		},
	}

	secAPI := c.Clientset.CoreV1().Secrets(c.Namespace)
	existing, err := secAPI.Get(ctx, imagePullSecretName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = secAPI.Create(ctx, secret, metav1.CreateOptions{})
		return err
	} else if err != nil {
		return err
	}

	secret.ResourceVersion = existing.ResourceVersion
	_, err = secAPI.Update(ctx, secret, metav1.UpdateOptions{})
	return err
}

// HasImagePullSecret checks if the registry pull secret exists in the namespace.
func (c *Client) HasImagePullSecret(ctx context.Context) bool {
	_, err := c.Clientset.CoreV1().Secrets(c.Namespace).Get(ctx, imagePullSecretName, metav1.GetOptions{})
	return err == nil
}

// ImagePullSecretName returns the name of the registry pull secret.
func ImagePullSecretName() string {
	return imagePullSecretName
}

// imagePullSecrets returns the imagePullSecrets list for PodSpec if the
// registry pull secret exists in the namespace. Returns nil otherwise.
func (c *Client) imagePullSecrets(ctx context.Context) []corev1.LocalObjectReference {
	if c.HasImagePullSecret(ctx) {
		return []corev1.LocalObjectReference{{Name: imagePullSecretName}}
	}
	return nil
}
