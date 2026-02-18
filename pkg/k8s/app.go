package k8s

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
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

	// Use PullAlways when image references a remote registry (contains a '/' with a dot/colon prefix)
	pullPolicy := corev1.PullIfNotPresent
	if strings.Contains(app.ImageRef, ".") || strings.Contains(app.ImageRef, ":") {
		// If imageRef looks like "harbor.local:30003/project/app:latest", use PullAlways
		parts := strings.SplitN(app.ImageRef, "/", 2)
		if len(parts) > 1 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":")) {
			pullPolicy = corev1.PullAlways
		}
	}

	container := corev1.Container{
		Name:            "app",
		Image:           app.ImageRef,
		Ports:           []corev1.ContainerPort{{ContainerPort: int32(app.Port)}},
		Env:             envVars,
		Command:         command,
		ReadinessProbe:  readinessProbe,
		ImagePullPolicy: pullPolicy,
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

	// Build annotations for metadata
	annotations := map[string]string{
		"microfoundry.io/lifecycle":   string(app.LifecycleType),
		"microfoundry.io/disk-mb":    fmt.Sprintf("%d", app.DiskMB),
		"microfoundry.io/port":       fmt.Sprintf("%d", app.Port),
		"microfoundry.io/created-at": app.CreatedAt.Format(time.RFC3339),
	}
	if app.GUID != "" {
		annotations["microfoundry.io/guid"] = app.GUID
	}
	if len(app.Buildpacks) > 0 {
		annotations["microfoundry.io/buildpacks"] = strings.Join(app.Buildpacks, ",")
	}
	if owner, ok := app.Env["MICROFOUNDRY_OWNER"]; ok {
		annotations["microfoundry.io/owner"] = owner
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        app.Name,
			Namespace:   c.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"microfoundry.io/observable": "true",
						"prometheus.io/scrape":       "true",
						"prometheus.io/port":         fmt.Sprintf("%d", app.Port),
						"prometheus.io/path":         "/metrics",
					},
				},
				Spec: corev1.PodSpec{
					Containers:       []corev1.Container{container},
					ImagePullSecrets: c.imagePullSecrets(ctx),
				},
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

	// Enable session affinity for WebSocket routes (sticky sessions)
	if models.HasSpecialProtocol(routes) == "websocket" {
		svc.Spec.SessionAffinity = corev1.ServiceAffinityClientIP
		svc.Spec.SessionAffinityConfig = &corev1.SessionAffinityConfig{
			ClientIP: &corev1.ClientIPConfig{
				TimeoutSeconds: int32Ptr(10800), // 3 hours
			},
		}
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
		var restartCount int
		var imageID string
		if len(pod.Status.ContainerStatuses) > 0 {
			cs := pod.Status.ContainerStatuses[0]
			restartCount = int(cs.RestartCount)
			imageID = cs.ImageID
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
			Index:        i,
			State:        state,
			Since:        since,
			RestartCount: restartCount,
			NodeName:     pod.Spec.NodeName,
			PodName:      pod.Name,
			ImageID:      imageID,
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

// GetAppDetail returns comprehensive app info extracted from K8s resources.
func (c *Client) GetAppDetail(ctx context.Context, name string) (*models.AppDetail, error) {
	dep, err := c.Clientset.AppsV1().Deployments(c.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting deployment %s: %w", name, err)
	}

	status, err := c.GetAppStatus(ctx, name)
	if err != nil {
		return nil, err
	}

	ann := dep.Annotations
	if ann == nil {
		ann = map[string]string{}
	}

	// Extract container spec
	var imageRef, command string
	var memoryMB, cpuMillis, port int
	var env map[string]string
	var healthCheck models.HealthCheck

	if len(dep.Spec.Template.Spec.Containers) > 0 {
		c0 := dep.Spec.Template.Spec.Containers[0]
		imageRef = c0.Image
		if len(c0.Command) > 2 {
			command = c0.Command[2] // /bin/sh -c <command>
		}
		if len(c0.Ports) > 0 {
			port = int(c0.Ports[0].ContainerPort)
		}

		// Parse resources
		if mem, ok := c0.Resources.Limits[corev1.ResourceMemory]; ok {
			memoryMB = int(mem.Value() / (1024 * 1024))
		}
		if cpu, ok := c0.Resources.Requests[corev1.ResourceCPU]; ok {
			cpuMillis = int(cpu.MilliValue())
		}

		// Extract env vars
		env = make(map[string]string)
		for _, e := range c0.Env {
			env[e.Name] = e.Value
		}

		// Extract health check
		if c0.ReadinessProbe != nil {
			if c0.ReadinessProbe.HTTPGet != nil {
				healthCheck = models.HealthCheck{
					Type:     models.HealthCheckHTTP,
					Endpoint: c0.ReadinessProbe.HTTPGet.Path,
				}
			} else if c0.ReadinessProbe.TCPSocket != nil {
				healthCheck = models.HealthCheck{Type: models.HealthCheckPort}
			}
		}
	}

	// Parse annotations
	diskMB, _ := strconv.Atoi(ann["microfoundry.io/disk-mb"])
	lifecycleType := ann["microfoundry.io/lifecycle"]
	if lifecycleType == "" {
		lifecycleType = "docker"
	}
	var buildpacks []string
	if bp := ann["microfoundry.io/buildpacks"]; bp != "" {
		buildpacks = strings.Split(bp, ",")
	}

	// Get image digest from first pod
	var imageDigest string
	if len(status.Instances) > 0 {
		imageDigest = status.Instances[0].ImageID
	}

	// Get routes
	routes := c.getAppRoutes(ctx, name)

	// Get secrets
	secrets := c.listAppSecrets(ctx, name)

	// Parse services from annotation
	var services []models.ServiceBindingInfo
	if svcList := ann["microfoundry.io/services"]; svcList != "" {
		for _, s := range strings.Split(svcList, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				services = append(services, models.ServiceBindingInfo{
					Name: s, Type: "managed", Status: "bound",
				})
			}
		}
	}

	state := "stopped"
	if status.RunningCount > 0 {
		state = "started"
	}

	detail := &models.AppDetail{
		Name:          name,
		GUID:          ann["microfoundry.io/guid"],
		State:         state,
		Organization:  c.Namespace,
		Owner:         ann["microfoundry.io/owner"],
		LifecycleType: lifecycleType,
		Buildpacks:    buildpacks,
		ImageRef:      imageRef,
		ImageDigest:   imageDigest,
		Command:       command,
		Instances:     int(*dep.Spec.Replicas),
		RunningCount:  status.RunningCount,
		MemoryMB:      memoryMB,
		DiskMB:        diskMB,
		CPUMillis:     cpuMillis,
		Port:          port,
		HealthCheck:   healthCheck,
		Routes:        routes,
		Env:           env,
		Labels:        dep.Labels,
		Annotations:   ann,
		Services:      services,
		Secrets:       secrets,
		InstanceList:  status.Instances,
		CreatedAt:     dep.CreationTimestamp.Time,
	}

	if t, err := time.Parse(time.RFC3339, ann["microfoundry.io/created-at"]); err == nil {
		detail.CreatedAt = t
	}

	return detail, nil
}

// ListAppItems returns enriched app list items from K8s deployments.
func (c *Client) ListAppItems(ctx context.Context) ([]models.AppListItem, error) {
	deps, err := c.Clientset.AppsV1().Deployments(c.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=" + labelManagedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("listing apps: %w", err)
	}

	ingressMap, _ := c.ListIngresses(ctx)

	var items []models.AppListItem
	for _, dep := range deps.Items {
		name := dep.Name
		ann := dep.Annotations
		if ann == nil {
			ann = map[string]string{}
		}

		// Get pod status
		var runningCount int
		pods, err := c.Clientset.CoreV1().Pods(c.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", name),
		})
		if err == nil {
			for _, pod := range pods.Items {
				if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Running != nil {
					runningCount++
				}
			}
		}

		state := "stopped"
		if runningCount > 0 {
			state = "started"
		}

		// Parse memory from container spec
		var memoryMB int
		if len(dep.Spec.Template.Spec.Containers) > 0 {
			if mem, ok := dep.Spec.Template.Spec.Containers[0].Resources.Limits[corev1.ResourceMemory]; ok {
				memoryMB = int(mem.Value() / (1024 * 1024))
			}
		}

		lifecycleType := ann["microfoundry.io/lifecycle"]
		if lifecycleType == "" {
			lifecycleType = "docker"
		}

		createdAt := dep.CreationTimestamp.Time.Format(time.RFC3339)
		if t := ann["microfoundry.io/created-at"]; t != "" {
			createdAt = t
		}

		item := models.AppListItem{
			Name:          name,
			Organization:  c.Namespace,
			Owner:         ann["microfoundry.io/owner"],
			State:         state,
			LifecycleType: lifecycleType,
			RunningCount:  runningCount,
			TotalCount:    int(*dep.Spec.Replicas),
			MemoryMB:      memoryMB,
			CreatedAt:     createdAt,
		}

		if hosts, ok := ingressMap[name]; ok {
			item.Routes = hosts
		}

		items = append(items, item)
	}

	return items, nil
}

