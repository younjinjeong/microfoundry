package microfoundry.authz

import rego.v1

default allow := false

# When auth is disabled, user is null — allow everything
allow if {
	input.user == null
}

# Platform admins have full access to all resources
allow if {
	"platform-admin" in input.user.roles
}

# Any authenticated user can read any resource
allow if {
	input.action == "read"
	input.user.id != ""
}

# Org admins can write/update resources within their org
allow if {
	input.action == "write"
	input.user.org_role == "admin"
}

# Org members can write apps, services, and secrets
allow if {
	input.action == "write"
	input.user.org_role == "member"
	input.resource in {"apps", "services", "secrets"}
}

# SCIM endpoints require platform-admin
allow if {
	input.resource == "scim"
	"platform-admin" in input.user.roles
}

# Settings and cluster management require platform-admin for writes
allow if {
	input.resource in {"settings", "clusters", "catalog"}
	input.action in {"write", "delete", "admin"}
	"platform-admin" in input.user.roles
}

# Delete requires org admin role
allow if {
	input.action == "delete"
	input.user.org_role == "admin"
	input.resource in {"apps", "services", "secrets"}
}

# User/org management: org admins can manage their own org
allow if {
	input.resource == "users"
	input.action == "write"
	input.user.org_role == "admin"
}
