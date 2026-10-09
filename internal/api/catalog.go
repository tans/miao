package api

import (
	"reflect"
	"sort"
	"strings"

	"github.com/tans/miao/internal/settings"
)

// Operation is one documented HTTP operation. Auth is one of public, user, or
// admin; SideEffects identifies operations that can change persisted state.
type Operation struct {
	// Method is the uppercase HTTP method.
	Method string
	// Path is the net/http route pattern.
	Path string
	// OperationID is the stable client-generation identifier.
	OperationID string
	// Summary is the short human-readable operation title.
	Summary string
	// Description explains authentication and side effects.
	Description string
	// Auth is public, user, or admin.
	Auth string
	// SideEffects reports whether the operation may write persisted state.
	SideEffects bool
	// RequestSchema names the request DTO schema, when a body is accepted.
	RequestSchema string
	// ResponseSchema names the success response DTO schema.
	ResponseSchema string
}

// Contract is the complete versioned API catalog used by all documentation
// outputs. Operations are sorted by path and method when returned by Current.
type Contract struct {
	// Title is the document title.
	Title string
	// Version is the API contract version.
	Version string
	// Description explains the contract boundary.
	Description string
	// Operations is the complete route catalog.
	Operations []Operation
}

const routeCatalog = `
GET /api/health
GET /api/openapi.json
GET /api/schema.json
GET /api/config-schema.json
POST /api/auth/register
POST /api/auth/login
POST /api/auth/verify-email
POST /api/auth/password-reset/request
POST /api/auth/password-reset/confirm
POST /api/auth/logout
GET /api/me
PATCH /api/me
DELETE /api/me
POST /api/me/deactivate
GET /api/workspace/members
PATCH /api/workspace/members/{id}
DELETE /api/workspace/members/{id}
GET /api/workspace/invites
POST /api/workspace/invites
DELETE /api/workspace/invites/{id}
POST /api/workspaces
POST /api/invites/accept
GET /api/workspace/ai-usage/requests
GET /api/workspace/ai-usage
PATCH /api/workspace/ai-budget
GET /api/workspace/audit
GET /api/workspace/export
GET /api/apps
POST /api/apps
GET /api/apps/{id}
PATCH /api/apps/{id}
DELETE /api/apps/{id}
GET /api/apps/{id}/collections
POST /api/apps/{id}/collections
PATCH /api/apps/{id}/collections/{slug}
DELETE /api/apps/{id}/collections/{slug}
GET /api/apps/{id}/collections/{slug}/records
POST /api/apps/{id}/collections/{slug}/records
GET /api/apps/{id}/collections/{slug}/records/{recordId}
PATCH /api/apps/{id}/collections/{slug}/records/{recordId}
DELETE /api/apps/{id}/collections/{slug}/records/{recordId}
GET /api/apps/{id}/collections/{slug}/records/{recordId}/files/{fieldName}
GET /api/apps/{id}/access
GET /api/apps/{id}/members
PUT /api/apps/{id}/access
POST /api/apps/{id}/visit
GET /api/apps/{id}/context
PUT /api/apps/{id}/context
GET /api/apps/{id}/record-changes
POST /api/apps/{id}/record-changes/{changeId}/restore
GET /api/apps/{id}/runtime
GET /api/apps/{id}/runtime/stream
GET /api/apps/{id}/versions
GET /api/apps/{id}/versions/{versionId}
POST /api/apps/{id}/versions/{versionId}/activate
GET /api/apps/{id}/versions/{versionId}/preview
GET /api/apps/{id}/versions/{versionId}/validation
GET /api/apps/{id}/versions/{versionId}/diff
POST /api/apps/{id}/runtime/actions/{actionId}
POST /api/apps/{id}/versions/preview
GET /api/apps/{id}/publication
GET /api/apps/{id}/backend/catalog
GET /api/apps/{id}/backend/spec
GET /api/apps/{id}/backend/candidates
POST /api/apps/{id}/backend/plans
GET /api/apps/{id}/backend/plans/{planId}
POST /api/apps/{id}/backend/plans/{planId}/apply
GET /api/apps/{id}/actions
POST /api/apps/{id}/actions
PATCH /api/apps/{id}/actions/{actionId}
POST /api/apps/{id}/actions/{actionId}/enable
POST /api/apps/{id}/actions/{actionId}/execute
GET /api/apps/{id}/automations
POST /api/apps/{id}/automations
POST /api/apps/{id}/automations/{ruleId}/enable
GET /api/notifications
POST /api/notifications/{notificationId}/read
GET /api/apps/{id}/workflows
POST /api/apps/{id}/workflows
PATCH /api/apps/{id}/workflows/{workflowId}
POST /api/apps/{id}/workflows/{workflowId}/enable
POST /api/apps/{id}/workflows/{workflowId}/transition
GET /api/apps/{id}/connectors
POST /api/apps/{id}/connectors
PATCH /api/apps/{id}/connectors/{connectorId}
POST /api/apps/{id}/connectors/{connectorId}/enable
POST /api/apps/{id}/connectors/{connectorId}/fetch
GET /api/apps/{id}/collection-scripts
POST /api/apps/{id}/collection-scripts
PATCH /api/apps/{id}/collection-scripts/{scriptId}
POST /api/apps/{id}/collection-scripts/{scriptId}/{action}
GET /api/apps/{id}/collection-scripts/{scriptId}/runs
GET /api/apps/{id}/collection-scripts/{scriptId}/runs/{runId}
POST /api/apps/{id}/collection-scripts/{scriptId}/runs/{runId}/retry-notifications
GET /api/apps/{id}/tasks
POST /api/apps/{id}/tasks
PATCH /api/apps/{id}/tasks/{taskId}
POST /api/apps/{id}/tasks/{taskId}/{action}
GET /api/apps/{id}/runs
GET /api/apps/{id}/runs/{runId}
POST /api/apps/{id}/runs/{runId}/{action}
GET /api/apps/{id}/files
POST /api/apps/{id}/files
GET /api/apps/{id}/files/{fileId}/content
POST /api/apps/{id}/files/{fileId}/attach
GET /api/apps/{id}/files/{fileId}/download
POST /api/apps/{id}/query
POST /api/apps/{id}/batch-plans
GET /api/apps/{id}/batch-plans/{jobId}
POST /api/apps/{id}/batch-plans/{jobId}/commit
POST /api/apps/{id}/import-plans
GET /api/apps/{id}/import-plans/{planId}
POST /api/apps/{id}/import-plans/{planId}/commit
GET /api/agent/threads
POST /api/agent/threads
GET /api/agent/threads/{threadId}/messages
POST /api/agent/threads/{threadId}/messages
GET /api/agent/conversation
PUT /api/agent/conversation
DELETE /api/agent/conversation
GET /api/agent/runs/{runId}
GET /api/agent/runs/{runId}/events
GET /api/agent/runs/{runId}/stream
POST /api/agent/runs
POST /api/agent/runs/{runId}/confirm
POST /api/agent/runs/{runId}/cancel
POST /api/agent/runs/{runId}/continue
POST /api/agent/runs/{runId}/resume
GET /api/build/templates
GET /api/admin/overview
GET /api/admin/runtime
GET /api/admin/settings
PUT /api/admin/settings/registration
PUT /api/admin/settings/mail
PUT /api/admin/settings/admins
PUT /api/admin/settings/backup
GET /api/admin/users
PATCH /api/admin/users/{id}/status
GET /api/admin/workspaces
GET /api/admin/apps
GET /api/admin/usage
GET /api/admin/audit
GET /api/admin/ai
PUT /api/admin/ai/{kind}
DELETE /api/admin/ai/{kind}
POST /api/admin/ai/{kind}/check
GET /api/admin/usage/requests
GET /api/public/{slug}/runtime
POST /api/public/{slug}/visit
GET /api/public/{slug}/records
GET /api/public/{slug}/records/{itemSlug}
GET /api/public/{slug}/images/{pageId}/{source}/{table}/{recordId}/{field}
`

