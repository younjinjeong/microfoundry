package service

import (
	"context"
	"fmt"

	"github.com/younjinjeong/microfoundry/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Binder handles injecting service credentials into app deployments.
type Binder struct {
	k8sClient *k8s.Client
}

// NewBinder creates a new binder.
func NewBinder(client *k8s.Client) *Binder {
	return &Binder{k8sClient: client}
}

// Bind injects service credentials from a K8s Secret into an app's Deployment as env vars.
func (b *Binder) Bind(ctx context.Context, appName, secretName string) error {
	deploy, err := b.k8sClient.Clientset.AppsV1().Deployments(b.k8sClient.Namespace).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("deployment %q not found: %w", appName, err)
	}

	// Add envFrom reference to the service secret
	envFrom := corev1.EnvFromSource{
		SecretRef: &corev1.SecretEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
		},
	}

	// Check if already bound
	containers := deploy.Spec.Template.Spec.Containers
	if len(containers) == 0 {
		return fmt.Errorf("deployment %q has no containers", appName)
	}

	for _, ef := range containers[0].EnvFrom {
		if ef.SecretRef != nil && ef.SecretRef.Name == secretName {
			return nil // Already bound
		}
	}

	deploy.Spec.Template.Spec.Containers[0].EnvFrom = append(
		deploy.Spec.Template.Spec.Containers[0].EnvFrom,
		envFrom,
	)

	// Add annotation
	if deploy.Spec.Template.Annotations == nil {
		deploy.Spec.Template.Annotations = make(map[string]string)
	}
	existing := deploy.Spec.Template.Annotations["microfoundry.io/bound-services"]
	if existing != "" {
		existing += ","
	}
	deploy.Spec.Template.Annotations["microfoundry.io/bound-services"] = existing + secretName

	_, err = b.k8sClient.Clientset.AppsV1().Deployments(b.k8sClient.Namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	return err
}

// Unbind removes service credentials from an app's Deployment.
func (b *Binder) Unbind(ctx context.Context, appName, secretName string) error {
	deploy, err := b.k8sClient.Clientset.AppsV1().Deployments(b.k8sClient.Namespace).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("deployment %q not found: %w", appName, err)
	}

	containers := deploy.Spec.Template.Spec.Containers
	if len(containers) == 0 {
		return nil
	}

	// Remove envFrom reference
	var updated []corev1.EnvFromSource
	for _, ef := range containers[0].EnvFrom {
		if ef.SecretRef != nil && ef.SecretRef.Name == secretName {
			continue
		}
		updated = append(updated, ef)
	}
	deploy.Spec.Template.Spec.Containers[0].EnvFrom = updated

	_, err = b.k8sClient.Clientset.AppsV1().Deployments(b.k8sClient.Namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	return err
}
