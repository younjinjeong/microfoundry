package admin

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"

	"github.com/younjinjeong/microfoundry/pkg/auth"
)

// uuidRegex validates UUID format for SCIM path parameters.
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func isValidUUID(s string) bool {
	return uuidRegex.MatchString(s)
}

// writeSCIMJSON writes a SCIM JSON response with the correct content type.
func writeSCIMJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[SCIM] failed to encode JSON response: %v", err)
	}
}

// writeSCIMError sends a SCIM error response.
func writeSCIMError(w http.ResponseWriter, status int, detail string) {
	writeSCIMJSON(w, status, auth.NewSCIMError(status, detail))
}

// scimBaseURL returns the external SCIM base URL from the request.
func scimBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

// --- SCIM v2 Handlers ---

// SCIMListUsersHandler handles GET /scim/v2/Users.
func (s *Server) SCIMListUsersHandler(w http.ResponseWriter, r *http.Request) {
	if s.keycloakAdmin == nil {
		writeSCIMError(w, http.StatusServiceUnavailable, "Keycloak admin not configured")
		return
	}

	startIndex, _ := strconv.Atoi(r.URL.Query().Get("startIndex"))
	if startIndex < 1 {
		startIndex = 1
	}
	count, _ := strconv.Atoi(r.URL.Query().Get("count"))
	if count <= 0 {
		count = 20
	}
	if count > 100 {
		count = 100
	}

	filter := r.URL.Query().Get("filter")
	search := ""
	if filter != "" {
		attr, op, value, err := auth.ParseSCIMFilter(filter)
		if err != nil {
			writeSCIMError(w, http.StatusBadRequest, err.Error())
			return
		}
		if attr == "userName" || attr == "emails.value" {
			switch op {
			case "eq", "co", "sw":
				search = value
			}
		}
	}

	first := startIndex - 1 // SCIM is 1-based, Keycloak is 0-based
	users, total, err := s.keycloakAdmin.ListUsers(r.Context(), search, first, count)
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, err.Error())
		return
	}

	baseURL := scimBaseURL(r)
	scimUsers := make([]auth.SCIMUser, 0, len(users))
	for _, u := range users {
		scimUsers = append(scimUsers, auth.KeycloakUserToSCIM(&u, baseURL))
	}

	resp := auth.NewSCIMListResponse(scimUsers, total, startIndex, count)
	writeSCIMJSON(w, http.StatusOK, resp)
}

// SCIMCreateUserHandler handles POST /scim/v2/Users.
func (s *Server) SCIMCreateUserHandler(w http.ResponseWriter, r *http.Request) {
	if s.keycloakAdmin == nil {
		writeSCIMError(w, http.StatusServiceUnavailable, "Keycloak admin not configured")
		return
	}

	var scimUser auth.SCIMUser
	if err := json.NewDecoder(r.Body).Decode(&scimUser); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if scimUser.UserName == "" {
		writeSCIMError(w, http.StatusBadRequest, "userName is required")
		return
	}

	kcUser := auth.SCIMUserToKeycloak(&scimUser)
	userID, err := s.keycloakAdmin.CreateUser(r.Context(), kcUser)
	if err != nil {
		writeSCIMError(w, http.StatusConflict, err.Error())
		return
	}

	// Fetch the created user to return full representation
	created, err := s.keycloakAdmin.GetUser(r.Context(), userID)
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, err.Error())
		return
	}

	baseURL := scimBaseURL(r)
	result := auth.KeycloakUserToSCIM(created, baseURL)

	w.Header().Set("Location", result.Meta.Location)
	writeSCIMJSON(w, http.StatusCreated, result)
}

// SCIMGetUserHandler handles GET /scim/v2/Users/{id}.
func (s *Server) SCIMGetUserHandler(w http.ResponseWriter, r *http.Request) {
	if s.keycloakAdmin == nil {
		writeSCIMError(w, http.StatusServiceUnavailable, "Keycloak admin not configured")
		return
	}

	userID := r.PathValue("id")
	if !isValidUUID(userID) {
		writeSCIMError(w, http.StatusBadRequest, "invalid user ID format")
		return
	}
	user, err := s.keycloakAdmin.GetUser(r.Context(), userID)
	if err != nil {
		writeSCIMError(w, http.StatusNotFound, err.Error())
		return
	}

	baseURL := scimBaseURL(r)
	writeSCIMJSON(w, http.StatusOK, auth.KeycloakUserToSCIM(user, baseURL))
}