// Current returns the single source of truth for the HTTP operation catalog.
func Current() Contract {
	operations := make([]Operation, 0, 140)
	for _, line := range strings.Split(routeCatalog, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 2 {
			continue
		}
		method, route := fields[0], normalizeRoute(fields[1])
		auth := authKind(route)
		schema := requestSchema(method, route)
		operations = append(operations, Operation{
			Method:         method,
			Path:           route,
			OperationID:    operationID(method, route),
			Summary:        method + " " + strings.TrimPrefix(route, "/api/"),
			Description:    operationDescription(method, route, auth),
			Auth:           auth,
			SideEffects:    method != "GET",
			RequestSchema:  schema,
			ResponseSchema: responseSchema(method, route),
		})
	}
	sort.Slice(operations, func(i, j int) bool {
		if operations[i].Path == operations[j].Path {
			return operations[i].Method < operations[j].Method
		}
		return operations[i].Path < operations[j].Path
	})
	return Contract{Title: "MIAO HTTP API", Version: "0.2.0", Description: "Generated from the Go API contract catalog. Product workflows and operational behavior remain in docs/OPERATIONS.md.", Operations: operations}
}

func normalizeRoute(route string) string { return strings.TrimSuffix(route, "/") }

func authKind(route string) string {
	if route == "/api/health" || strings.HasPrefix(route, "/api/openapi") || strings.HasPrefix(route, "/api/schema") || strings.HasPrefix(route, "/api/config-schema") || strings.HasPrefix(route, "/api/auth/register") || strings.HasPrefix(route, "/api/auth/login") || strings.HasPrefix(route, "/api/auth/verify-email") || strings.HasPrefix(route, "/api/auth/password-reset/") || strings.HasPrefix(route, "/api/public/") {
		return "public"
	}
	if strings.HasPrefix(route, "/api/admin/") {
		return "admin"
	}
	return "user"
}

