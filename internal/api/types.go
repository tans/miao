// Package api defines MIAO's public HTTP contract and the generators that
// publish it as OpenAPI, JSON Schema, and Markdown reference material.
package api

// ErrorResponse is returned when an HTTP request cannot be completed.
type ErrorResponse struct {
	// Error is the localized human-readable error message.
	Error string `json:"error"`
}

// HealthResponse describes the process and persistence health of MIAO.
type HealthResponse struct {
	// OK is true when the service and its persistence are available.
	OK bool `json:"ok"`
	// Service is the product service name.
	Service string `json:"service"`
	// Persistence identifies the backing store.
	Persistence string `json:"persistence"`
	// Runtime identifies the server implementation.
	Runtime string `json:"runtime"`
	// Time is the RFC3339 response timestamp.
	Time string `json:"time"`
}

// TokenResponse is the common authentication response envelope. User and
// tenant are intentionally open objects until the legacy handlers migrate to
// named DTOs; the contract still keeps the envelope stable for clients.
type TokenResponse struct {
	// Token is the bearer token for subsequent authenticated requests.
	Token string `json:"token"`
	// User is the public user representation.
	User map[string]any `json:"user"`
	// Tenant is the selected workspace representation.
	Tenant map[string]any `json:"tenant"`
	// Workspaces contains the user's other available workspaces.
	Workspaces []any `json:"workspaces,omitempty"`
	// NeedsOnboard indicates that the account has no application yet.
	NeedsOnboard bool `json:"needs_onboarding,omitempty"`
}

// PageResponse is the common paginated response envelope used by list APIs.
type PageResponse struct {
	// Items contains the page records.
	Items []any `json:"items"`
	// Page is the one-based page number when pagination is enabled.
	Page int `json:"page,omitempty"`
	// PerPage is the requested page size when pagination is enabled.
	PerPage int `json:"perPage,omitempty"`
	// TotalItems is the total number of matching records.
	TotalItems int `json:"totalItems,omitempty"`
	// TotalPages is the total number of available pages.
	TotalPages int `json:"totalPages,omitempty"`
}

// CreateWorkspaceRequest creates a new workspace for the authenticated user.
type CreateWorkspaceRequest struct {
	// Name is the trimmed workspace display name.
	Name string `json:"name"`
}

// LoginRequest authenticates a user with an email and password.
type LoginRequest struct {
	// Email identifies the account.
	Email string `json:"email"`
	// Password is the account credential.
	Password string `json:"password"`
}

// RegisterRequest creates a user account. InviteToken is optional for open
// registration and required when the platform is configured for invitations.
type RegisterRequest struct {
	// Email is the account address.
	Email string `json:"email"`
	// Password is the initial account credential.
	Password string `json:"password"`
	// Name is the optional display name.
	Name string `json:"name,omitempty"`
	// InviteToken joins an invitation-only workspace.
	InviteToken string `json:"invite_token,omitempty"`
}

// VerifyEmailRequest confirms an email address using the emailed token.
type VerifyEmailRequest struct {
	// Token is the emailed verification token.
	Token string `json:"token"`
}

// PasswordResetRequest starts a password reset flow.
type PasswordResetRequest struct {
	// Email is the account address that receives the reset link.
	Email string `json:"email"`
}

// PasswordResetConfirmRequest completes a password reset flow.
type PasswordResetConfirmRequest struct {
	// Token is the reset token from the email.
	Token string `json:"token"`
	// Password is the replacement account credential.
	Password string `json:"password"`
}

// AnyObject is the migration boundary for handlers that still accept a
// validated but endpoint-specific JSON object. New endpoints should replace it
// with a named request or response DTO before adding business behavior.
type AnyObject map[string]any
