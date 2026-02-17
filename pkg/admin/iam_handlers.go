package admin

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/younjinjeong/microfoundry/pkg/auth"
)

// IAMTabHandler returns the content for a specific IAM tab via HTMX.
// GET /users/tab/{tab}
func (s *Server) IAMTabHandler(w http.ResponseWriter, r *http.Request) {
	tab := r.PathValue("tab")

	validIAMTabs := map[string]string{
		"orgs":     "tab_iam_orgs.html",
		"users":    "tab_iam_users.html",
		"policies": "tab_iam_policies.html",
		"audit":    "tab_iam_audit.html",
	}

	templateName, ok := validIAMTabs[tab]
	if !ok {
		http.Error(w, "invalid tab", http.StatusBadRequest)
		return
	}

	switch tab {
	case "orgs":
		s.renderIAMOrgsTab(w, r)
	case "users":
		s.renderIAMUsersTab(w, r)
	case "policies":
		s.renderIAMPoliciesTab(w, r, templateName)
	case "audit":
		s.renderIAMAuditTab(w, r, templateName)
	}
}

// renderIAMOrgsTab renders the organizations tab content.
func (s *Server) renderIAMOrgsTab(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	data := map[string]any{}

	if user != nil {
		store := s.orgStore()
		if store == nil {
			data["Error"] = "Organization store not available"
			s.templates.RenderPartial(w, "tab_iam_orgs.html", data)
			return
		}
		orgs, err := store.GetUserOrgs(r.Context(), user.Email)
		if err != nil {
			log.Printf("[IAM] failed to get user orgs for %s: %v", user.Email, err)
			data["Error"] = "Failed to load organizations"
		}

		selectedOrg := r.URL.Query().Get("org")
		if selectedOrg == "" && len(orgs) > 0 {
			selectedOrg = orgs[0].ID
		}

		data["Orgs"] = orgs
		data["SelectedOrg"] = selectedOrg
		data["ActiveOrg"] = user.ActiveOrgID
		data["User"] = user

		if selectedOrg != "" {
			orgDetail, err := store.Get(r.Context(), selectedOrg)
			if err == nil {
				data["OrgDetail"] = orgDetail
				members, err := store.ListMembers(r.Context(), selectedOrg)
				if err != nil {
					log.Printf("[IAM] failed to list members for org %s: %v", selectedOrg, err)
				}
				data["Members"] = members

				userRole := "viewer"
				for _, m := range members {
					if m.Email == user.Email {
						userRole = m.Role
						break
					}
				}
				data["UserRole"] = userRole
			}
		}
	}

	s.templates.RenderPartial(w, "tab_iam_orgs.html", data)
}

// renderIAMUsersTab renders the Keycloak users tab content.
func (s *Server) renderIAMUsersTab(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{}

	if s.keycloakAdmin != nil {
		search := r.URL.Query().Get("search")
		users, total, err := s.keycloakAdmin.ListUsers(r.Context(), search, 0, 50)
		if err != nil {
			data["Error"] = err.Error()
		} else {
			// Fetch roles for each user
			for i := range users {
				roles, err := s.keycloakAdmin.GetUserRoles(r.Context(), users[i].ID)
				if err != nil {
					log.Printf("[IAM] failed to get roles for user %s: %v", users[i].ID, err)
					continue
				}
				roleNames := make([]string, 0, len(roles))
				for _, role := range roles {
					roleNames = append(roleNames, role.Name)
				}
				users[i].RealmRoles = roleNames
			}
			data["Users"] = users
			data["TotalUsers"] = total
			data["Search"] = search
		}

		// Get available realm roles for assignment
		realmRoles, err := s.keycloakAdmin.GetRealmRoles(r.Context())
		if err != nil {
			log.Printf("[IAM] failed to get realm roles: %v", err)
		}
		data["RealmRoles"] = realmRoles
	} else {
		data["NotConfigured"] = true
	}

	s.templates.RenderPartial(w, "tab_iam_users.html", data)
}

// renderIAMPoliciesTab renders the OPA policies tab content.
func (s *Server) renderIAMPoliciesTab(w http.ResponseWriter, r *http.Request, templateName string) {
	data := map[string]any{}

	if s.opa != nil {
		data["Policies"] = s.opa.GetPolicies()
	} else {
		data["NotConfigured"] = true
	}

	s.templates.RenderPartial(w, templateName, data)
}

// renderIAMAuditTab renders the audit log tab content.
func (s *Server) renderIAMAuditTab(w http.ResponseWriter, r *http.Request, templateName string) {
	data := map[string]any{}

	if s.auditLog != nil {
		userFilter := r.URL.Query().Get("user")
		resourceFilter := r.URL.Query().Get("resource")
		actionFilter := r.URL.Query().Get("action")

		entries := s.auditLog.ListFiltered(userFilter, resourceFilter, actionFilter, 100)
		data["Entries"] = entries
		data["TotalCount"] = s.auditLog.Count()
		data["UserFilter"] = userFilter
		data["ResourceFilter"] = resourceFilter
		data["ActionFilter"] = actionFilter
	} else {
		data["NotConfigured"] = true
	}

	s.templates.RenderPartial(w, templateName, data)
}

