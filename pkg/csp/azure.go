package csp

import (
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

// AzureTokenProvider exchanges a Keycloak JWT for an Azure AD access token
// via Federated Identity Credentials (client assertion).
type AzureTokenProvider struct {
	keycloak   *KeycloakTokenFetcher
	tenantID   string
	clientID   string
	httpClient *http.Client

	mu     sync.Mutex
	cached *CSPCredentials
}

// NewAzureTokenProvider creates a provider that uses Keycloak OIDC federation
// with Azure AD Federated Identity Credentials.
func NewAzureTokenProvider(keycloak *KeycloakTokenFetcher, tenantID, clientID string) *AzureTokenProvider {
	return &AzureTokenProvider{
		keycloak:   keycloak,
		tenantID:   tenantID,
		clientID:   clientID,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (z *AzureTokenProvider) Provider() string { return "azure" }

func (z *AzureTokenProvider) GetCredentials(ctx context.Context) (*CSPCredentials, error) {
	z.mu.Lock()
	defer z.mu.Unlock()

	if z.cached != nil && time.Now().Add(60*time.Second).Before(z.cached.ExpiresAt) {
		return z.cached, nil
	}

	// Audience for the Keycloak token is the Azure AD app's client ID
	jwt, err := z.keycloak.GetToken(ctx, z.clientID)
	if err != nil {
		return nil, fmt.Errorf("obtaining keycloak token for Azure: %w", err)
	}

	// Exchange Keycloak JWT for Azure AD access token via federated credential
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", z.tenantID)

	data := url.Values{
		"grant_type":            {"client_credentials"},
		"client_id":             {z.clientID},
		"client_assertion_type": {"urn:ietf:params:oauth:client-assertion-type:jwt-bearer"},
		"client_assertion":      {jwt},
		"scope":                 {"https://management.azure.com/.default"},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("building Azure token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := z.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting Azure AD token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Azure AD token request failed (%d): %s", resp.StatusCode, body)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding Azure token response: %w", err)
	}

	z.cached = &CSPCredentials{
		AzureAccessToken: result.AccessToken,
		ExpiresAt:        time.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
	}
	return z.cached, nil
}
