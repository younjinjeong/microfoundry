package k8s

import (
	"context"
	"fmt"

	"github.com/younjinjeong/microfoundry/pkg/models"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"maps"
)

const tlsSecretName = "mf-tls-wildcard"

// CreateIngress creates or updates an Ingress resource for the given routes.
// If the mf-tls-wildcard secret exists in the namespace, TLS termination is configured automatically.
func (c *Client) CreateIngress(ctx context.Context, appName string, routes []models.Route) error {
	if len(routes) == 0 {
		return nil
	}

	pathType := networkingv1.PathTypePrefix
	ingressClassName := c.Gateway.IngressClass()
	var rules []networkingv1.IngressRule
	var tlsHosts []string

	for _, route := range routes {
		path := route.Path
		if path == "" {
			path = "/"
		}
		host := fmt.Sprintf("%s.%s", route.Host, route.Domain)
		tlsHosts = append(tlsHosts, host)
		rules = append(rules, networkingv1.IngressRule{
			Host: host,
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

	// Build annotations: base app annotations + protocol-specific annotations
	annotations := c.Gateway.AppAnnotations()
	if proto := models.HasSpecialProtocol(routes); proto != "" {
		if pa := c.Gateway.ProtocolAnnotations(proto); pa != nil {
			maps.Copy(annotations, pa)
		}
	}

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        appName,
			Namespace:   c.Namespace,
			Labels:      appLabels(appName),
			Annotations: annotations,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingressClassName,
			Rules:            rules,
		},
	}

	// Add TLS if the wildcard TLS secret exists
	if c.HasTLSSecret(ctx) {
		ingress.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      tlsHosts,
				SecretName: tlsSecretName,
			},
		}
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

// CreateSystemIngress creates an ingress for a system service (e.g., Keycloak).
// The service is expected to exist in the client's namespace.
func (c *Client) CreateSystemIngress(ctx context.Context, name, domain string, servicePort int32) error {
	pathType := networkingv1.PathTypePrefix
	ingressClassName := c.Gateway.IngressClass()
	host := fmt.Sprintf("%s.%s", name, domain)

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: c.Namespace,
			Labels: map[string]string{
				"app":                          name,
				"app.kubernetes.io/managed-by": labelManagedBy,
				"microfoundry.io/system":       "true",
			},
			Annotations: c.Gateway.SystemAnnotations(name),
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingressClassName,
			Rules: []networkingv1.IngressRule{
				{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: name,
											Port: networkingv1.ServiceBackendPort{Number: servicePort},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Add TLS if the wildcard TLS secret exists
	if c.HasTLSSecret(ctx) {
		ingress.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{host},
				SecretName: tlsSecretName,
			},
		}
	}

	existing, err := c.Clientset.NetworkingV1().Ingresses(c.Namespace).Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = c.Clientset.NetworkingV1().Ingresses(c.Namespace).Create(ctx, ingress, metav1.CreateOptions{})
	} else if err == nil {
		ingress.ResourceVersion = existing.ResourceVersion
		_, err = c.Clientset.NetworkingV1().Ingresses(c.Namespace).Update(ctx, ingress, metav1.UpdateOptions{})
	}
	if err != nil {
		return fmt.Errorf("creating system ingress for %s: %w", name, err)
	}
	return nil
}

// HasTLSSecret checks if the TLS wildcard secret exists in the client's namespace.
func (c *Client) HasTLSSecret(ctx context.Context) bool {
	_, err := c.Clientset.CoreV1().Secrets(c.Namespace).Get(ctx, tlsSecretName, metav1.GetOptions{})
	return err == nil
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