// CreateKeycloakUserHandler handles POST /users/keycloak.
func (s *Server) CreateKeycloakUserHandler(w http.ResponseWriter, r *http.Request) {
	if s.keycloakAdmin == nil {
		http.Error(w, "Keycloak admin not configured", http.StatusServiceUnavailable)
		return
	}

	username := r.FormValue("username")
	email := r.FormValue("email")
	firstName := r.FormValue("first_name")
	lastName := r.FormValue("last_name")
	password := r.FormValue("password")

	if username == "" || email == "" {
		http.Error(w, "Username and email are required", http.StatusBadRequest)
		return
	}

	user := &auth.KeycloakUser{
		Username:      username,
		Email:         email,
		FirstName:     firstName,
		LastName:      lastName,
		Enabled:       true,
		EmailVerified: true,
	}

	userID, err := s.keycloakAdmin.CreateUser(r.Context(), user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if password != "" {
		if err := s.keycloakAdmin.ResetPassword(r.Context(), userID, password, false); err != nil {
			log.Printf("[IAM] user %s created but password reset failed: %v", userID, err)
			http.Error(w, "User created but password set failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("HX-Redirect", "/users?tab=users")
	w.WriteHeader(http.StatusOK)
}

// DeleteKeycloakUserHandler handles DELETE /users/keycloak/{id}.
func (s *Server) DeleteKeycloakUserHandler(w http.ResponseWriter, r *http.Request) {
	if s.keycloakAdmin == nil {
		http.Error(w, "Keycloak admin not configured", http.StatusServiceUnavailable)
		return
	}

	userID := r.PathValue("id")
	if err := s.keycloakAdmin.DeleteUser(r.Context(), userID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/users?tab=users")
	w.WriteHeader(http.StatusOK)
}

// ToggleKeycloakUserHandler handles POST /users/keycloak/{id}/toggle.
func (s *Server) ToggleKeycloakUserHandler(w http.ResponseWriter, r *http.Request) {
	if s.keycloakAdmin == nil {
		http.Error(w, "Keycloak admin not configured", http.StatusServiceUnavailable)
		return
	}

	userID := r.PathValue("id")
	user, err := s.keycloakAdmin.GetUser(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	user.Enabled = !user.Enabled
	if err := s.keycloakAdmin.UpdateUser(r.Context(), user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/users?tab=users")
	w.WriteHeader(http.StatusOK)
}

// AssignKeycloakRoleHandler handles POST /users/keycloak/{id}/roles.
func (s *Server) AssignKeycloakRoleHandler(w http.ResponseWriter, r *http.Request) {
	if s.keycloakAdmin == nil {
		http.Error(w, "Keycloak admin not configured", http.StatusServiceUnavailable)
		return
	}

	userID := r.PathValue("id")
	roleID := r.FormValue("role_id")
	roleName := r.FormValue("role_name")

	if roleID == "" || roleName == "" {
		http.Error(w, "role_id and role_name are required", http.StatusBadRequest)
		return
	}

	role := []auth.KeycloakRole{{ID: roleID, Name: roleName}}
	if err := s.keycloakAdmin.AssignUserRole(r.Context(), userID, role); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/users?tab=users")
	w.WriteHeader(http.StatusOK)
}

// SavePolicyHandler handles POST /users/policies.
func (s *Server) SavePolicyHandler(w http.ResponseWriter, r *http.Request) {
	if s.opa == nil {
		http.Error(w, "OPA not configured", http.StatusServiceUnavailable)
		return
	}

	name := r.FormValue("name")
	source := r.FormValue("source")

	if name == "" || source == "" {
		http.Error(w, "Policy name and source are required", http.StatusBadRequest)
		return
	}

	if err := s.opa.UpdatePolicy(name, source); err != nil {
		http.Error(w, "Invalid Rego: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Redirect", "/users?tab=policies")
	w.WriteHeader(http.StatusOK)
}

// --- JSON API handlers for IAM ---

// APIAuditLogHandler handles GET /api/audit.
func (s *Server) APIAuditLogHandler(w http.ResponseWriter, r *http.Request) {
	if s.auditLog == nil {
		writeJSON(w, http.StatusOK, map[string]any{"entries": []any{}, "total": 0})
		return
	}

	userFilter := r.URL.Query().Get("user")
	resourceFilter := r.URL.Query().Get("resource")
	actionFilter := r.URL.Query().Get("action")

	entries := s.auditLog.ListFiltered(userFilter, resourceFilter, actionFilter, 200)
	writeJSON(w, http.StatusOK, map[string]any{
		"entries": entries,
		"total":   s.auditLog.Count(),
	})
}

// APIKeycloakUsersHandler handles GET /api/users.
func (s *Server) APIKeycloakUsersHandler(w http.ResponseWriter, r *http.Request) {
	if s.keycloakAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "keycloak admin not configured"})
		return
	}

	search := r.URL.Query().Get("search")
	users, total, err := s.keycloakAdmin.ListUsers(r.Context(), search, 0, 50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"users": users,
		"total": total,
	})
}

// APIPoliciesHandler handles GET /api/policies.
func (s *Server) APIPoliciesHandler(w http.ResponseWriter, r *http.Request) {
	if s.opa == nil {
		writeJSON(w, http.StatusOK, map[string]any{"policies": map[string]string{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policies": s.opa.GetPolicies()})
}

// APISavePolicyHandler handles PUT /api/policies.
func (s *Server) APISavePolicyHandler(w http.ResponseWriter, r *http.Request) {
	if s.opa == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "OPA not configured"})
		return
	}

	var body struct {
		Name   string `json:"name"`
		Source string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := s.opa.UpdatePolicy(body.Name, body.Source); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
