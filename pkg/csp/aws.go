package csp

import (
	"context"
	"fmt"
	"sync"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// AWSTokenProvider exchanges a Keycloak JWT for temporary AWS credentials
// via STS AssumeRoleWithWebIdentity.
type AWSTokenProvider struct {
	keycloak *KeycloakTokenFetcher
	roleARN  string
	audience string
	region   string

	mu     sync.Mutex
	cached *CSPCredentials
}

// NewAWSTokenProvider creates a provider that uses Keycloak OIDC federation
// to obtain temporary AWS credentials.
func NewAWSTokenProvider(keycloak *KeycloakTokenFetcher, roleARN, region string) *AWSTokenProvider {
	return &AWSTokenProvider{
		keycloak: keycloak,
		roleARN:  roleARN,
		audience: "sts.amazonaws.com",
		region:   region,
	}
}

func (a *AWSTokenProvider) Provider() string { return "aws" }

func (a *AWSTokenProvider) GetCredentials(ctx context.Context) (*CSPCredentials, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Return cached if still valid (60s safety margin)
	if a.cached != nil && time.Now().Add(60*time.Second).Before(a.cached.ExpiresAt) {
		return a.cached, nil
	}

	// 1. Get Keycloak JWT with STS audience
	jwt, err := a.keycloak.GetToken(ctx, a.audience)
	if err != nil {
		return nil, fmt.Errorf("obtaining keycloak token for AWS: %w", err)
	}

	// 2. Call STS AssumeRoleWithWebIdentity
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(a.region))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	stsClient := sts.NewFromConfig(cfg)
	sessionName := "microfoundry-oidc"
	out, err := stsClient.AssumeRoleWithWebIdentity(ctx, &sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          &a.roleARN,
		RoleSessionName:  &sessionName,
		WebIdentityToken: &jwt,
	})
	if err != nil {
		return nil, fmt.Errorf("STS AssumeRoleWithWebIdentity: %w", err)
	}

	a.cached = &CSPCredentials{
		AccessKeyID:     *out.Credentials.AccessKeyId,
		SecretAccessKey: *out.Credentials.SecretAccessKey,
		SessionToken:    *out.Credentials.SessionToken,
		ExpiresAt:       *out.Credentials.Expiration,
	}
	return a.cached, nil
}
