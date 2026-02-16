package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/younjinjeong/microfoundry/pkg/k8s"
	"github.com/younjinjeong/microfoundry/pkg/secrets"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Binder handles injecting service credentials into app deployments.
type Binder struct {
	k8sClient  *k8s.Client
	mgr        *Manager
	secretsMgr *secrets.Manager
}

// NewBinder creates a new binder with VCAP_SERVICES support.
func NewBinder(client *k8s.Client, mgr *Manager) *Binder {
	return &Binder{
		k8sClient:  client,
		mgr:        mgr,
		secretsMgr: mgr.SecretsManager(),
	}
}

// Bind injects service credentials from a K8s Secret into an app's Deployment as env vars
// and builds/updates the VCAP_SERVICES env var. Retries on conflict.
func (b *Binder) Bind(ctx context.Context, appName, serviceName string) error {
	secretName := SecretName(serviceName)

	for i := 0; i < maxRetries; i++ {
		deploy, err := b.k8sClient.Clientset.AppsV1().Deployments(b.k8sClient.Namespace).Get(ctx, appName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("deployment %q not found: %w", appName, err)
		}

		containers := deploy.Spec.Template.Spec.Containers
		if len(containers) == 0 {
			return fmt.Errorf("deployment %q has no containers", appName)
		}

		// Add envFrom reference to the service secret (idempotent)
		envFrom := corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			},
		}

		alreadyBound := false
		for _, ef := range containers[0].EnvFrom {
			if ef.SecretRef != nil && ef.SecretRef.Name == secretName {
				alreadyBound = true
				break
			}
		}
		if !alreadyBound {
			deploy.Spec.Template.Spec.Containers[0].EnvFrom = append(
				deploy.Spec.Template.Spec.Containers[0].EnvFrom,
				envFrom,
			)
		}

		// Update bound services annotation
		if deploy.Spec.Template.Annotations == nil {
			deploy.Spec.Template.Annotations = make(map[string]string)
		}
		boundServices := parseBoundServices(deploy.Spec.Template.Annotations["microfoundry.io/bound-services"])
		if !contains(boundServices, serviceName) {
			boundServices = append(boundServices, serviceName)
		}
		deploy.Spec.Template.Annotations["microfoundry.io/bound-services"] = strings.Join(boundServices, ",")

		// Build VCAP_SERVICES from all bound services
		vcapJSON, err := BuildVCAPServices(ctx, b.mgr, b.secretsMgr, boundServices)
		if err != nil {
			return fmt.Errorf("building VCAP_SERVICES: %w", err)
		}
		setEnvVar(&deploy.Spec.Template.Spec.Containers[0], "VCAP_SERVICES", vcapJSON)

		_, err = b.k8sClient.Clientset.AppsV1().Deployments(b.k8sClient.Namespace).Update(ctx, deploy, metav1.UpdateOptions{})
		if err == nil {
			return nil
		}
		if !errors.IsConflict(err) {
			return err
		}
	}
	return fmt.Errorf("conflict: too many retries updating deployment %q", appName)
}

// Unbind removes service credentials from an app's Deployment and rebuilds VCAP_SERVICES.
// Retries on conflict.
func (b *Binder) Unbind(ctx context.Context, appName, serviceName string) error {
	secretName := SecretName(serviceName)

	for i := 0; i < maxRetries; i++ {
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

		// Update bound services annotation
		if deploy.Spec.Template.Annotations == nil {
			deploy.Spec.Template.Annotations = make(map[string]string)
		}
		boundServices := parseBoundServices(deploy.Spec.Template.Annotations["microfoundry.io/bound-services"])
		var remaining []string
		for _, s := range boundServices {
			if s != serviceName {
				remaining = append(remaining, s)
			}
		}

		if len(remaining) > 0 {
			deploy.Spec.Template.Annotations["microfoundry.io/bound-services"] = strings.Join(remaining, ",")
			vcapJSON, err := BuildVCAPServices(ctx, b.mgr, b.secretsMgr, remaining)
			if err != nil {
				return fmt.Errorf("building VCAP_SERVICES: %w", err)
			}
			setEnvVar(&deploy.Spec.Template.Spec.Containers[0], "VCAP_SERVICES", vcapJSON)
		} else {
			delete(deploy.Spec.Template.Annotations, "microfoundry.io/bound-services")
			removeEnvVar(&deploy.Spec.Template.Spec.Containers[0], "VCAP_SERVICES")
		}

		_, err = b.k8sClient.Clientset.AppsV1().Deployments(b.k8sClient.Namespace).Update(ctx, deploy, metav1.UpdateOptions{})
		if err == nil {
			return nil
		}
		if !errors.IsConflict(err) {
			return err
		}
	}
	return fmt.Errorf("conflict: too many retries updating deployment %q", appName)
}

func parseBoundServices(annotation string) []string {
	if annotation == "" {
		return nil
	}
	var services []string
	for _, s := range strings.Split(annotation, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			s = strings.TrimPrefix(s, "mf-svc-")
			services = append(services, s)
		}
	}
	return services
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func setEnvVar(container *corev1.Container, name, value string) {
	for i, e := range container.Env {
		if e.Name == name {
			container.Env[i].Value = value
			return
		}
	}
	container.Env = append(container.Env, corev1.EnvVar{Name: name, Value: value})
}

func removeEnvVar(container *corev1.Container, name string) {
	var updated []corev1.EnvVar
	for _, e := range container.Env {
		if e.Name != name {
			updated = append(updated, e)
		}
	}
	container.Env = updated
}
