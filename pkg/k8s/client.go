package k8s

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Client wraps the Kubernetes clientset for MicroFoundry operations.
type Client struct {
	Clientset kubernetes.Interface
	Namespace string
	Domain    string
	Gateway   GatewayProvider

	// EKS IAM auth fields (set via WithEKSCluster option)
	eksClusterName string
	eksRegion      string
}

// ClientOption configures optional Client parameters.
type ClientOption func(*Client)

// WithIngressClass sets the gateway provider by ingress class name.
func WithIngressClass(ingressClass string) ClientOption {
	return func(c *Client) {
		c.Gateway = NewGatewayProvider(ingressClass)
	}
}

// WithEKSCluster configures EKS IAM-based auth for this client.
func WithEKSCluster(clusterName, region string) ClientOption {
	return func(c *Client) {
		c.eksClusterName = clusterName
		c.eksRegion = region
	}
}

// NewClient creates a K8s client using the best available auth method:
//  1. EKS IAM auth — if WithEKSCluster option is provided
//  2. Kubeconfig file — standard local development path
func NewClient(kubeContext, namespace, domain string, opts ...ClientOption) (*Client, error) {
	if namespace == "" {
		namespace = "microfoundry"
	}
	if domain == "" {
		domain = "cf-local.dev"
	}

	c := &Client{
		Namespace: namespace,
		Domain:    domain,
		Gateway:   NewGatewayProvider("nginx"),
	}
	for _, o := range opts {
		o(c)
	}

	var restConfig *rest.Config
	var err error

	if c.eksClusterName != "" {
		// Path 1: EKS IAM auth (Fargate task role / IRSA / EC2 instance profile)
		log.Printf("[k8s] connecting to EKS cluster %q (region=%s) via IAM", c.eksClusterName, c.eksRegion)
		restConfig, err = BuildEKSConfig(c.eksClusterName, c.eksRegion)
		if err != nil {
			return nil, fmt.Errorf("building EKS config for %q: %w", c.eksClusterName, err)
		}
	} else {
		// Path 2: kubeconfig file (local dev, mounted kubeconfig)
		restConfig, err = buildKubeconfigConfig(kubeContext)
		if err != nil {
			return nil, err
		}
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}

	c.Clientset = clientset
	return c, nil
}

// buildKubeconfigConfig loads a rest.Config from the local kubeconfig file.
func buildKubeconfigConfig(kubeContext string) (*rest.Config, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(homedir.HomeDir(), ".kube", "config")
	}

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
	return config, nil
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
