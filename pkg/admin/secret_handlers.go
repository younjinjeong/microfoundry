package admin

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/younjinjeong/microfoundry/pkg/models"
	"github.com/younjinjeong/microfoundry/pkg/secrets"
)

// SecretsListHandler shows all managed secrets.
func (s *Server) SecretsListHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	mgr := secrets.NewManager(client)
	items, err := mgr.List(ctx)
	if err != nil {
		log.Printf("error listing secrets: %v", err)
		items = []models.SecretListItem{}
	}

	// Apply type filter
	filter := r.URL.Query().Get("type")
	if filter == "" {
		filter = "all"
	}
	if filter != "all" {
		var filtered []models.SecretListItem
		for _, item := range items {
			if item.Type == filter {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	data := s.pageData("Secrets Manager", "secrets")
	data.Content = map[string]any{
		"Secrets": items,
		"Filter":  filter,
	}
	s.templates.Render(w, "secrets.html", data)
}

// SecretDetailHandler shows details of a single secret.
func (s *Server) SecretDetailHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	mgr := secrets.NewManager(client)
	detail, err := mgr.Get(ctx, name)
	if err != nil {
		http.Error(w, "Secret not found: "+err.Error(), http.StatusNotFound)
		return
	}

	data := s.pageData(name, "secrets")
	data.Content = map[string]any{
		"Detail": detail,
	}
	s.templates.Render(w, "secret_detail.html", data)
}

// SecretRevealHandler returns the actual value of a single key (HTMX partial).
func (s *Server) SecretRevealHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")
	key := r.PathValue("key")

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	mgr := secrets.NewManager(client)
	value, err := mgr.GetValue(ctx, name, key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<span class="font-mono text-sm text-gray-900 break-all">%s</span>`, value)
}

// CreateSecretFormHandler renders the secret creation form.
func (s *Server) CreateSecretFormHandler(w http.ResponseWriter, r *http.Request) {
	data := s.pageData("Create Secret", "secrets")
	s.templates.Render(w, "secret_create.html", data)
}

// CreateSecretHandler creates a user-defined secret.
func (s *Server) CreateSecretHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	name := r.FormValue("name")
	description := r.FormValue("description")

	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	if !models.ValidSecretName.MatchString(name) {
		http.Error(w, "invalid secret name: must be lowercase alphanumeric with hyphens, 2-42 characters", http.StatusBadRequest)
		return
	}

	// Parse key-value pairs from form
	if err := r.ParseForm(); err != nil {
		http.Error(w, "parsing form: "+err.Error(), http.StatusBadRequest)
		return
	}
	keyNames := r.Form["key_name"]
	keyValues := r.Form["key_value"]

	if len(keyNames) == 0 {
		http.Error(w, "at least one key-value pair is required", http.StatusBadRequest)
		return
	}

	data := make(map[string]string)
	for i, k := range keyNames {
		if k == "" {
			continue
		}
		v := ""
		if i < len(keyValues) {
			v = keyValues[i]
		}
		data[k] = v
	}

	if len(data) == 0 {
		http.Error(w, "at least one non-empty key is required", http.StatusBadRequest)
		return
	}

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	mgr := secrets.NewManager(client)
	if err := mgr.CreateUserSecret(ctx, name, description, data); err != nil {
		http.Error(w, "creating secret: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/secrets/"+name, http.StatusSeeOther)
}

// DeleteSecretHandler deletes a user-defined secret.
func (s *Server) DeleteSecretHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")

	client, err := s.getClient(r)
	if err != nil {
		http.Error(w, "No cluster available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	mgr := secrets.NewManager(client)

	// Check if it's a service secret — those must be deleted via delete-service
	detail, err := mgr.Get(ctx, name)
	if err != nil {
		http.Error(w, "Secret not found: "+err.Error(), http.StatusNotFound)
		return
	}

	if detail.Type == models.SecretTypeService {
		http.Error(w, fmt.Sprintf("cannot delete service secret %q — use 'mf delete-service %s' instead", name, detail.ServiceInstance), http.StatusBadRequest)
		return
	}

	// Check for bound apps
	if len(detail.BoundApps) > 0 {
		http.Error(w, fmt.Sprintf("secret %q has %d active binding(s) — unbind all apps first", name, len(detail.BoundApps)), http.StatusBadRequest)
		return
	}

	if err := mgr.Delete(ctx, name); err != nil {
		http.Error(w, "deleting secret: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// API handlers

// APISecretsListHandler returns secrets as JSON.
func (s *Server) APISecretsListHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	client, err := s.getClient(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	mgr := secrets.NewManager(client)
	items, err := mgr.List(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, items)
}

// APISecretDetailHandler returns a single secret as JSON (values redacted).
func (s *Server) APISecretDetailHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")

	client, err := s.getClient(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	mgr := secrets.NewManager(client)
	detail, err := mgr.Get(ctx, name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

// APICreateSecretHandler creates a user-defined secret from JSON.
func (s *Server) APICreateSecretHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Data        map[string]string `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if !models.ValidSecretName.MatchString(req.Name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid secret name"})
		return
	}
	if len(req.Data) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one key-value pair is required"})
		return
	}

	client, err := s.getClient(r)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	mgr := secrets.NewManager(client)
	if err := mgr.CreateUserSecret(ctx, req.Name, req.Description, req.Data); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"name": req.Name, "status": "created"})
}
