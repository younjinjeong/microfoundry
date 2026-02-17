package tls

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// SecretName is the K8s TLS secret name used across all ingresses.
const SecretName = "mf-tls-wildcard"

// EnsureMkcertInstalled checks that mkcert is available on PATH.
func EnsureMkcertInstalled() error {
	_, err := exec.LookPath("mkcert")
	if err != nil {
		return fmt.Errorf("mkcert not found on PATH — install it from https://github.com/FiloSottile/mkcert")
	}
	return nil
}

// InstallRootCA runs `mkcert -install` to trust the local CA in system stores.
func InstallRootCA() error {
	cmd := exec.Command("mkcert", "-install")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GenerateCert generates a wildcard TLS certificate for the given domain.
// Returns paths to the generated cert and key PEM files.
func GenerateCert(domain string) (certFile, keyFile string, err error) {
	dir, err := os.MkdirTemp("", "mf-tls-*")
	if err != nil {
		return "", "", fmt.Errorf("creating temp dir: %w", err)
	}

	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")

	cmd := exec.Command("mkcert",
		"-cert-file", certFile,
		"-key-file", keyFile,
		fmt.Sprintf("*.%s", domain),
		domain,
		"localhost",
		"127.0.0.1",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		os.RemoveAll(dir)
		return "", "", fmt.Errorf("mkcert cert generation failed: %w", err)
	}
	return certFile, keyFile, nil
}

// EnsureTLSSecret creates or updates a kubernetes.io/tls Secret in the given namespace.
func EnsureTLSSecret(ctx context.Context, clientset kubernetes.Interface, namespace string, certPEM, keyPEM []byte) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SecretName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "microfoundry",
				"microfoundry.io/component":    "tls-wildcard",
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       certPEM,
			corev1.TLSPrivateKeyKey: keyPEM,
		},
	}

	secAPI := clientset.CoreV1().Secrets(namespace)
	existing, err := secAPI.Get(ctx, SecretName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = secAPI.Create(ctx, secret, metav1.CreateOptions{})
		return err
	} else if err != nil {
		return err
	}

	secret.ResourceVersion = existing.ResourceVersion
	_, err = secAPI.Update(ctx, secret, metav1.UpdateOptions{})
	return err
}

// HasTLSSecret checks if the TLS wildcard secret exists in the given namespace.
func HasTLSSecret(ctx context.Context, clientset kubernetes.Interface, namespace string) bool {
	_, err := clientset.CoreV1().Secrets(namespace).Get(ctx, SecretName, metav1.GetOptions{})
	return err == nil
}

// Cleanup removes temporary certificate files.
func Cleanup(certFile, keyFile string) {
	if certFile != "" {
		os.RemoveAll(filepath.Dir(certFile))
	}
}

// SaveCertsToConfigDir copies cert and key to ~/.mf/ for admin server TLS use.
// Returns paths to the saved files.
func SaveCertsToConfigDir(certPEM, keyPEM []byte) (certPath, keyPath string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(home, ".mf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}

	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")

	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return "", "", fmt.Errorf("writing cert: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return "", "", fmt.Errorf("writing key: %w", err)
	}
	return certPath, keyPath, nil
}
