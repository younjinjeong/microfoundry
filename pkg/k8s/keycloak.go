package k8s

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	keycloakName  = "keycloak"
	keycloakImage = "quay.io/keycloak/keycloak:26.0"
	keycloakPort  = 8180
)

// DeployKeycloak creates a Keycloak Deployment and Service in the cluster namespace.
func (c *Client) DeployKeycloak(ctx context.Context, adminUser, adminPass string) error {
	labels := map[string]string{
		"app":                    keycloakName,
		"microfoundry.io/system": "true",
	}

	replicas := int32(1)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      keycloakName,
			Namespace: c.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  keycloakName,
						Image: keycloakImage,
						Args:  []string{"start-dev", "--http-port=8180"},
						Ports: []corev1.ContainerPort{{
							ContainerPort: int32(keycloakPort),
							Protocol:      corev1.ProtocolTCP,
						}},
						Env: []corev1.EnvVar{
							{Name: "KEYCLOAK_ADMIN", Value: adminUser},
							{Name: "KEYCLOAK_ADMIN_PASSWORD", Value: adminPass},
							{Name: "KC_PROXY_HEADERS", Value: "xforwarded"},
							{Name: "KC_HTTP_PORT", Value: "8180"},
							{Name: "KC_HOSTNAME_STRICT", Value: "false"},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/health/ready",
									Port: intstr.FromInt32(int32(keycloakPort)),
								},
							},
							InitialDelaySeconds: 30,
							PeriodSeconds:       10,
						},
					}},
				},
			},
		},
	}

	existing, err := c.Clientset.AppsV1().Deployments(c.Namespace).Get(ctx, keycloakName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("checking keycloak deployment: %w", err)
		}
		_, err = c.Clientset.AppsV1().Deployments(c.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("creating keycloak deployment: %w", err)
		}
	} else {
		deployment.ResourceVersion = existing.ResourceVersion
		_, err = c.Clientset.AppsV1().Deployments(c.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("updating keycloak deployment: %w", err)
		}
	}

	// Create Service
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      keycloakName,
			Namespace: c.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Port:       int32(keycloakPort),
				TargetPort: intstr.FromInt32(int32(keycloakPort)),
				Protocol:   corev1.ProtocolTCP,
			}},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	existingSvc, err := c.Clientset.CoreV1().Services(c.Namespace).Get(ctx, keycloakName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("checking keycloak service: %w", err)
		}
		_, err = c.Clientset.CoreV1().Services(c.Namespace).Create(ctx, svc, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("creating keycloak service: %w", err)
		}
	} else {
		svc.ResourceVersion = existingSvc.ResourceVersion
		svc.Spec.ClusterIP = existingSvc.Spec.ClusterIP
		_, err = c.Clientset.CoreV1().Services(c.Namespace).Update(ctx, svc, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("updating keycloak service: %w", err)
		}
	}

	return nil
}

// IsKeycloakReady checks if the Keycloak deployment has at least one ready replica.
func (c *Client) IsKeycloakReady(ctx context.Context) (bool, error) {
	deploy, err := c.Clientset.AppsV1().Deployments(c.Namespace).Get(ctx, keycloakName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	return deploy.Status.ReadyReplicas > 0, nil
}

// GetKeycloakInternalURL returns the cluster-internal URL for Keycloak.
func (c *Client) GetKeycloakInternalURL() string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", keycloakName, c.Namespace, keycloakPort)
}

// KeycloakPort returns the port Keycloak listens on (for port-forwarding).
func (c *Client) KeycloakPort() int {
	return keycloakPort
}
