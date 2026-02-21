package admin

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/younjinjeong/microfoundry/pkg/hosts"
	"github.com/younjinjeong/microfoundry/pkg/k8s"
	"github.com/younjinjeong/microfoundry/pkg/models"
	"github.com/younjinjeong/microfoundry/pkg/settings"
	mftls "github.com/younjinjeong/microfoundry/pkg/tls"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	data := s.pageDataWithUser(r,"Registry", "settings-registry")
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

	data := s.pageDataWithUser(r,"Webhooks", "settings-webhooks")
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

	data := s.pageDataWithUser(r,"SMTP", "settings-smtp")
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

// --- Service Endpoints ---

// EndpointsSettingsHandler renders the service endpoints configuration page.
func (s *Server) EndpointsSettingsHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	ps, _ := store.Get(ctx)
	endpoints := client.DiscoverPlatformServices(ctx, client.Domain, ps.Endpoints.Overrides)

	data := s.pageDataWithUser(r,"Service Endpoints", "settings-endpoints")
	data.Content = map[string]any{
		"Endpoints": endpoints,
		"Domain":    client.Domain,
		"Saved":     r.URL.Query().Get("saved") == "true",
	}
	s.templates.Render(w, "settings_endpoints.html", data)
}

// SaveEndpointsHandler persists endpoint URL overrides and hot-swaps monitoring clients.
func (s *Server) SaveEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	overrides := make(map[string]string)
	for _, svc := range models.PlatformServices {
		if url := r.FormValue("url_" + svc.Name); url != "" {
			overrides[svc.Name] = url
		}
	}

	cfg := models.EndpointsConfig{Overrides: overrides}
	if err := store.SaveEndpoints(r.Context(), cfg); err != nil {
		http.Error(w, "Failed to save: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Re-discover and update in-memory monitoring clients
	endpoints := client.DiscoverPlatformServices(r.Context(), client.Domain, overrides)
	s.UpdateEndpointURLs(endpoints)

	http.Redirect(w, r, "/settings/endpoints?saved=true", http.StatusSeeOther)
}

// CreateEndpointIngressHandler creates an ingress for a platform service.
func (s *Server) CreateEndpointIngressHandler(w http.ResponseWriter, r *http.Request) {
	serviceName := r.PathValue("name")

	client, err := s.getClient(r)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="text-red-600">No cluster: %s</span>`, err)
		return
	}

	// Find the service definition
	var svc *models.ServiceEndpoint
	for i := range models.PlatformServices {
		if models.PlatformServices[i].Name == serviceName {
			svc = &models.PlatformServices[i]
			break
		}
	}
	if svc == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<span class="text-red-600">Unknown service</span>`)
		return
	}

	ctx := r.Context()
	ns := svc.Namespace
	if ns == "" {
		ns = client.Namespace
	}

	// For services in a different namespace, create a scoped client
	targetClient := client
	if ns != client.Namespace {
		targetClient = &k8s.Client{
			Clientset: client.Clientset,
			Namespace: ns,
			Domain:    client.Domain,
			Gateway:   client.Gateway,
		}
	}

	if err := targetClient.CreateSystemIngress(ctx, svc.Name, client.Domain, svc.ServicePort); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="text-red-600">Failed: %s</span>`, err)
		return
	}

	host := fmt.Sprintf("%s.%s", svc.Name, client.Domain)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800">Ingress: %s</span>`, host)
}

// TestEndpointHandler tests connectivity to a service endpoint.
func (s *Server) TestEndpointHandler(w http.ResponseWriter, r *http.Request) {
	serviceName := r.PathValue("name")
	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<span class="text-red-600">Invalid form data</span>`)
		return
	}

	testURL := r.FormValue("url")
	if testURL == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<span class="text-yellow-600">No URL to test</span>`)
		return
	}

	// Find health path for this service
	healthPath := "/"
	for _, svc := range models.PlatformServices {
		if svc.Name == serviceName {
			healthPath = svc.HealthPath
			break
		}
	}

	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Get(testURL + healthPath)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err != nil {
		fmt.Fprintf(w, `<span class="text-red-600">Failed: %s</span>`, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Fprint(w, `<span class="text-green-600">Connected</span>`)
	} else {
		fmt.Fprintf(w, `<span class="text-yellow-600">Reachable but HTTP %d</span>`, resp.StatusCode)
	}
}

// APIGetEndpointsHandler returns discovered endpoints as JSON.
func (s *Server) APIGetEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	client, err := s.getClient(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	ps, _ := store.Get(r.Context())
	endpoints := client.DiscoverPlatformServices(r.Context(), client.Domain, ps.Endpoints.Overrides)
	writeJSON(w, http.StatusOK, endpoints)
}

