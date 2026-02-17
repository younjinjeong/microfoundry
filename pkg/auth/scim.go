package auth

import (
	"fmt"
	"strings"
	"time"
)

// SCIM v2 schema URNs (RFC 7643).
const (
	SCIMSchemaUser         = "urn:ietf:params:scim:schemas:core:2.0:User"
	SCIMSchemaListResponse = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	SCIMSchemaError        = "urn:ietf:params:scim:api:messages:2.0:Error"
	SCIMSchemaPatchOp      = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
)

// SCIMUser represents a SCIM v2 User resource.
type SCIMUser struct {
	Schemas    []string      `json:"schemas"`
	ID         string        `json:"id,omitempty"`
	ExternalID string        `json:"externalId,omitempty"`
	UserName   string        `json:"userName"`
	Name       *SCIMName     `json:"name,omitempty"`
	Emails     []SCIMEmail   `json:"emails,omitempty"`
	Active     bool          `json:"active"`
	Meta       *SCIMMetadata `json:"meta,omitempty"`
}

// SCIMName holds the name components.
type SCIMName struct {
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
	Formatted  string `json:"formatted,omitempty"`
}

// SCIMEmail represents an email entry.
type SCIMEmail struct {
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Primary bool   `json:"primary,omitempty"`
}

// SCIMMetadata holds resource metadata.
type SCIMMetadata struct {
	ResourceType string `json:"resourceType"`
	Created      string `json:"created,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	Location     string `json:"location,omitempty"`
}

// SCIMListResponse is a paginated list of SCIM resources.
type SCIMListResponse struct {
	Schemas      []string   `json:"schemas"`
	TotalResults int        `json:"totalResults"`
	ItemsPerPage int        `json:"itemsPerPage"`
	StartIndex   int        `json:"startIndex"`
	Resources    []SCIMUser `json:"Resources"`
}

// SCIMError represents a SCIM error response.
type SCIMError struct {
	Schemas []string `json:"schemas"`
	Detail  string   `json:"detail"`
	Status  string   `json:"status"`
}

// SCIMPatchOp represents a SCIM PATCH request body.
type SCIMPatchOp struct {
	Schemas    []string        `json:"schemas"`
	Operations []SCIMOperation `json:"Operations"`
}

// SCIMOperation is a single SCIM patch operation.
type SCIMOperation struct {
	Op    string `json:"op"`
	Path  string `json:"path,omitempty"`
	Value any    `json:"value,omitempty"`
}

// KeycloakUserToSCIM converts a Keycloak user to a SCIM User resource.
func KeycloakUserToSCIM(user *KeycloakUser, baseURL string) SCIMUser {
	scim := SCIMUser{
		Schemas:  []string{SCIMSchemaUser},
		ID:       user.ID,
		UserName: user.Username,
		Active:   user.Enabled,
	}

	if user.FirstName != "" || user.LastName != "" {
		scim.Name = &SCIMName{
			GivenName:  user.FirstName,
			FamilyName: user.LastName,
			Formatted:  strings.TrimSpace(user.FirstName + " " + user.LastName),
		}
	}

	if user.Email != "" {
		scim.Emails = []SCIMEmail{
			{Value: user.Email, Type: "work", Primary: true},
		}
	}

	created := ""
	if user.CreatedTimestamp > 0 {
		created = time.UnixMilli(user.CreatedTimestamp).UTC().Format(time.RFC3339)
	}

	scim.Meta = &SCIMMetadata{
		ResourceType: "User",
		Created:      created,
		Location:     baseURL + "/scim/v2/Users/" + user.ID,
	}

	return scim
}

// SCIMUserToKeycloak converts a SCIM User to a Keycloak user for creation/update.
func SCIMUserToKeycloak(scim *SCIMUser) *KeycloakUser {
	user := &KeycloakUser{
		ID:       scim.ID,
		Username: scim.UserName,
		Enabled:  scim.Active,
	}

	if scim.Name != nil {
		user.FirstName = scim.Name.GivenName
		user.LastName = scim.Name.FamilyName
	}

	if len(scim.Emails) > 0 {
		for _, email := range scim.Emails {
			if email.Primary {
				user.Email = email.Value
				user.EmailVerified = true
				break
			}
		}
		if user.Email == "" {
			user.Email = scim.Emails[0].Value
		}
	}

	return user
}

// ParseSCIMFilter parses a basic SCIM filter expression.
// Supports: attrName op "value" (e.g., userName eq "john").
func ParseSCIMFilter(filter string) (attr, op, value string, err error) {
	if filter == "" {
		return "", "", "", nil
	}

	parts := strings.SplitN(filter, " ", 3)
	if len(parts) < 3 {
		return "", "", "", fmt.Errorf("invalid SCIM filter: %q", filter)
	}

	attr = parts[0]
	op = strings.ToLower(parts[1])
	value = strings.Trim(parts[2], "\"")

	switch op {
	case "eq", "co", "sw":
		// supported operators
	default:
		return "", "", "", fmt.Errorf("unsupported SCIM filter operator: %q", op)
	}

	return attr, op, value, nil
}

// NewSCIMError creates a SCIM error response.
func NewSCIMError(status int, detail string) SCIMError {
	return SCIMError{
		Schemas: []string{SCIMSchemaError},
		Detail:  detail,
		Status:  fmt.Sprintf("%d", status),
	}
}

// NewSCIMListResponse creates a paginated SCIM list response.
func NewSCIMListResponse(users []SCIMUser, totalResults, startIndex, itemsPerPage int) SCIMListResponse {
	if users == nil {
		users = []SCIMUser{}
	}
	return SCIMListResponse{
		Schemas:      []string{SCIMSchemaListResponse},
		TotalResults: totalResults,
		ItemsPerPage: itemsPerPage,
		StartIndex:   startIndex,
		Resources:    users,
	}
}
