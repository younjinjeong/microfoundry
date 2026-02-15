package k8s

import (
	"context"
	"fmt"

	"github.com/younjinjeong/microfoundry/pkg/models"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateIngress creates or updates an Ingress resource for the given routes.
func (c *Client) CreateIngress(ctx context.Context, appName string, routes []models.Route) error {
	if len(routes) == 0 {
		return nil
	}

	pathType := networkingv1.PathTypePrefix
	ingressClassName := "nginx"
	var rules []networkingv1.IngressRule

	for _, route := range routes {
		path := route.Path
		if path == "" {
			path = "/"
		}
		rules = append(rules, networkingv1.IngressRule{
			Host: fmt.Sprintf("%s.%s", route.Host, route.Domain),
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     path,
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: appName,
									Port: networkingv1.ServiceBackendPort{Number: 80},
								},
							},
						},
					},
				},
			},
		})
	}

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: c.Namespace,
			Labels:    appLabels(appName),
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingressClassName,
			Rules:            rules,
		},
	}

	existing, err := c.Clientset.NetworkingV1().Ingresses(c.Namespace).Get(ctx, appName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = c.Clientset.NetworkingV1().Ingresses(c.Namespace).Create(ctx, ingress, metav1.CreateOptions{})
	} else if err == nil {
		ingress.ResourceVersion = existing.ResourceVersion
		_, err = c.Clientset.NetworkingV1().Ingresses(c.Namespace).Update(ctx, ingress, metav1.UpdateOptions{})
	}
	if err != nil {
		return fmt.Errorf("creating ingress for %s: %w", appName, err)
	}
	return nil
}

// DeleteIngress removes the Ingress resource for an app.
func (c *Client) DeleteIngress(ctx context.Context, appName string) error {
	err := c.Clientset.NetworkingV1().Ingresses(c.Namespace).Delete(ctx, appName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("deleting ingress for %s: %w", appName, err)
	}
	return nil
}

// ListIngresses returns all MicroFoundry-managed ingresses.
func (c *Client) ListIngresses(ctx context.Context) (map[string][]string, error) {
	ingresses, err := c.Clientset.NetworkingV1().Ingresses(c.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=" + labelManagedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("listing ingresses: %w", err)
	}

	result := make(map[string][]string)
	for _, ing := range ingresses.Items {
		var hosts []string
		for _, rule := range ing.Spec.Rules {
			hosts = append(hosts, rule.Host)
		}
		result[ing.Name] = hosts
	}
	return result, nil
}
