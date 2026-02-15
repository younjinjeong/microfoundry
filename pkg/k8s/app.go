package k8s

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/younjinjeong/microfoundry/pkg/models"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const labelManagedBy = "microfoundry"

func appLabels(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       name,
		"app.kubernetes.io/managed-by": labelManagedBy,
	}
}

// DeployApp creates or updates a K8s Deployment + Service for the given app.
func (c *Client) DeployApp(ctx context.Context, app models.App, routes []models.Route) error {
	labels := appLabels(app.Name)
	replicas := int32(app.Instances)

	// Build environment variables
	var envVars []corev1.EnvVar
	envVars = append(envVars, corev1.EnvVar{Name: "PORT", Value: fmt.Sprintf("%d", app.Port)})
	for k, v := range app.Env {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}

	// Build probes
	var readinessProbe *corev1.Probe
	switch app.HealthCheck.Type {
	case models.HealthCheckHTTP:
		endpoint := app.HealthCheck.Endpoint
		if endpoint == "" {
			endpoint = "/"
		}
		readinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: endpoint,
					Port: intstr.FromInt32(int32(app.Port)),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		}
	case models.HealthCheckPort, "":
		readinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt32(int32(app.Port)),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		}
	}

	// Container command
	var command []string
	if app.Command != "" {
		command = []string{"/bin/sh", "-c", app.Command}
	}

	container := corev1.Container{
		Name:            "app",
		Image:           app.ImageRef,
		Ports:           []corev1.ContainerPort{{ContainerPort: int32(app.Port)}},
		Env:             envVars,
		Command:         command,
		ReadinessProbe:  readinessProbe,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", app.MemoryMB/2)),
				corev1.ResourceCPU:    resource.MustParse("100m"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", app.MemoryMB)),
			},
		},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: c.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{container}},
			},
		},
	}

	// Create or update Deployment
	existing, err := c.Clientset.AppsV1().Deployments(c.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = c.Clientset.AppsV1().Deployments(c.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	} else if err == nil {
		deployment.ResourceVersion = existing.ResourceVersion
		_, err = c.Clientset.AppsV1().Deployments(c.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	}
	if err != nil {
		return fmt.Errorf("deploying app %s: %w", app.Name, err)
	}

	// Create or update Service
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: c.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{Port: 80, TargetPort: intstr.FromInt32(int32(app.Port))},
			},
		},
	}

	existingSvc, err := c.Clientset.CoreV1().Services(c.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = c.Clientset.CoreV1().Services(c.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	} else if err == nil {
		svc.ResourceVersion = existingSvc.ResourceVersion
		svc.Spec.ClusterIP = existingSvc.Spec.ClusterIP
		_, err = c.Clientset.CoreV1().Services(c.Namespace).Update(ctx, svc, metav1.UpdateOptions{})
	}
	if err != nil {
		return fmt.Errorf("creating service for %s: %w", app.Name, err)
	}

	return nil
}

// DeleteApp removes the Deployment, Service, and Ingress for an app.
func (c *Client) DeleteApp(ctx context.Context, name string) error {
	propagation := metav1.DeletePropagationForeground
	opts := metav1.DeleteOptions{PropagationPolicy: &propagation}

	_ = c.DeleteIngress(ctx, name)
	_ = c.Clientset.CoreV1().Services(c.Namespace).Delete(ctx, name, opts)
	err := c.Clientset.AppsV1().Deployments(c.Namespace).Delete(ctx, name, opts)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("deleting app %s: %w", name, err)
	}
	return nil
}

// ScaleApp updates the replica count for an app.
func (c *Client) ScaleApp(ctx context.Context, name string, instances int) error {
	scale, err := c.Clientset.AppsV1().Deployments(c.Namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting scale for %s: %w", name, err)
	}
	scale.Spec.Replicas = int32(instances)
	_, err = c.Clientset.AppsV1().Deployments(c.Namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("scaling %s: %w", name, err)
	}
	return nil
}

// GetAppStatus returns the live status of an app from K8s.
func (c *Client) GetAppStatus(ctx context.Context, name string) (*models.AppStatus, error) {
	dep, err := c.Clientset.AppsV1().Deployments(c.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting deployment %s: %w", name, err)
	}

	pods, err := c.Clientset.CoreV1().Pods(c.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", name),
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods for %s: %w", name, err)
	}

	var instances []models.InstanceStatus
	running := 0
	for i, pod := range pods.Items {
		state := "DOWN"
		var since time.Time
		if len(pod.Status.ContainerStatuses) > 0 {
			cs := pod.Status.ContainerStatuses[0]
			if cs.State.Running != nil {
				state = "RUNNING"
				since = cs.State.Running.StartedAt.Time
				running++
			} else if cs.State.Waiting != nil {
				state = "STARTING"
			} else if cs.State.Terminated != nil {
				state = "CRASHED"
			}
		}
		instances = append(instances, models.InstanceStatus{
			Index: i,
			State: state,
			Since: since,
		})
	}

	status := &models.AppStatus{
		RunningCount: running,
		TotalCount:   int(*dep.Spec.Replicas),
		Instances:    instances,
	}
	return status, nil
}

// ListApps returns all MicroFoundry-managed deployments.
func (c *Client) ListApps(ctx context.Context) ([]string, error) {
	deps, err := c.Clientset.AppsV1().Deployments(c.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=" + labelManagedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("listing apps: %w", err)
	}
	var names []string
	for _, d := range deps.Items {
		names = append(names, d.Name)
	}
	return names, nil
}

// GetAppLogs returns a log stream for an app's pods.
func (c *Client) GetAppLogs(ctx context.Context, name string, follow bool) (io.ReadCloser, error) {
	pods, err := c.Clientset.CoreV1().Pods(c.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", name),
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods for %s: %w", name, err)
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods found for app %s", name)
	}

	// Stream logs from first pod
	opts := &corev1.PodLogOptions{
		Follow:    follow,
		Container: "app",
	}
	return c.Clientset.CoreV1().Pods(c.Namespace).GetLogs(pods.Items[0].Name, opts).Stream(ctx)
}

// WaitForRollout waits until all pods are ready or timeout.
func (c *Client) WaitForRollout(ctx context.Context, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		dep, err := c.Clientset.AppsV1().Deployments(c.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if dep.Status.ReadyReplicas == *dep.Spec.Replicas {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timeout waiting for %s rollout", name)
}