// APISaveEndpointsHandler saves endpoint overrides from JSON body.
func (s *Server) APISaveEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	var body models.EndpointsConfig
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if err := store.SaveEndpoints(r.Context(), body); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Update in-memory URLs
	client, _ := s.getClient(r)
	if client != nil {
		endpoints := client.DiscoverPlatformServices(r.Context(), client.Domain, body.Overrides)
		s.UpdateEndpointURLs(endpoints)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// --- Platform Settings (DNS, TLS, Environment) ---

// IngressInfo represents a K8s Ingress resource for display.
type IngressInfo struct {
	Name      string
	Namespace string
	Host      string
	TLS       bool
	Secret    string
	Service   string
	Port      int32
	Class     string
}

// listIngresses lists all Ingress resources in the given namespaces.
func (s *Server) listIngresses(ctx context.Context, client *k8s.Client, namespaces []string) []IngressInfo {
	var result []IngressInfo
	for _, ns := range namespaces {
		ingresses, err := client.Clientset.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, ing := range ingresses.Items {
			className := ""
			if ing.Spec.IngressClassName != nil {
				className = *ing.Spec.IngressClassName
			}
			hasTLS := len(ing.Spec.TLS) > 0
			secretName := ""
			if hasTLS && len(ing.Spec.TLS[0].SecretName) > 0 {
				secretName = ing.Spec.TLS[0].SecretName
			}
			for _, rule := range ing.Spec.Rules {
				svcName := ""
				var svcPort int32
				if rule.HTTP != nil && len(rule.HTTP.Paths) > 0 {
					svcName = rule.HTTP.Paths[0].Backend.Service.Name
					svcPort = rule.HTTP.Paths[0].Backend.Service.Port.Number
				}
				result = append(result, IngressInfo{
					Name:      ing.Name,
					Namespace: ns,
					Host:      rule.Host,
					TLS:       hasTLS,
					Secret:    secretName,
					Service:   svcName,
					Port:      svcPort,
					Class:     className,
				})
			}
		}
	}
	return result
}

func hostsFilePath() string {
	if runtime.GOOS == "windows" {
		return `C:\Windows\System32\drivers\etc\hosts`
	}
	return "/etc/hosts"
}

func providerLabel(provider string) string {
	switch provider {
	case "docker-desktop":
		return "Local Development — Docker Desktop"
	case "eks":
		return "Production — AWS EKS"
	case "gke":
		return "Production — Google GKE"
	case "aks":
		return "Production — Azure AKS"
	default:
		return "Kubernetes"
	}
}

func isLocalProvider(provider string) bool {
	return provider == "docker-desktop" || provider == "kind" || provider == "minikube"
}

// PlatformSettingsHandler renders the DNS, TLS, and environment settings page.
func (s *Server) PlatformSettingsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	clusters := s.clientManager.ListClusters(ctx)

	// Find active cluster info
	var activeProvider, activeVersion string
	var activeNodeCount int
	for _, c := range clusters {
		if c.IsActive {
			activeProvider = c.Provider
			activeVersion = c.Version
			activeNodeCount = c.NodeCount
			break
		}
	}
	if activeProvider == "" {
		activeProvider = "docker-desktop"
	}

	// TLS certificate info — parse admin cert PEM for details
	var certInfo map[string]any
	if s.tlsCertFile != "" {
		if certPEM, err := os.ReadFile(s.tlsCertFile); err == nil {
			block, _ := pem.Decode(certPEM)
			if block != nil {
				if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
					certInfo = map[string]any{
						"CertFile":  s.tlsCertFile,
						"KeyFile":   s.tlsKeyFile,
						"Subject":   cert.Subject.CommonName,
						"DNSNames":  cert.DNSNames,
						"NotBefore": cert.NotBefore,
						"NotAfter":  cert.NotAfter,
						"Issuer":    cert.Issuer.CommonName,
					}
				}
			}
		}
	}

	// K8s TLS secret check
	hasTLSSecret := mftls.HasTLSSecret(ctx, client.Clientset, client.Namespace)
	hasTLSSecretMon := mftls.HasTLSSecret(ctx, client.Clientset, "monitoring")

	// Hosts file entries
	hostsEntries, _ := hosts.List()

	// All ingresses across app and monitoring namespaces
	ingresses := s.listIngresses(ctx, client, []string{client.Namespace, "monitoring"})

	// Platform services
	store, _ := s.settingsStore(r)
	var overrides map[string]string
	if store != nil {
		ps, _ := store.Get(ctx)
		overrides = ps.Endpoints.Overrides
	}
	discovered := client.DiscoverPlatformServices(ctx, client.Domain, overrides)

	content := map[string]any{
		"Environment":      providerLabel(activeProvider),
		"Provider":         activeProvider,
		"IsLocal":          isLocalProvider(activeProvider),
		"K8sVersion":       activeVersion,
		"NodeCount":        activeNodeCount,
		"ActiveCluster":    s.clientManager.GetActive(),
		"Clusters":         clusters,
		"Domain":           client.Domain,
		"AdminDomain":      s.adminDomain,
		"AuthDomain":       s.config.Auth.IssuerURL,
		"Namespace":        client.Namespace,
		"TLSEnabled":       s.tlsEnabled,
		"CertInfo":         certInfo,
		"HasTLSSecret":     hasTLSSecret,
		"HasTLSSecretMon":  hasTLSSecretMon,
		"TLSSecretName":    mftls.SecretName,
		"HostsEntries":     hostsEntries,
		"HostsFilePath":    hostsFilePath(),
		"Ingresses":        ingresses,
		"PlatformServices": discovered,
		"Version":          s.version,
		"AuthEnabled":      s.authEnabled(),
	}

	data := s.pageDataWithUser(r, "Platform Settings", "platform")
	data.Content = content
	s.templates.Render(w, "settings_platform.html", data)
}