// SCIMUpdateUserHandler handles PUT /scim/v2/Users/{id}.
func (s *Server) SCIMUpdateUserHandler(w http.ResponseWriter, r *http.Request) {
	if s.keycloakAdmin == nil {
		writeSCIMError(w, http.StatusServiceUnavailable, "Keycloak admin not configured")
		return
	}

	userID := r.PathValue("id")
	if !isValidUUID(userID) {
		writeSCIMError(w, http.StatusBadRequest, "invalid user ID format")
		return
	}

	var scimUser auth.SCIMUser
	if err := json.NewDecoder(r.Body).Decode(&scimUser); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	kcUser := auth.SCIMUserToKeycloak(&scimUser)
	kcUser.ID = userID

	if err := s.keycloakAdmin.UpdateUser(r.Context(), kcUser); err != nil {
		writeSCIMError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, err := s.keycloakAdmin.GetUser(r.Context(), userID)
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, err.Error())
		return
	}

	baseURL := scimBaseURL(r)
	writeSCIMJSON(w, http.StatusOK, auth.KeycloakUserToSCIM(updated, baseURL))
}

// SCIMPatchUserHandler handles PATCH /scim/v2/Users/{id}.
func (s *Server) SCIMPatchUserHandler(w http.ResponseWriter, r *http.Request) {
	if s.keycloakAdmin == nil {
		writeSCIMError(w, http.StatusServiceUnavailable, "Keycloak admin not configured")
		return
	}

	userID := r.PathValue("id")
	if !isValidUUID(userID) {
		writeSCIMError(w, http.StatusBadRequest, "invalid user ID format")
		return
	}

	var patch auth.SCIMPatchOp
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.keycloakAdmin.GetUser(r.Context(), userID)
	if err != nil {
		writeSCIMError(w, http.StatusNotFound, err.Error())
		return
	}

	for _, op := range patch.Operations {
		switch op.Op {
		case "replace":
			switch op.Path {
			case "active":
				if v, ok := op.Value.(bool); ok {
					user.Enabled = v
				}
			case "userName":
				if v, ok := op.Value.(string); ok {
					user.Username = v
				}
			case "name.givenName":
				if v, ok := op.Value.(string); ok {
					user.FirstName = v
				}
			case "name.familyName":
				if v, ok := op.Value.(string); ok {
					user.LastName = v
				}
			}
		}
	}

	if err := s.keycloakAdmin.UpdateUser(r.Context(), user); err != nil {
		writeSCIMError(w, http.StatusInternalServerError, err.Error())
		return
	}

	baseURL := scimBaseURL(r)
	writeSCIMJSON(w, http.StatusOK, auth.KeycloakUserToSCIM(user, baseURL))
}

// SCIMDeleteUserHandler handles DELETE /scim/v2/Users/{id}.
func (s *Server) SCIMDeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	if s.keycloakAdmin == nil {
		writeSCIMError(w, http.StatusServiceUnavailable, "Keycloak admin not configured")
		return
	}

	userID := r.PathValue("id")
	if !isValidUUID(userID) {
		writeSCIMError(w, http.StatusBadRequest, "invalid user ID format")
		return
	}
	if err := s.keycloakAdmin.DeleteUser(r.Context(), userID); err != nil {
		writeSCIMError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SCIMServiceProviderConfigHandler handles GET /scim/v2/ServiceProviderConfig.
func (s *Server) SCIMServiceProviderConfigHandler(w http.ResponseWriter, r *http.Request) {
	config := map[string]any{
		"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		"patch": map[string]any{
			"supported": true,
		},
		"bulk": map[string]any{
			"supported":      false,
			"maxOperations":  0,
			"maxPayloadSize": 0,
		},
		"filter": map[string]any{
			"supported":  true,
			"maxResults": 100,
		},
		"changePassword": map[string]any{
			"supported": true,
		},
		"sort": map[string]any{
			"supported": false,
		},
		"etag": map[string]any{
			"supported": false,
		},
		"authenticationSchemes": []map[string]any{
			{
				"type":        "oauthbearertoken",
				"name":        "OAuth Bearer Token",
				"description": "Authentication using bearer tokens via Keycloak OIDC",
			},
		},
	}
	writeSCIMJSON(w, http.StatusOK, config)
}

// SCIMResourceTypesHandler handles GET /scim/v2/ResourceTypes.
func (s *Server) SCIMResourceTypesHandler(w http.ResponseWriter, r *http.Request) {
	resourceTypes := []map[string]any{
		{
			"schemas":     []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"},
			"id":          "User",
			"name":        "User",
			"endpoint":    "/scim/v2/Users",
			"description": "User Account",
			"schema":      auth.SCIMSchemaUser,
		},
	}
	writeSCIMJSON(w, http.StatusOK, resourceTypes)
}

// SCIMSchemasHandler handles GET /scim/v2/Schemas.
func (s *Server) SCIMSchemasHandler(w http.ResponseWriter, r *http.Request) {
	schemas := []map[string]any{
		{
			"id":          auth.SCIMSchemaUser,
			"name":        "User",
			"description": "User Account",
			"attributes": []map[string]any{
				{"name": "userName", "type": "string", "required": true, "uniqueness": "server"},
				{"name": "name", "type": "complex"},
				{"name": "emails", "type": "complex", "multiValued": true},
				{"name": "active", "type": "boolean"},
			},
		},
	}
	writeSCIMJSON(w, http.StatusOK, schemas)
}
