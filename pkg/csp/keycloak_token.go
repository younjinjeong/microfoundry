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

// KeycloakTokenFetcher obtains service-account tokens from Keycloak
// with a target audience for CSP OIDC federation.
type KeycloakTokenFetcher struct {
	tokenURL     string
	clientID     string
	clientSecret string
	httpClient   *http.Client

	mu    sync.Mutex
	cache map[string]*cachedToken
}

type cachedToken struct {
	accessToken string
	expiresAt   time.Time
}

// NewKeycloakTokenFetcher creates a fetcher that obtains service-account
// tokens via the client_credentials grant.
func NewKeycloakTokenFetcher(tokenURL, clientID, clientSecret string) *KeycloakTokenFetcher {
	return &KeycloakTokenFetcher{
		tokenURL:     tokenURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		cache:        make(map[string]*cachedToken),
	}
}

// GetToken returns a Keycloak access token for the specified audience.
// Tokens are cached per audience with a 60-second safety margin.
func (k *KeycloakTokenFetcher) GetToken(ctx context.Context, audience string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	// Check cache
	if ct, ok := k.cache[audience]; ok && time.Now().Add(60*time.Second).Before(ct.expiresAt) {
		return ct.accessToken, nil
	}

	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {k.clientID},
		"client_secret": {k.clientSecret},
	}
	if audience != "" {
		data.Set("audience", audience)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", k.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("building token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting keycloak token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("keycloak token request failed (%d): %s", resp.StatusCode, body)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}

	k.cache[audience] = &cachedToken{
		accessToken: result.AccessToken,
		expiresAt:   time.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
	}

	return result.AccessToken, nil
}
