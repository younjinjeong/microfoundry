package auth

import (
	"net/http"
	"strings"
	"time"
)

// OPAMiddleware creates an HTTP middleware that evaluates OPA authorization policies.
// When opa is nil (auth disabled), all requests are allowed.
func OPAMiddleware(opa *OPAEngine, sessions *SessionManager, orgStore *OrgStore, auditLog *AuditLog) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip public paths
			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// If OPA engine is nil, allow everything (auth disabled)
			if opa == nil {
				next.ServeHTTP(w, r)
				return
			}

			input := buildAuthzInput(r, orgStore)
			result, err := opa.Evaluate(r.Context(), input)
			if err != nil {
				http.Error(w, "authorization error", http.StatusInternalServerError)
				return
			}

			// Record audit entry
			if auditLog != nil {
				email := ""
				userID := ""
				if input.User != nil {
					email = input.User.Email
					userID = input.User.ID
				}
				auditLog.Record(AuditEntry{
					Timestamp: time.Now().UTC(),
					UserEmail: email,
					UserID:    userID,
					Action:    input.Action,
					Resource:  input.Resource,
					Path:      r.URL.Path,
					Method:    r.Method,
					OrgID:     input.OrgID,
					Allowed:   result.Allow,
					Reason:    result.Reason,
					IP:        r.RemoteAddr,
				})
			}

			if !result.Allow {
				if input.User == nil {
					http.Redirect(w, r, "/login", http.StatusFound)
					return
				}
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isPublicPath returns true for paths that bypass OPA authorization.
func isPublicPath(path string) bool {
	publicPrefixes := []string{
		"/static/",
		"/metrics",
		"/login",
		"/auth/",
		"/health",
	}
	for _, p := range publicPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return path == "/" || path == "/favicon.ico"
}

// buildAuthzInput constructs the OPA input from the HTTP request context.
func buildAuthzInput(r *http.Request, orgStore *OrgStore) AuthzInput {
	user := UserFromContext(r.Context())
	action, resource := classifyRoute(r.Method, r.URL.Path)

	input := AuthzInput{
		Action:   action,
		Resource: resource,
		OrgID:    ActiveOrgFromContext(r.Context()),
		Path:     r.URL.Path,
		Method:   r.Method,
	}

	if user != nil {
		orgRole := ""
		if orgStore != nil && input.OrgID != "" {
			members, _ := orgStore.ListMembers(r.Context(), input.OrgID)
			for _, m := range members {
				if m.Email == user.Email {
					orgRole = m.Role
					break
				}
			}
		}
		input.User = &AuthzUser{
			ID:      user.UserID,
			Email:   user.Email,
			Roles:   user.Roles,
			OrgRole: orgRole,
		}
	}

	return input
}

// classifyRoute determines the action and resource type from HTTP method and path.
func classifyRoute(method, path string) (action, resource string) {
	// Determine action from HTTP method
	switch method {
	case "GET", "HEAD", "OPTIONS":
		action = "read"
	case "POST", "PUT", "PATCH":
		action = "write"
	case "DELETE":
		action = "delete"
	default:
		action = "read"
	}

	// Determine resource from path
	path = strings.TrimPrefix(path, "/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) == 0 {
		return action, "dashboard"
	}

	switch parts[0] {
	case "apps":
		resource = "apps"
	case "services":
		resource = "services"
	case "secrets":
		resource = "secrets"
	case "clusters":
		resource = "clusters"
	case "catalog", "topologies":
		resource = "catalog"
	case "monitoring":
		resource = "monitoring"
	case "config":
		resource = "settings"
	case "settings":
		resource = "settings"
	case "users":
		resource = "users"
	case "scim":
		resource = "scim"
	case "api":
		// For API routes, classify by the second path segment
		if len(parts) >= 2 {
			switch parts[1] {
			case "apps":
				resource = "apps"
			case "services":
				resource = "services"
			case "secrets":
				resource = "secrets"
			case "clusters":
				resource = "clusters"
			case "catalog", "topologies":
				resource = "catalog"
			case "orgs":
				resource = "users"
			case "settings":
				resource = "settings"
			case "monitoring":
				resource = "monitoring"
			case "config":
				resource = "settings"
			case "audit":
				resource = "audit"
			default:
				resource = parts[1]
			}
		} else {
			resource = "api"
		}
	default:
		resource = "unknown"
	}

	return action, resource
}
