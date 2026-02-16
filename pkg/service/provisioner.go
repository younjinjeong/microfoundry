package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/younjinjeong/microfoundry/pkg/k8s"
	"github.com/younjinjeong/microfoundry/pkg/models"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Provisioner deploys real K8s resources for backing services.
type Provisioner struct {
	k8sClient *k8s.Client
}

// NewProvisioner creates a new service provisioner.
func NewProvisioner(client *k8s.Client) *Provisioner {
	return &Provisioner{k8sClient: client}
}

// GatewayConfigMapName returns the ConfigMap name for a gateway instance.
func GatewayConfigMapName(instanceName string) string {
	return "mf-gw-config-" + instanceName
}

// Provision creates K8s Deployment + Service + optional PVC for a service instance.
// Returns the credential map to be stored in a K8s Secret.
func (p *Provisioner) Provision(ctx context.Context, instanceName, serviceTypeID, planID string) (map[string]string, error) {
	tmpl, ok := GetTemplate(serviceTypeID)
	if !ok {
		return nil, fmt.Errorf("no template for service type %q", serviceTypeID)
	}

	plan, ok := FindPlan(serviceTypeID, planID)
	if !ok {
		return nil, fmt.Errorf("no plan %q for service type %q", planID, serviceTypeID)
	}

	password := generatePassword(24)
	k8sName := models.ServiceSecretPrefix + instanceName
	labels := serviceLabels(instanceName, serviceTypeID)

	// 1. Create PVC if needed
	if tmpl.NeedsPVC && plan.Resources.StorageGB > 0 {
		if err := p.createPVC(ctx, k8sName, plan.Resources.StorageGB, labels); err != nil {
			return nil, fmt.Errorf("creating PVC: %w", err)
		}
	}

	// 2. Create gateway ConfigMap if needed
	if tmpl.IsGateway {
		if err := p.createGatewayConfigMap(ctx, k8sName, instanceName, tmpl, labels); err != nil {
			return nil, fmt.Errorf("creating gateway ConfigMap: %w", err)
		}
	}

	// 3. Create Deployment
	if err := p.createDeployment(ctx, k8sName, instanceName, tmpl, plan, password, labels); err != nil {
		return nil, fmt.Errorf("creating Deployment: %w", err)
	}

	// 4. Create ClusterIP Service
	if err := p.createService(ctx, k8sName, tmpl, labels); err != nil {
		return nil, fmt.Errorf("creating Service: %w", err)
	}

	// 5. Wait for readiness
	if err := p.waitForReady(ctx, k8sName, 90*time.Second); err != nil {
		return nil, fmt.Errorf("waiting for readiness: %w", err)
	}

	// 6. Build outputs
	host := fmt.Sprintf("%s.%s.svc.cluster.local", k8sName, p.k8sClient.Namespace)
	outputs := tmpl.BuildOutputs(host, int(tmpl.ServicePort), password, instanceName)
	return outputs, nil
}

// Deprovision removes all K8s resources for a service instance.
func (p *Provisioner) Deprovision(ctx context.Context, instanceName string) error {
	k8sName := models.ServiceSecretPrefix + instanceName
	ns := p.k8sClient.Namespace
	propagation := metav1.DeletePropagationForeground
	opts := metav1.DeleteOptions{PropagationPolicy: &propagation}

	// Delete Deployment
	err := p.k8sClient.Clientset.AppsV1().Deployments(ns).Delete(ctx, k8sName, opts)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("deleting deployment: %w", err)
	}

	// Delete Service
	err = p.k8sClient.Clientset.CoreV1().Services(ns).Delete(ctx, k8sName, opts)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("deleting service: %w", err)
	}

	// Delete PVC
	err = p.k8sClient.Clientset.CoreV1().PersistentVolumeClaims(ns).Delete(ctx, k8sName, opts)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("deleting PVC: %w", err)
	}

	// Delete gateway ConfigMap (no-op if not a gateway)
	gwCmName := GatewayConfigMapName(instanceName)
	err = p.k8sClient.Clientset.CoreV1().ConfigMaps(ns).Delete(ctx, gwCmName, opts)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("deleting gateway ConfigMap: %w", err)
	}

	return nil
}

func serviceLabels(instanceName, serviceTypeID string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by":      "microfoundry",
		"microfoundry.io/service-instance":  instanceName,
		"microfoundry.io/service-type":      serviceTypeID,
		"microfoundry.io/component":         "backing-service",
	}
}

func (p *Provisioner) createPVC(ctx context.Context, name string, storageGB int, labels map[string]string) error {
	ns := p.k8sClient.Namespace
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(fmt.Sprintf("%dGi", storageGB)),
				},
			},
		},
	}

	_, err := p.k8sClient.Clientset.CoreV1().PersistentVolumeClaims(ns).Create(ctx, pvc, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		return nil // idempotent
	}
	return err
}

