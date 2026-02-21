package csp

import (
	"context"
	"fmt"
	"sync"

	"github.com/younjinjeong/microfoundry/pkg/models"
	"github.com/younjinjeong/microfoundry/pkg/settings"
)

// Manager provides CSP credentials for Terraform injection.
// It decides between static credentials (from K8s Secret) and OIDC
// federation (via Keycloak token exchange) based on the auth_mode setting.
type Manager struct {
	keycloak  *KeycloakTokenFetcher
	mu        sync.RWMutex
	providers map[string]TokenProvider
}

// NewManager creates a CSP credential manager backed by Keycloak OIDC.
func NewManager(tokenURL, clientID, clientSecret string) *Manager {
	return &Manager{
		keycloak:  NewKeycloakTokenFetcher(tokenURL, clientID, clientSecret),
		providers: make(map[string]TokenProvider),
	}
}

// GetAWSEnv returns environment variables for Terraform based on current AWS settings.
// For OIDC mode, it exchanges a Keycloak token for temporary STS credentials.
// For static mode, it reads credentials from the K8s Secret.
func (m *Manager) GetAWSEnv(ctx context.Context, cfg models.AWSConfig, store *settings.Store) (map[string]string, error) {
	env := make(map[string]string)

	if !cfg.Enabled {
		return env, nil
	}

	if cfg.Region != "" {
		env["AWS_DEFAULT_REGION"] = cfg.Region
		env["AWS_REGION"] = cfg.Region
	}

	if cfg.IsOIDC() {
		if cfg.OIDCRoleARN == "" {
			return env, fmt.Errorf("AWS OIDC mode requires oidc_role_arn")
		}
		provider := m.getOrCreateAWS(cfg)
		creds, err := provider.GetCredentials(ctx)
		if err != nil {
			return env, fmt.Errorf("AWS OIDC token exchange: %w", err)
		}
		env["AWS_ACCESS_KEY_ID"] = creds.AccessKeyID
		env["AWS_SECRET_ACCESS_KEY"] = creds.SecretAccessKey
		env["AWS_SESSION_TOKEN"] = creds.SessionToken
		return env, nil
	}

	// Static mode: read from settings store
	if cfg.AccessKeyID != "" {
		env["AWS_ACCESS_KEY_ID"] = cfg.AccessKeyID
	}
	if store != nil {
		if secret, _ := store.GetCredential(ctx, "aws-secret-access-key"); secret != "" {
			env["AWS_SECRET_ACCESS_KEY"] = secret
		}
	}
	return env, nil
}

// GetGCPEnv returns environment variables for Terraform based on current GCP settings.
func (m *Manager) GetGCPEnv(ctx context.Context, cfg models.GCPConfig, store *settings.Store) (map[string]string, error) {
	env := make(map[string]string)

	if !cfg.Enabled {
		return env, nil
	}

	if cfg.ProjectID != "" {
		env["GOOGLE_PROJECT"] = cfg.ProjectID
		env["GCLOUD_PROJECT"] = cfg.ProjectID
	}
	if cfg.Region != "" {
		env["GOOGLE_REGION"] = cfg.Region
	}

	if cfg.IsOIDC() {
		if cfg.WorkloadIdentityProvider == "" || cfg.ServiceAccountEmail == "" {
			return env, fmt.Errorf("GCP OIDC mode requires wif_provider and service_account_email")
		}
		provider := m.getOrCreateGCP(cfg)
		creds, err := provider.GetCredentials(ctx)
		if err != nil {
			return env, fmt.Errorf("GCP OIDC token exchange: %w", err)
		}
		env["GOOGLE_OAUTH_ACCESS_TOKEN"] = creds.GCPAccessToken
		return env, nil
	}

	// Static mode: service account JSON is applied via GOOGLE_APPLICATION_CREDENTIALS
	// (the executor writes the JSON to a temp file and sets the env var)
	return env, nil
}

