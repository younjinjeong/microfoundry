package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/younjinjeong/microfoundry/pkg/models"
	"github.com/younjinjeong/microfoundry/pkg/settings"
)

// settingsStore creates a settings store for the current request's active cluster.
func (s *Server) settingsStore(r *http.Request) (*settings.Store, error) {
	client, err := s.getClient(r)
	if err != nil {
		return nil, err
	}
	return settings.NewStore(client), nil
}

// --- Registry ---

// RegistrySettingsHandler renders the container registry configuration page.
func (s *Server) RegistrySettingsHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	ps, _ := store.Get(ctx)
	hasPwd, _ := store.GetCredential(ctx, "registry-password")

	data := s.pageData("Registry", "settings-registry")
	data.Content = map[string]any{
		"Registry":    ps.Registry,
		"HasPassword": hasPwd != "",
		"Saved":       r.URL.Query().Get("saved") == "true",
	}
	s.templates.Render(w, "settings_registry.html", data)
}

// SaveRegistryHandler persists registry settings to K8s ConfigMap + Secret.
func (s *Server) SaveRegistryHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	cfg := models.RegistryConfig{
		URL:      r.FormValue("url"),
		Project:  r.FormValue("project"),
		Username: r.FormValue("username"),
		Insecure: r.FormValue("insecure") == "true",
		Enabled:  r.FormValue("enabled") == "true",
	}
	password := r.FormValue("password")

	if err := store.SaveRegistry(r.Context(), cfg, password); err != nil {
		http.Error(w, "Failed to save: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// If registry is enabled and credentials are available, ensure the imagePullSecret exists
	if cfg.Enabled && cfg.URL != "" && cfg.Username != "" {
		if password == "" {
			password, _ = store.GetCredential(r.Context(), "registry-password")
		}
		if password != "" {
			client, _ := s.getClient(r)
			if client != nil {
				_ = client.EnsureImagePullSecret(r.Context(), cfg.URL, cfg.Username, password)
			}
		}
	}

	http.Redirect(w, r, "/settings/registry?saved=true", http.StatusSeeOther)
}

// TestRegistryHandler tests the registry connection by attempting docker login.
func (s *Server) TestRegistryHandler(w http.ResponseWriter, r *http.Request) {
	url := r.FormValue("url")
	username := r.FormValue("username")
	password := r.FormValue("password")

	if url == "" || username == "" {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<span class="text-red-600">URL and username are required.</span>`)
		return
	}

	// If no password in form, try stored credential
	if password == "" {
		store, err := s.settingsStore(r)
		if err == nil {
			password, _ = store.GetCredential(r.Context(), "registry-password")
		}
	}

	if password == "" {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<span class="text-red-600">Password is required for connection test.</span>`)
		return
	}

	// Test by trying an HTTP request to /v2/ (OCI registry API)
	testURL := fmt.Sprintf("https://%s/v2/", url)
	if r.FormValue("insecure") == "true" {
		testURL = fmt.Sprintf("http://%s/v2/", url)
	}

	req, _ := http.NewRequestWithContext(r.Context(), "GET", testURL, nil)
	req.SetBasicAuth(username, password)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)

	w.Header().Set("Content-Type", "text/html")
	if err != nil {
		fmt.Fprintf(w, `<span class="text-red-600">Connection failed: %s</span>`, err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
		// 200 = authenticated, 401 = registry reachable but bad creds
		if resp.StatusCode == http.StatusOK {
			fmt.Fprint(w, `<span class="text-green-600">Connected and authenticated successfully.</span>`)
		} else {
			fmt.Fprint(w, `<span class="text-yellow-600">Registry reachable but authentication failed. Check credentials.</span>`)
		}
	} else {
		fmt.Fprintf(w, `<span class="text-yellow-600">Registry returned status %d.</span>`, resp.StatusCode)
	}
}

// --- Webhooks ---

// WebhooksSettingsHandler renders the webhooks list page.
func (s *Server) WebhooksSettingsHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	ps, _ := store.Get(r.Context())

	data := s.pageData("Webhooks", "settings-webhooks")
	data.Content = map[string]any{
		"Webhooks":   ps.Webhooks,
		"EventTypes": models.WebhookEventTypes,
	}
	s.templates.Render(w, "settings_webhooks.html", data)
}