func (p *Provisioner) createDeployment(ctx context.Context, k8sName, instanceName string, tmpl ServiceTemplate, plan models.ServicePlan, password string, labels map[string]string) error {
	ns := p.k8sClient.Namespace
	replicas := int32(1)

	// Build environment variables with substitution
	var envVars []corev1.EnvVar
	for k, v := range tmpl.Env {
		v = strings.ReplaceAll(v, "${PASSWORD}", password)
		v = strings.ReplaceAll(v, "${INSTANCE_NAME}", instanceName)
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}

	// Build container ports
	var ports []corev1.ContainerPort
	for _, p := range tmpl.Ports {
		ports = append(ports, corev1.ContainerPort{ContainerPort: p})
	}

	// Build command
	var command []string
	if len(tmpl.Command) > 0 {
		command = make([]string, len(tmpl.Command))
		for i, c := range tmpl.Command {
			c = strings.ReplaceAll(c, "${PASSWORD}", password)
			c = strings.ReplaceAll(c, "${INSTANCE_NAME}", instanceName)
			command[i] = c
		}
	}

	// Build readiness probe
	var probe *corev1.Probe
	switch tmpl.Probe.Type {
	case "tcp":
		probe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(tmpl.Probe.Port)},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       5,
		}
	case "http":
		probe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: tmpl.Probe.Path,
					Port: intstr.FromInt32(tmpl.Probe.Port),
				},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       5,
		}
	}

	container := corev1.Container{
		Name:            "service",
		Image:           tmpl.Image,
		Ports:           ports,
		Env:             envVars,
		Command:         command,
		ReadinessProbe:  probe,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", plan.Resources.MemoryMB/2)),
				corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", plan.Resources.CPUMillis)),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", plan.Resources.MemoryMB)),
			},
		},
	}

	// Volume mount if PVC
	var volumes []corev1.Volume
	if tmpl.NeedsPVC && tmpl.PVCMountPath != "" {
		container.VolumeMounts = []corev1.VolumeMount{
			{Name: "data", MountPath: tmpl.PVCMountPath},
		}
		volumes = []corev1.Volume{
			{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: k8sName,
					},
				},
			},
		}
	}

	// Volume mount for gateway ConfigMap
	if tmpl.IsGateway && tmpl.ConfigMountPath != "" {
		cmName := GatewayConfigMapName(instanceName)
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "gateway-config",
			MountPath: tmpl.ConfigMountPath,
			ReadOnly:  true,
		})
		volumes = append(volumes, corev1.Volume{
			Name: "gateway-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
				},
			},
		})
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sName,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
					Volumes:    volumes,
				},
			},
		},
	}

	existing, err := p.k8sClient.Clientset.AppsV1().Deployments(ns).Get(ctx, k8sName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = p.k8sClient.Clientset.AppsV1().Deployments(ns).Create(ctx, deployment, metav1.CreateOptions{})
	} else if err == nil {
		deployment.ResourceVersion = existing.ResourceVersion
		_, err = p.k8sClient.Clientset.AppsV1().Deployments(ns).Update(ctx, deployment, metav1.UpdateOptions{})
	}
	return err
}

func (p *Provisioner) createService(ctx context.Context, k8sName string, tmpl ServiceTemplate, labels map[string]string) error {
	ns := p.k8sClient.Namespace

	var svcPorts []corev1.ServicePort
	for i, port := range tmpl.Ports {
		name := fmt.Sprintf("port-%d", i)
		if port == tmpl.ServicePort {
			name = "primary"
		}
		svcPorts = append(svcPorts, corev1.ServicePort{
			Name:       name,
			Port:       port,
			TargetPort: intstr.FromInt32(port),
		})
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sName,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports:    svcPorts,
		},
	}

	existing, err := p.k8sClient.Clientset.CoreV1().Services(ns).Get(ctx, k8sName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = p.k8sClient.Clientset.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{})
	} else if err == nil {
		svc.ResourceVersion = existing.ResourceVersion
		svc.Spec.ClusterIP = existing.Spec.ClusterIP
		_, err = p.k8sClient.Clientset.CoreV1().Services(ns).Update(ctx, svc, metav1.UpdateOptions{})
	}
	return err
}

func (p *Provisioner) waitForReady(ctx context.Context, k8sName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		dep, err := p.k8sClient.Clientset.AppsV1().Deployments(p.k8sClient.Namespace).Get(ctx, k8sName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if dep.Status.ReadyReplicas >= 1 {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timeout waiting for %s to become ready", k8sName)
}

func (p *Provisioner) createGatewayConfigMap(ctx context.Context, k8sName, instanceName string, tmpl ServiceTemplate, labels map[string]string) error {
	ns := p.k8sClient.Namespace
	cmName := GatewayConfigMapName(instanceName)

	// Empty routes → initial config (Kong: empty services, Nginx: 404 catch-all)
	emptyRoutes, _ := json.Marshal([]models.GatewayRoute{})
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: ns,
			Labels:    labels,
		},
		Data: map[string]string{
			tmpl.ConfigFileName: tmpl.BuildConfig(nil),
			"routes":            string(emptyRoutes),
		},
	}

	existing, err := p.k8sClient.Clientset.CoreV1().ConfigMaps(ns).Get(ctx, cmName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = p.k8sClient.Clientset.CoreV1().ConfigMaps(ns).Create(ctx, cm, metav1.CreateOptions{})
	} else if err == nil {
		cm.ResourceVersion = existing.ResourceVersion
		_, err = p.k8sClient.Clientset.CoreV1().ConfigMaps(ns).Update(ctx, cm, metav1.UpdateOptions{})
	}
	return err
}

func generatePassword(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "fallback-change-me"
	}
	return hex.EncodeToString(b)[:length]
}
