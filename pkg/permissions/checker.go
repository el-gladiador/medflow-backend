// Package permissions provides utilities for checking JSONB array permissions
// against required permissions with support for wildcards.
//
// Permission Format:
//   - "*" - Full access (all permissions)
//   - "resource.*" - All actions on a resource (e.g., "inventory.*")
//   - "resource.action" - Specific action (e.g., "inventory.read")
//   - "resource.subresource.action" - Nested permission (e.g., "staff.documents.upload")
package permissions

import (
	"strings"
)

// HasPermission checks if the user's permissions include the required permission.
// Supports wildcard matching:
//   - "*" matches everything
//   - "inventory.*" matches "inventory.read", "inventory.write", etc.
//   - Exact match for specific permissions
func HasPermission(userPerms []string, required string) bool {
	if required == "" {
		return true // No permission required
	}

	for _, p := range userPerms {
		if p == "*" {
			return true // Full admin access
		}
		if p == required {
			return true // Exact match
		}
		// Check wildcard patterns like "inventory.*"
		if strings.HasSuffix(p, ".*") {
			prefix := strings.TrimSuffix(p, ".*")
			if strings.HasPrefix(required, prefix+".") {
				return true
			}
		}
	}
	return false
}

// HasAnyPermission checks if the user has any of the required permissions.
func HasAnyPermission(userPerms []string, required []string) bool {
	for _, req := range required {
		if HasPermission(userPerms, req) {
			return true
		}
	}
	return false
}

// HasAllPermissions checks if the user has all of the required permissions.
func HasAllPermissions(userPerms []string, required []string) bool {
	for _, req := range required {
		if !HasPermission(userPerms, req) {
			return false
		}
	}
	return true
}

// ExpandWildcard expands a wildcard permission pattern to check if it covers
// a set of specific permissions. Returns the list of permissions that would be covered.
func ExpandWildcard(pattern string, allKnownPerms []string) []string {
	if pattern == "*" {
		return allKnownPerms
	}

	if !strings.HasSuffix(pattern, ".*") {
		// Not a wildcard, return as-is if it exists
		for _, p := range allKnownPerms {
			if p == pattern {
				return []string{pattern}
			}
		}
		return nil
	}

	prefix := strings.TrimSuffix(pattern, ".*")
	var matches []string
	for _, p := range allKnownPerms {
		if strings.HasPrefix(p, prefix+".") {
			matches = append(matches, p)
		}
	}
	return matches
}

// FilterByPrefix returns all permissions that match a given prefix.
// Useful for getting all permissions in a category (e.g., "inventory").
func FilterByPrefix(perms []string, prefix string) []string {
	var matches []string
	for _, p := range perms {
		if strings.HasPrefix(p, prefix+".") || p == prefix {
			matches = append(matches, p)
		}
	}
	return matches
}

// MergePermissions merges multiple permission sets, removing duplicates.
// Useful for combining role permissions with permission overrides.
func MergePermissions(sets ...[]string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, set := range sets {
		for _, p := range set {
			if !seen[p] {
				seen[p] = true
				result = append(result, p)
			}
		}
	}

	return result
}

// RemovePermissions removes specific permissions from a set.
// Useful for applying permission revocations.
func RemovePermissions(perms []string, toRemove []string) []string {
	removeSet := make(map[string]bool)
	for _, p := range toRemove {
		removeSet[p] = true
	}

	var result []string
	for _, p := range perms {
		if !removeSet[p] {
			result = append(result, p)
		}
	}

	return result
}

// CommonPermissions is a list of standard permissions used in MedFlow.
// This can be used for validation and autocomplete.
var CommonPermissions = []string{
	// Inventory permissions
	"inventory.read",
	"inventory.write",
	"inventory.delete",
	"inventory.adjust",
	"inventory.transfer",
	"inventory.alerts.manage",
	"inventory.*",

	// Staff permissions
	"staff.read",
	"staff.write",
	"staff.delete",
	"staff.documents.read",
	"staff.documents.upload",
	"staff.documents.delete",
	"staff.financials.read",
	"staff.financials.write",
	"staff.*",

	// User permissions
	"users.read",
	"users.write",
	"users.delete",
	"users.roles.assign",
	"users.permissions.override",
	"users.*",

	// Reports permissions
	"reports.read",
	"reports.generate",
	"reports.export",
	"reports.*",

	// Profile permissions (self-management)
	"profile.read",
	"profile.update",
	"profile.password.change",
	"profile.*",

	// Admin permissions
	"admin.settings",
	"admin.audit.read",
	"admin.tenant.manage",
	"admin.*",

	// Full access
	"*",
}

// IsValidPermission checks if a permission string is in the known list.
// Allows wildcards and custom permissions not in the standard list.
func IsValidPermission(perm string) bool {
	// Allow wildcard
	if perm == "*" {
		return true
	}

	// Check against known permissions
	for _, p := range CommonPermissions {
		if p == perm {
			return true
		}
	}

	// Allow any permission that follows the pattern resource.action
	parts := strings.Split(perm, ".")
	return len(parts) >= 2
}
