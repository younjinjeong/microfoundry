package csp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// GCPTokenProvider exchanges a Keycloak JWT for a GCP access token
// via Workload Identity Federation (STS token exchange + SA impersonation).
type GCPTokenProvider struct {
	keycloak       *KeycloakTokenFetcher
	wifProvider    string // full resource name of the WIF provider
	serviceAccount string // GCP service account email to impersonate
	httpClient     *http.Client

	mu     sync.Mutex
	cached *CSPCredentials
}

// NewGCPTokenProvider creates a provider that uses Keycloak OIDC federation
// with GCP Workload Identity Federation.
func NewGCPTokenProvider(keycloak *KeycloakTokenFetcher, wifProvider, serviceAccountEmail string) *GCPTokenProvider {
	return &GCPTokenProvider{
		keycloak:       keycloak,
		wifProvider:    wifProvider,
		serviceAccount: serviceAccountEmail,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (g *GCPTokenProvider) Provider() string { return "gcp" }

func (g *GCPTokenProvider) GetCredentials(ctx context.Context) (*CSPCredentials, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.cached != nil && time.Now().Add(60*time.Second).Before(g.cached.ExpiresAt) {
		return g.cached, nil
	}

	// Audience for Keycloak token is the WIF provider resource name prefixed with IAM path
	audience := "//iam.googleapis.com/" + g.wifProvider
	jwt, err := g.keycloak.GetToken(ctx, audience)
	if err != nil {
		return nil, fmt.Errorf("obtaining keycloak token for GCP: %w", err)
	}

	// Step 1: STS token exchange — swap Keycloak JWT for federated token
	federatedToken, err := g.stsExchange(ctx, jwt)
	if err != nil {
		return nil, fmt.Errorf("GCP STS token exchange: %w", err)
	}

	// Step 2: Impersonate service account to get a GCP access token
	accessToken, expiresAt, err := g.impersonateSA(ctx, federatedToken)
	if err != nil {
		return nil, fmt.Errorf("GCP SA impersonation: %w", err)
	}

	g.cached = &CSPCredentials{
		GCPAccessToken: accessToken,
		ExpiresAt:      expiresAt,
	}
	return g.cached, nil
}

// stsExchange calls the GCP STS endpoint to exchange a JWT for a federated token.
func (g *GCPTokenProvider) stsExchange(ctx context.Context, jwt string) (string, error) {
	data := url.Values{
		"grant_type":           {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"audience":             {"//iam.googleapis.com/" + g.wifProvider},
		"scope":                {"https://www.googleapis.com/auth/cloud-platform"},
		"requested_token_type": {"urn:ietf:params:oauth:token-type:access_token"},
		"subject_token_type":   {"urn:ietf:params:oauth:token-type:jwt"},
		"subject_token":        {jwt},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://sts.googleapis.com/v1/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("STS exchange failed (%d): %s", resp.StatusCode, body)
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.AccessToken, nil
}

// impersonateSA uses a federated token to impersonate a GCP service account.
func (g *GCPTokenProvider) impersonateSA(ctx context.Context, federatedToken string) (string, time.Time, error) {
	saURL := fmt.Sprintf("https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken", g.serviceAccount)
	body := map[string]any{
		"scope":    []string{"https://www.googleapis.com/auth/cloud-platform"},
		"lifetime": "3600s",
	}
	bodyJSON, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", saURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Authorization", "Bearer "+federatedToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", time.Time{}, fmt.Errorf("SA impersonation failed (%d): %s", resp.StatusCode, respBody)
	}

	var result struct {
		AccessToken string `json:"accessToken"`
		ExpireTime  string `json:"expireTime"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", time.Time{}, err
	}

	expiresAt, _ := time.Parse(time.RFC3339, result.ExpireTime)
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(1 * time.Hour)
	}
	return result.AccessToken, expiresAt, nil
}
