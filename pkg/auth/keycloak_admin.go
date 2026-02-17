package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// KeycloakUser represents a user from Keycloak's Admin REST API.
type KeycloakUser struct {
	ID               string              `json:"id,omitempty"`
	Username         string              `json:"username"`
	Email            string              `json:"email"`
	FirstName        string              `json:"firstName,omitempty"`
	LastName         string              `json:"lastName,omitempty"`
	Enabled          bool                `json:"enabled"`
	EmailVerified    bool                `json:"emailVerified,omitempty"`
	CreatedTimestamp int64               `json:"createdTimestamp,omitempty"`
	Attributes       map[string][]string `json:"attributes,omitempty"`
	RealmRoles       []string            `json:"-"` // populated by separate call
}

// KeycloakRole represents a Keycloak realm role.
type KeycloakRole struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// KeycloakAdminClient provides CRUD operations against the Keycloak Admin REST API
// using a service account client_credentials grant.
type KeycloakAdminClient struct {
	baseURL      string
	realm        string
	clientID     string
	clientSecret string
	httpClient   *http.Client

	mu          sync.Mutex
	token       string
	tokenExpiry time.Time
}

// NewKeycloakAdminClient creates an admin API client.
// baseURL is the Keycloak server URL (e.g. "http://localhost:8180").
func NewKeycloakAdminClient(baseURL, realm, clientID, clientSecret string) *KeycloakAdminClient {
	if realm == "" {
		realm = "microfoundry"
	}
	return &KeycloakAdminClient{
		baseURL:      strings.TrimRight(baseURL, "/"),
		realm:        realm,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// getToken obtains or reuses a cached service account token via client_credentials.
func (kc *KeycloakAdminClient) getToken(ctx context.Context) (string, error) {
	kc.mu.Lock()
	defer kc.mu.Unlock()

	if kc.token != "" && time.Now().Before(kc.tokenExpiry) {
		return kc.token, nil
	}

	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {kc.clientID},
		"client_secret": {kc.clientSecret},
	}

	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", kc.baseURL, kc.realm)
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := kc.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting service account token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed (%d): %s", resp.StatusCode, body)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	kc.token = result.AccessToken
	// Cache with 30s safety margin
	kc.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn-30) * time.Second)
	return kc.token, nil
}

// doRequest executes an authenticated request against the Keycloak Admin API.
func (kc *KeycloakAdminClient) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	token, err := kc.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("obtaining admin token: %w", err)
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, kc.baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return kc.httpClient.Do(req)
}

// ListUsers returns users matching the search query with pagination.
// Returns the user list, total count, and any error.
func (kc *KeycloakAdminClient) ListUsers(ctx context.Context, search string, first, max int) ([]KeycloakUser, int, error) {
	if max <= 0 {
		max = 20
	}
	path := fmt.Sprintf("/admin/realms/%s/users?first=%d&max=%d", kc.realm, first, max)
	if search != "" {
		path += "&search=" + url.QueryEscape(search)
	}

	resp, err := kc.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("list users failed (%d): %s", resp.StatusCode, body)
	}

	var users []KeycloakUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, 0, err
	}

	total := len(users)
	if h := resp.Header.Get("X-Total-Count"); h != "" {
		if n, err := strconv.Atoi(h); err == nil {
			total = n
		}
	}

	return users, total, nil
}