// getAppRoutes returns detailed route info for an app from its Ingress.
func (c *Client) getAppRoutes(ctx context.Context, appName string) []models.RouteDetail {
	ingress, err := c.Clientset.NetworkingV1().Ingresses(c.Namespace).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil
	}

	hasTLS := len(ingress.Spec.TLS) > 0

	// Detect special protocol from ingress annotations
	routeProtocol := detectProtocolFromAnnotations(ingress.Annotations)

	var routes []models.RouteDetail
	for _, rule := range ingress.Spec.Rules {
		host := rule.Host
		hostPart := host
		domainPart := ""
		if idx := strings.Index(host, "."); idx > 0 {
			hostPart = host[:idx]
			domainPart = host[idx+1:]
		}

		// Use Route.Scheme() for protocol-aware URL scheme
		r := models.Route{Protocol: routeProtocol}
		scheme := r.Scheme(hasTLS)

		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				routes = append(routes, models.RouteDetail{
					Host:     hostPart,
					Domain:   domainPart,
					Path:     path.Path,
					URL:      scheme + "://" + host + path.Path,
					Protocol: scheme,
				})
			}
		}
	}

	return routes
}

// detectProtocolFromAnnotations infers the route protocol from ingress annotations.
func detectProtocolFromAnnotations(ann map[string]string) string {
	if ann == nil {
		return ""
	}
	// nginx: backend-protocol=GRPC
	if v, ok := ann["nginx.ingress.kubernetes.io/backend-protocol"]; ok && strings.EqualFold(v, "GRPC") {
		return "grpc"
	}
	// nginx: connection-proxy-header=upgrade (WebSocket)
	if _, ok := ann["nginx.ingress.kubernetes.io/connection-proxy-header"]; ok {
		return "websocket"
	}
	// kong: protocols contain ws or grpc
	if v, ok := ann["konghq.com/protocols"]; ok {
		if strings.Contains(v, "ws") {
			return "websocket"
		}
		if strings.Contains(v, "grpc") {
			return "grpc"
		}
	}
	// traefik: serversscheme=h2c (gRPC)
	if v, ok := ann["traefik.ingress.kubernetes.io/service.serversscheme"]; ok && v == "h2c" {
		return "grpc"
	}
	return ""
}

// listAppSecrets returns metadata about K8s Secrets associated with an app.
func (c *Client) listAppSecrets(ctx context.Context, appName string) []models.SecretInfo {
	secrets, err := c.Clientset.CoreV1().Secrets(c.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", appName),
	})
	if err != nil || len(secrets.Items) == 0 {
		return nil
	}

	var result []models.SecretInfo
	for _, s := range secrets.Items {
		result = append(result, models.SecretInfo{
			Name:      s.Name,
			Type:      string(s.Type),
			KeyCount:  len(s.Data),
			CreatedAt: s.CreationTimestamp.Format(time.RFC3339),
		})
	}
	return result
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

func int32Ptr(i int32) *int32 { return &i }