func requestSchema(method, route string) string {
	if method == "GET" || route == "/api/auth/logout" || strings.HasSuffix(route, "/visit") || strings.HasSuffix(route, "/enable") || strings.HasSuffix(route, "/cancel") || strings.HasSuffix(route, "/continue") || strings.HasSuffix(route, "/resume") || strings.HasSuffix(route, "/check") {
		return ""
	}
	switch route {
	case "/api/auth/login":
		return "LoginRequest"
	case "/api/auth/register":
		return "RegisterRequest"
	case "/api/auth/verify-email":
		return "VerifyEmailRequest"
	case "/api/auth/password-reset/request":
		return "PasswordResetRequest"
	case "/api/auth/password-reset/confirm":
		return "PasswordResetConfirmRequest"
	case "/api/workspaces":
		return "CreateWorkspaceRequest"
	default:
		return "AnyObject"
	}
}

func responseSchema(method, route string) string {
	if route == "/api/health" {
		return "HealthResponse"
	}
	if method == "GET" && (strings.HasSuffix(route, "s") || strings.Contains(route, "/records") || strings.Contains(route, "/messages") || strings.Contains(route, "/events") || strings.Contains(route, "/usage")) {
		return "PageResponse"
	}
	if route == "/api/auth/login" || route == "/api/auth/register" {
		return "TokenResponse"
	}
	return "AnyObject"
}

func operationID(method, route string) string {
	var b strings.Builder
	b.WriteString(strings.ToLower(method))
	for _, r := range route {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return strings.Trim(b.String(), "_")
}

func operationDescription(method, route, auth string) string {
	sideEffect := "does not change persisted state"
	if method != "GET" {
		sideEffect = "may change persisted state"
	}
	return "Authentication: " + auth + "; this operation " + sideEffect + ". Endpoint-specific business behavior is documented in docs/OPERATIONS.md."
}

// ConfigTypes returns the public settings structs that are used to derive the
// configuration JSON Schema. Secrets are intentionally absent from these DTOs.
func ConfigTypes() map[string]reflect.Type {
	return map[string]reflect.Type{
		"Registration": reflect.TypeOf(settings.Registration{}),
		"Mail":         reflect.TypeOf(settings.Mail{}),
		"Admins":       reflect.TypeOf(settings.Admins{}),
		"Backup":       reflect.TypeOf(settings.Backup{}),
		"Branding":     reflect.TypeOf(settings.Branding{}),
		"Jev":          reflect.TypeOf(settings.Jev{}),
	}
}