// CountUsers returns the total number of users in the realm.
func (kc *KeycloakAdminClient) CountUsers(ctx context.Context) (int, error) {
	path := fmt.Sprintf("/admin/realms/%s/users/count", kc.realm)
	resp, err := kc.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("count users failed (%d): %s", resp.StatusCode, body)
	}

	var count int
	if err := json.NewDecoder(resp.Body).Decode(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// GetUser retrieves a single user by ID.
func (kc *KeycloakAdminClient) GetUser(ctx context.Context, userID string) (*KeycloakUser, error) {
	path := fmt.Sprintf("/admin/realms/%s/users/%s", kc.realm, userID)
	resp, err := kc.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("user not found: %s", userID)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get user failed (%d): %s", resp.StatusCode, body)
	}

	var user KeycloakUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

// CreateUser creates a new user and returns the created user's ID.
func (kc *KeycloakAdminClient) CreateUser(ctx context.Context, user *KeycloakUser) (string, error) {
	path := fmt.Sprintf("/admin/realms/%s/users", kc.realm)
	resp, err := kc.doRequest(ctx, "POST", path, user)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create user failed (%d): %s", resp.StatusCode, body)
	}

	// Extract user ID from Location header
	location := resp.Header.Get("Location")
	if location != "" {
		parts := strings.Split(location, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}
	return "", nil
}

// UpdateUser updates an existing user.
func (kc *KeycloakAdminClient) UpdateUser(ctx context.Context, user *KeycloakUser) error {
	path := fmt.Sprintf("/admin/realms/%s/users/%s", kc.realm, user.ID)
	resp, err := kc.doRequest(ctx, "PUT", path, user)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update user failed (%d): %s", resp.StatusCode, body)
	}
	return nil
}

// DeleteUser removes a user by ID.
func (kc *KeycloakAdminClient) DeleteUser(ctx context.Context, userID string) error {
	path := fmt.Sprintf("/admin/realms/%s/users/%s", kc.realm, userID)
	resp, err := kc.doRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete user failed (%d): %s", resp.StatusCode, body)
	}
	return nil
}

// ResetPassword sets a new password for a user.
func (kc *KeycloakAdminClient) ResetPassword(ctx context.Context, userID, newPassword string, temporary bool) error {
	path := fmt.Sprintf("/admin/realms/%s/users/%s/reset-password", kc.realm, userID)
	cred := map[string]any{
		"type":      "password",
		"value":     newPassword,
		"temporary": temporary,
	}
	resp, err := kc.doRequest(ctx, "PUT", path, cred)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reset password failed (%d): %s", resp.StatusCode, body)
	}
	return nil
}

// GetUserRoles returns the realm roles assigned to a user.
func (kc *KeycloakAdminClient) GetUserRoles(ctx context.Context, userID string) ([]KeycloakRole, error) {
	path := fmt.Sprintf("/admin/realms/%s/users/%s/role-mappings/realm", kc.realm, userID)
	resp, err := kc.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get user roles failed (%d): %s", resp.StatusCode, body)
	}

	var roles []KeycloakRole
	if err := json.NewDecoder(resp.Body).Decode(&roles); err != nil {
		return nil, err
	}
	return roles, nil
}

// GetRealmRoles returns all available realm roles.
func (kc *KeycloakAdminClient) GetRealmRoles(ctx context.Context) ([]KeycloakRole, error) {
	path := fmt.Sprintf("/admin/realms/%s/roles", kc.realm)
	resp, err := kc.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get realm roles failed (%d): %s", resp.StatusCode, body)
	}

	var roles []KeycloakRole
	if err := json.NewDecoder(resp.Body).Decode(&roles); err != nil {
		return nil, err
	}
	return roles, nil
}

// AssignUserRole assigns realm roles to a user.
func (kc *KeycloakAdminClient) AssignUserRole(ctx context.Context, userID string, roles []KeycloakRole) error {
	path := fmt.Sprintf("/admin/realms/%s/users/%s/role-mappings/realm", kc.realm, userID)
	resp, err := kc.doRequest(ctx, "POST", path, roles)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("assign role failed (%d): %s", resp.StatusCode, body)
	}
	return nil
}

// RemoveUserRole removes realm roles from a user.
func (kc *KeycloakAdminClient) RemoveUserRole(ctx context.Context, userID string, roles []KeycloakRole) error {
	path := fmt.Sprintf("/admin/realms/%s/users/%s/role-mappings/realm", kc.realm, userID)
	resp, err := kc.doRequest(ctx, "DELETE", path, roles)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remove role failed (%d): %s", resp.StatusCode, body)
	}
	return nil
}
