package csp

import (
	"context"
	"time"
)

// CSPCredentials holds temporary cloud provider credentials obtained via OIDC federation.
type CSPCredentials struct {
	// AWS temporary credentials from STS AssumeRoleWithWebIdentity
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string

	// GCP access token from Workload Identity Federation
	GCPAccessToken string

	// Azure access token from Federated Identity Credential
	AzureAccessToken string

	// ExpiresAt is the credential expiration time.
	ExpiresAt time.Time
}

// TokenProvider obtains temporary CSP credentials via OIDC federation.
type TokenProvider interface {
	// GetCredentials returns cached or freshly-exchanged CSP credentials.
	GetCredentials(ctx context.Context) (*CSPCredentials, error)
	// Provider returns the CSP name ("aws", "gcp", "azure").
	Provider() string
}
