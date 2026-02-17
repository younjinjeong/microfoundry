package admin

import (
	"encoding/json"
	"net/http"

	"github.com/younjinjeong/microfoundry/pkg/auth"
)

func (s *Server) orgStore() *auth.OrgStore {
	return s.orgStore_
}

// OrgsPageHandler renders the organization management page with tabbed layout.
func (s *Server) OrgsPageHandler(w http.ResponseWriter, r *http.Request) {
	data := s.pageDataWithUser(r, "Users & Organizations", "users")

	user := auth.UserFromContext(r.Context())
	if user == nil {
		// Auth not enabled — show setup instructions
		s.templates.Render(w, "users.html", data)
		return
	}

	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "orgs"
	}

	content := map[string]any{
		"Tab":  tab,
		"User": user,
	}

	switch tab {
	case "orgs":
		ctx := r.Context()
		store := s.orgStore()
		orgs, _ := store.GetUserOrgs(ctx, user.Email)

		selectedOrg := r.URL.Query().Get("org")
		if selectedOrg == "" && len(orgs) > 0 {
			selectedOrg = orgs[0].ID
		}

		content["Orgs"] = orgs
		content["SelectedOrg"] = selectedOrg
		content["ActiveOrg"] = user.ActiveOrgID

		if selectedOrg != "" {
			orgDetail, err := store.Get(ctx, selectedOrg)
			if err == nil {
				content["OrgDetail"] = orgDetail
				members, _ := store.ListMembers(ctx, selectedOrg)
				content["Members"] = members

				userRole := "viewer"
				for _, m := range members {
					if m.Email == user.Email {
						userRole = m.Role
						break
					}
				}
				content["UserRole"] = userRole
			}
		}

	case "users":
		if s.keycloakAdmin != nil {
			search := r.URL.Query().Get("search")
			users, total, err := s.keycloakAdmin.ListUsers(r.Context(), search, 0, 50)
			if err != nil {
				content["Error"] = err.Error()
			} else {
				for i := range users {
					roles, _ := s.keycloakAdmin.GetUserRoles(r.Context(), users[i].ID)
					roleNames := make([]string, 0, len(roles))
					for _, role := range roles {
						roleNames = append(roleNames, role.Name)
					}
					users[i].RealmRoles = roleNames
				}
				content["Users"] = users
				content["TotalUsers"] = total
				content["Search"] = search
			}
			realmRoles, _ := s.keycloakAdmin.GetRealmRoles(r.Context())
			content["RealmRoles"] = realmRoles
		} else {
			content["NotConfigured"] = true
		}

	case "policies":
		if s.opa != nil {
			content["Policies"] = s.opa.GetPolicies()
		} else {
			content["NotConfigured"] = true
		}

	case "audit":
		if s.auditLog != nil {
			userFilter := r.URL.Query().Get("user")
			resourceFilter := r.URL.Query().Get("resource")
			actionFilter := r.URL.Query().Get("action")
			entries := s.auditLog.ListFiltered(userFilter, resourceFilter, actionFilter, 100)
			content["Entries"] = entries
			content["TotalCount"] = s.auditLog.Count()
			content["UserFilter"] = userFilter
			content["ResourceFilter"] = resourceFilter
			content["ActionFilter"] = actionFilter
		} else {
			content["NotConfigured"] = true
		}
	}

	data.Content = content
	s.templates.Render(w, "users.html", data)
}

// CreateOrgHandler creates a new organization.
func (s *Server) CreateOrgHandler(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "Organization name is required", http.StatusBadRequest)
		return
	}

	store := s.orgStore()
	org, err := store.Create(r.Context(), name, user.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/users?org="+org.ID, http.StatusSeeOther)
}

// DeleteOrgHandler deletes an organization.
func (s *Server) DeleteOrgHandler(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	orgID := r.PathValue("id")
	store := s.orgStore()

	if err := store.Delete(r.Context(), orgID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/users")
	w.WriteHeader(http.StatusOK)
}

// InviteMemberHandler adds a member to an organization.
func (s *Server) InviteMemberHandler(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	orgID := r.PathValue("id")
	email := r.FormValue("email")
	name := r.FormValue("name")
	role := r.FormValue("role")

	if email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}
	if name == "" {
		name = email
	}
	if role == "" {
		role = "member"
	}

	store := s.orgStore()
	if err := store.AddMember(r.Context(), orgID, email, name, role); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/users?org="+orgID, http.StatusSeeOther)
}

// RemoveMemberHandler removes a member from an organization.
func (s *Server) RemoveMemberHandler(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	orgID := r.PathValue("id")
	email := r.PathValue("email")

	store := s.orgStore()
	if err := store.RemoveMember(r.Context(), orgID, email); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// SetMemberRoleHandler changes a member's role.
func (s *Server) SetMemberRoleHandler(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	orgID := r.PathValue("id")
	email := r.PathValue("email")
	role := r.FormValue("role")

	validRoles := map[string]bool{"admin": true, "member": true, "viewer": true}
	if !validRoles[role] {
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}

	store := s.orgStore()
	if err := store.SetMemberRole(r.Context(), orgID, email, role); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/users?org="+orgID, http.StatusSeeOther)
}

// SwitchOrgHandler sets the active organization for the user's session.
func (s *Server) SwitchOrgHandler(w http.ResponseWriter, r *http.Request) {
	if s.sessions == nil {
		http.Error(w, "Auth not enabled", http.StatusBadRequest)
		return
	}

	orgID := r.PathValue("id")
	if err := s.sessions.SetActiveOrg(w, r, orgID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/users?org="+orgID, http.StatusSeeOther)
}

// --- JSON API handlers ---

func (s *Server) APIListOrgsHandler(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	store := s.orgStore()
	orgs, err := store.GetUserOrgs(r.Context(), user.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, orgs)
}

func (s *Server) APIGetOrgHandler(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	orgID := r.PathValue("id")
	store := s.orgStore()

	org, err := store.Get(r.Context(), orgID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	members, _ := store.ListMembers(r.Context(), orgID)

	writeJSON(w, http.StatusOK, map[string]any{
		"org":     org,
		"members": members,
	})
}

func (s *Server) APICreateOrgHandler(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	store := s.orgStore()
	org, err := store.Create(r.Context(), body.Name, user.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, org)
}

func (s *Server) APIListMembersHandler(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	orgID := r.PathValue("id")
	store := s.orgStore()

	members, err := store.ListMembers(r.Context(), orgID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, members)
}