// CreateWebhookHandler adds a new webhook.
func (s *Server) CreateWebhookHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	r.ParseForm()
	wh := models.WebhookConfig{
		ID:        uuid.New().String()[:8],
		Name:      r.FormValue("name"),
		URL:       r.FormValue("url"),
		Events:    r.Form["events"],
		Enabled:   r.FormValue("enabled") == "true",
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	if err := store.AddWebhook(r.Context(), wh); err != nil {
		http.Error(w, "Failed to create webhook: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/settings/webhooks")
	w.WriteHeader(http.StatusOK)
}

// DeleteWebhookHandler removes a webhook by ID.
func (s *Server) DeleteWebhookHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	id := r.PathValue("id")
	if err := store.DeleteWebhook(r.Context(), id); err != nil {
		http.Error(w, "Failed to delete webhook: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// TestWebhookHandler sends a test payload to a webhook.
func (s *Server) TestWebhookHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	id := r.PathValue("id")
	ps, _ := store.Get(r.Context())

	var wh *models.WebhookConfig
	for i := range ps.Webhooks {
		if ps.Webhooks[i].ID == id {
			wh = &ps.Webhooks[i]
			break
		}
	}

	if wh == nil {
		http.Error(w, "Webhook not found", http.StatusNotFound)
		return
	}

	payload := map[string]any{
		"event":     "test.ping",
		"timestamp": time.Now().Format(time.RFC3339),
		"source":    "microfoundry",
		"data": map[string]string{
			"message": "Test webhook from MicroFoundry",
		},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(r.Context(), "POST", wh.URL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MicroFoundry-Event", "test.ping")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "error", "message": err.Error()})
		return
	}
	defer resp.Body.Close()

	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "sent",
		"status_code": resp.StatusCode,
	})
}

// --- SMTP ---

// SMTPSettingsHandler renders the SMTP configuration page.
func (s *Server) SMTPSettingsHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	ps, _ := store.Get(ctx)
	hasPwd, _ := store.GetCredential(ctx, "smtp-password")

	data := s.pageData("SMTP", "settings-smtp")
	data.Content = map[string]any{
		"SMTP":        ps.SMTP,
		"HasPassword": hasPwd != "",
		"Saved":       r.URL.Query().Get("saved") == "true",
	}
	s.templates.Render(w, "settings_smtp.html", data)
}

// SaveSMTPHandler persists SMTP settings to K8s ConfigMap + Secret.
func (s *Server) SaveSMTPHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	port, _ := strconv.Atoi(r.FormValue("port"))
	if port == 0 {
		port = 587
	}

	cfg := models.SMTPConfig{
		Host:     r.FormValue("host"),
		Port:     port,
		Username: r.FormValue("username"),
		FromAddr: r.FormValue("from_addr"),
		TLS:      r.FormValue("tls") == "true",
		Enabled:  r.FormValue("enabled") == "true",
	}
	password := r.FormValue("password")

	if err := store.SaveSMTP(r.Context(), cfg, password); err != nil {
		http.Error(w, "Failed to save: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings/smtp?saved=true", http.StatusSeeOther)
}

// TestSMTPHandler tests the SMTP connection by dialing the server.
func (s *Server) TestSMTPHandler(w http.ResponseWriter, r *http.Request) {
	host := r.FormValue("host")
	port := r.FormValue("port")
	if host == "" || port == "" {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<span class="text-red-600">Host and port are required.</span>`)
		return
	}

	addr := host + ":" + port
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)

	w.Header().Set("Content-Type", "text/html")
	if err != nil {
		fmt.Fprintf(w, `<span class="text-red-600">Connection failed: %s</span>`, err.Error())
		return
	}
	conn.Close()
	fmt.Fprint(w, `<span class="text-green-600">TCP connection to SMTP server successful.</span>`)
}

// --- API Endpoints ---

// APIGetSettingsHandler returns all platform settings as JSON.
func (s *Server) APIGetSettingsHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	ps, err := store.Get(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Redact sensitive fields
	ps.Registry.Username = ""
	writeJSON(w, http.StatusOK, ps)
}

// APISaveRegistryHandler saves registry settings from JSON body.
func (s *Server) APISaveRegistryHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	var body struct {
		models.RegistryConfig
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if err := store.SaveRegistry(r.Context(), body.RegistryConfig, body.Password); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// APIGetWebhooksHandler returns all webhooks as JSON.
func (s *Server) APIGetWebhooksHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	ps, _ := store.Get(r.Context())
	writeJSON(w, http.StatusOK, ps.Webhooks)
}

// APICreateWebhookHandler creates a webhook from JSON body.
func (s *Server) APICreateWebhookHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	var wh models.WebhookConfig
	if err := json.NewDecoder(r.Body).Decode(&wh); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	wh.ID = uuid.New().String()[:8]
	wh.CreatedAt = time.Now().Format(time.RFC3339)

	if err := store.AddWebhook(r.Context(), wh); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, wh)
}

// APIDeleteWebhookHandler deletes a webhook by ID.
func (s *Server) APIDeleteWebhookHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	id := r.PathValue("id")
	if err := store.DeleteWebhook(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// APISaveSMTPHandler saves SMTP settings from JSON body.
func (s *Server) APISaveSMTPHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	var body struct {
		models.SMTPConfig
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if err := store.SaveSMTP(r.Context(), body.SMTPConfig, body.Password); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}