// --- Cloud Providers ---

// CloudProvidersSettingsHandler renders the CSP authentication settings page.
func (s *Server) CloudProvidersSettingsHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	ps, _ := store.Get(ctx)
	hasAWSSecret, _ := store.GetCredential(ctx, "aws-secret-access-key")
	hasGCPSecret, _ := store.GetCredential(ctx, "gcp-service-account-json")
	hasAzureSecret, _ := store.GetCredential(ctx, "azure-client-secret")

	data := s.pageDataWithUser(r, "Cloud Providers", "settings-csp")
	data.Content = map[string]any{
		"Config":         ps.CloudProviders,
		"HasAWSSecret":   hasAWSSecret != "",
		"HasGCPSecret":   hasGCPSecret != "",
		"HasAzureSecret": hasAzureSecret != "",
		"Saved":          r.URL.Query().Get("saved") == "true",
	}
	s.templates.Render(w, "settings_csp.html", data)
}

// SaveCloudProvidersHandler persists CSP settings to K8s ConfigMap + Secret.
func (s *Server) SaveCloudProvidersHandler(w http.ResponseWriter, r *http.Request) {
	store, err := s.settingsStore(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	cfg := models.CloudProviderConfig{
		AWS: models.AWSConfig{
			Enabled:     r.FormValue("aws_enabled") == "true",
			Region:      r.FormValue("aws_region"),
			AccessKeyID: r.FormValue("aws_access_key_id"),
			AssumeRole:  r.FormValue("aws_assume_role"),
		},
		GCP: models.GCPConfig{
			Enabled:   r.FormValue("gcp_enabled") == "true",
			ProjectID: r.FormValue("gcp_project_id"),
			Region:    r.FormValue("gcp_region"),
		},
		Azure: models.AzureConfig{
			Enabled:        r.FormValue("azure_enabled") == "true",
			TenantID:       r.FormValue("azure_tenant_id"),
			ClientID:       r.FormValue("azure_client_id"),
			SubscriptionID: r.FormValue("azure_subscription_id"),
			ResourceGroup:  r.FormValue("azure_resource_group"),
			Location:       r.FormValue("azure_location"),
		},
	}

	secrets := map[string]string{
		"aws-secret-access-key":    r.FormValue("aws_secret_access_key"),
		"gcp-service-account-json": r.FormValue("gcp_service_account_json"),
		"azure-client-secret":      r.FormValue("azure_client_secret"),
	}

	if err := store.SaveCloudProviders(r.Context(), cfg, secrets); err != nil {
		http.Error(w, "Failed to save: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings/cloud-providers?saved=true", http.StatusSeeOther)
}

// TestCloudProviderHandler tests connectivity for a cloud provider.
func (s *Server) TestCloudProviderHandler(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	var msg string

	switch provider {
	case "aws":
		region := r.FormValue("aws_region")
		accessKey := r.FormValue("aws_access_key_id")
		secretKey := r.FormValue("aws_secret_access_key")
		if region == "" || accessKey == "" {
			msg = `<span class="text-red-600">AWS Region and Access Key ID are required.</span>`
		} else if secretKey == "" {
			store, err := s.settingsStore(r)
			if err != nil {
				msg = `<span class="text-red-600">No cluster available.</span>`
			} else {
				secretKey, _ = store.GetCredential(r.Context(), "aws-secret-access-key")
				if secretKey == "" {
					msg = `<span class="text-red-600">Secret Access Key is required.</span>`
				} else {
					msg = fmt.Sprintf(`<span class="text-green-600">AWS credentials configured (region: %s). Full validation requires aws-sdk-go.</span>`, region)
				}
			}
		} else {
			msg = fmt.Sprintf(`<span class="text-green-600">AWS credentials configured (region: %s). Full validation requires aws-sdk-go.</span>`, region)
		}
	case "gcp":
		projectID := r.FormValue("gcp_project_id")
		if projectID == "" {
			msg = `<span class="text-red-600">GCP Project ID is required.</span>`
		} else {
			msg = fmt.Sprintf(`<span class="text-green-600">GCP credentials configured (project: %s). Full validation requires google-cloud-go.</span>`, projectID)
		}
	case "azure":
		tenantID := r.FormValue("azure_tenant_id")
		clientID := r.FormValue("azure_client_id")
		if tenantID == "" || clientID == "" {
			msg = `<span class="text-red-600">Azure Tenant ID and Client ID are required.</span>`
		} else {
			msg = fmt.Sprintf(`<span class="text-green-600">Azure credentials configured (tenant: %s). Full validation requires azure-sdk-for-go.</span>`, tenantID)
		}
	default:
		msg = `<span class="text-red-600">Unknown provider.</span>`
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, msg)
}