// GetAzureEnv returns environment variables for Terraform based on current Azure settings.
func (m *Manager) GetAzureEnv(ctx context.Context, cfg models.AzureConfig, store *settings.Store) (map[string]string, error) {
	env := make(map[string]string)

	if !cfg.Enabled {
		return env, nil
	}

	if cfg.SubscriptionID != "" {
		env["ARM_SUBSCRIPTION_ID"] = cfg.SubscriptionID
	}
	if cfg.TenantID != "" {
		env["ARM_TENANT_ID"] = cfg.TenantID
	}
	if cfg.ClientID != "" {
		env["ARM_CLIENT_ID"] = cfg.ClientID
	}

	if cfg.IsOIDC() {
		if cfg.TenantID == "" || cfg.ClientID == "" {
			return env, fmt.Errorf("Azure OIDC mode requires tenant_id and client_id")
		}
		provider := m.getOrCreateAzure(cfg)
		creds, err := provider.GetCredentials(ctx)
		if err != nil {
			return env, fmt.Errorf("Azure OIDC token exchange: %w", err)
		}
		env["ARM_OIDC_TOKEN"] = creds.AzureAccessToken
		env["ARM_USE_OIDC"] = "true"
		return env, nil
	}

	// Static mode: read client secret from store
	if store != nil {
		if secret, _ := store.GetCredential(ctx, "azure-client-secret"); secret != "" {
			env["ARM_CLIENT_SECRET"] = secret
		}
	}
	return env, nil
}

// TestAWS attempts an OIDC token exchange for AWS and returns nil on success.
func (m *Manager) TestAWS(ctx context.Context, cfg models.AWSConfig) error {
	if cfg.OIDCRoleARN == "" {
		return fmt.Errorf("IAM Role ARN is required")
	}
	provider := m.getOrCreateAWS(cfg)
	_, err := provider.GetCredentials(ctx)
	return err
}

// TestGCP attempts an OIDC token exchange for GCP and returns nil on success.
func (m *Manager) TestGCP(ctx context.Context, cfg models.GCPConfig) error {
	if cfg.WorkloadIdentityProvider == "" || cfg.ServiceAccountEmail == "" {
		return fmt.Errorf("Workload Identity Provider and Service Account Email are required")
	}
	provider := m.getOrCreateGCP(cfg)
	_, err := provider.GetCredentials(ctx)
	return err
}

// TestAzure attempts an OIDC token exchange for Azure and returns nil on success.
func (m *Manager) TestAzure(ctx context.Context, cfg models.AzureConfig) error {
	if cfg.TenantID == "" || cfg.ClientID == "" {
		return fmt.Errorf("Tenant ID and Client ID are required")
	}
	provider := m.getOrCreateAzure(cfg)
	_, err := provider.GetCredentials(ctx)
	return err
}

func (m *Manager) getOrCreateAWS(cfg models.AWSConfig) *AWSTokenProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := "aws:" + cfg.OIDCRoleARN
	if p, ok := m.providers[key]; ok {
		return p.(*AWSTokenProvider)
	}
	p := NewAWSTokenProvider(m.keycloak, cfg.OIDCRoleARN, cfg.Region)
	m.providers[key] = p
	return p
}

func (m *Manager) getOrCreateGCP(cfg models.GCPConfig) *GCPTokenProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := "gcp:" + cfg.WorkloadIdentityProvider
	if p, ok := m.providers[key]; ok {
		return p.(*GCPTokenProvider)
	}
	p := NewGCPTokenProvider(m.keycloak, cfg.WorkloadIdentityProvider, cfg.ServiceAccountEmail)
	m.providers[key] = p
	return p
}

func (m *Manager) getOrCreateAzure(cfg models.AzureConfig) *AzureTokenProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := "azure:" + cfg.TenantID + ":" + cfg.ClientID
	if p, ok := m.providers[key]; ok {
		return p.(*AzureTokenProvider)
	}
	p := NewAzureTokenProvider(m.keycloak, cfg.TenantID, cfg.ClientID)
	m.providers[key] = p
	return p
}
