package k8s

import (
	"context"
	"fmt"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Client wraps the Kubernetes clientset for MicroFoundry operations.
type Client struct {
	Clientset kubernetes.Interface
	Namespace string
	Domain    string
}

// NewClient creates a K8s client from kubeconfig.
func NewClient(kubeContext, namespace, domain string) (*Client, error) {
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")

	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	overrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}

	if namespace == "" {
		namespace = "microfoundry"
	}
	if domain == "" {
		domain = "cf-local.dev"
	}

	return &Client{
		Clientset: clientset,
		Namespace: namespace,
		Domain:    domain,
	}, nil
}

// EnsureNamespace creates the MicroFoundry namespace if it does not exist.
func (c *Client) EnsureNamespace(ctx context.Context) error {
	_, err := c.Clientset.CoreV1().Namespaces().Get(ctx, c.Namespace, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "microfoundry",
			},
		},
	}
	_, err = c.Clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating namespace %s: %w", c.Namespace, err)
	}
	return nil
}
