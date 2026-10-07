package api

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// OpenAPI returns an OpenAPI 3.1 document built from Current and the named Go
// DTOs in this package. It returns ordinary maps so callers can encode or
// extend the document without taking a dependency on an OpenAPI library.
func OpenAPI() map[string]any {
	contract := Current()
	paths := map[string]any{}
	for _, operation := range contract.Operations {
		pathItem, _ := paths[operation.Path].(map[string]any)
		if pathItem == nil {
			pathItem = map[string]any{}
			paths[operation.Path] = pathItem
		}
		item := map[string]any{
			"operationId": operation.OperationID,
			"summary":     operation.Summary,
			"description": operation.Description,
			"tags":        []string{tagForPath(operation.Path)},
			"responses":   responses(operation),
		}
		if operation.Auth != "public" {
			item["security"] = []any{map[string]any{"bearerAuth": []any{}}}
		}
		item["parameters"] = pathParameters(operation.Path)
		if operation.RequestSchema != "" {
			item["requestBody"] = map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{"schema": schemaRef(operation.RequestSchema)},
				},
			}
		}
		pathItem[strings.ToLower(operation.Method)] = item
	}
	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":       contract.Title,
			"version":     contract.Version,
			"description": contract.Description,
		},
		"servers": []any{map[string]any{"url": "/"}},
		"paths":   paths,
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{"type": "http", "scheme": "bearer"},
			},
			"schemas": Schemas(),
		},
	}
}

func responses(operation Operation) map[string]any {
	status := "200"
	if operation.Method == "POST" && strings.HasSuffix(operation.Path, "/runs") {
		status = "202"
	}
	return map[string]any{
		status: map[string]any{
			"description": "Successful response",
			"content":     map[string]any{"application/json": map[string]any{"schema": schemaRef(operation.ResponseSchema)}},
		},
		"400": errorResponse("Invalid request"),
		"401": errorResponse("Authentication required"),
		"403": errorResponse("Permission denied"),
		"404": errorResponse("Resource not found"),
		"409": errorResponse("Conflict"),
		"500": errorResponse("Server error"),
	}
}

func errorResponse(description string) map[string]any {
	return map[string]any{"description": description, "content": map[string]any{"application/json": map[string]any{"schema": schemaRef("ErrorResponse")}}}
}

func pathParameters(route string) []any {
	params := []any{}
	for _, part := range strings.Split(route, "/") {
		if !strings.HasPrefix(part, "{") || !strings.HasSuffix(part, "}") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
		params = append(params, map[string]any{"name": name, "in": "path", "required": true, "schema": map[string]any{"type": "string"}})
	}
	return params
}

func schemaRef(name string) map[string]any {
	if name == "" {
		return map[string]any{}
	}
	return map[string]any{"$ref": "#/components/schemas/" + name}
}

func tagForPath(route string) string {
	parts := strings.Split(strings.TrimPrefix(route, "/api/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "core"
	}
	return parts[0]
}

// Schemas returns the OpenAPI component schemas for API DTOs and configuration
// DTOs. Reflection keeps field names and primitive types aligned with Go tags.
func Schemas() map[string]any {
	types := map[string]reflect.Type{
		"ErrorResponse":               reflect.TypeOf(ErrorResponse{}),
		"HealthResponse":              reflect.TypeOf(HealthResponse{}),
		"TokenResponse":               reflect.TypeOf(TokenResponse{}),
		"PageResponse":                reflect.TypeOf(PageResponse{}),
		"CreateWorkspaceRequest":      reflect.TypeOf(CreateWorkspaceRequest{}),
		"LoginRequest":                reflect.TypeOf(LoginRequest{}),
		"RegisterRequest":             reflect.TypeOf(RegisterRequest{}),
		"VerifyEmailRequest":          reflect.TypeOf(VerifyEmailRequest{}),
		"PasswordResetRequest":        reflect.TypeOf(PasswordResetRequest{}),
		"PasswordResetConfirmRequest": reflect.TypeOf(PasswordResetConfirmRequest{}),
		"AnyObject":                   reflect.TypeOf(AnyObject{}),
	}
	for name, typ := range ConfigTypes() {
		types["Config"+name] = typ
	}
	out := map[string]any{}
	for name, typ := range types {
		out[name] = schemaForType(typ)
	}
	return out
}

// ConfigSchema returns a standalone JSON Schema document for settings DTOs.
func ConfigSchema() map[string]any {
	defs := map[string]any{}
	for name, typ := range ConfigTypes() {
		defs[name] = schemaForType(typ)
	}
	return map[string]any{
		"$schema":     "https://json-schema.org/draft/2020-12/schema",
		"$id":         "https://miao.local/schema/config.json",
		"title":       "MIAO configuration DTOs",
		"description": "Generated from internal/settings exported configuration structs. Secrets are never included.",
		"$defs":       defs,
		"oneOf":       []any{map[string]any{"$ref": "#/$defs/Registration"}, map[string]any{"$ref": "#/$defs/Mail"}, map[string]any{"$ref": "#/$defs/Admins"}, map[string]any{"$ref": "#/$defs/Backup"}, map[string]any{"$ref": "#/$defs/LLM"}, map[string]any{"$ref": "#/$defs/Jev"}},
	}
}

func schemaForType(typ reflect.Type) map[string]any {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Struct:
		properties := map[string]any{}
		required := []string{}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.PkgPath != "" {
				continue
			}
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "" {
				name = field.Name
			}
			if name == "-" {
				continue
			}
			properties[name] = schemaForType(field.Type)
			if !strings.Contains(field.Tag.Get("json"), "omitempty") {
				required = append(required, name)
			}
		}
		out := map[string]any{"type": "object", "properties": properties}
		if len(required) > 0 {
			sort.Strings(required)
			out["required"] = required
		}
		return out
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": schemaForType(typ.Elem())}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": schemaForType(typ.Elem())}
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return map[string]any{"type": "integer"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer", "minimum": 0}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	default:
		return map[string]any{}
	}
}

// JSON returns deterministic indented JSON for a generated document.
func JSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// Markdown returns a generated API reference table. It deliberately points
// product behavior to the manually maintained operations manual.
func Markdown() string {
	contract := Current()
	var b strings.Builder
	b.WriteString("<!-- Code generated by miao; DO NOT EDIT. -->\n\n# MIAO HTTP API\n\n")
	b.WriteString("This reference is generated from Go definitions. See [OPERATIONS.md](../OPERATIONS.md) for product workflows, permissions, deployment, and recovery behavior.\n\n")
	b.WriteString(fmt.Sprintf("- OpenAPI: [`openapi.json`](openapi.json)\n- JSON Schema: [`api-schema.json`](api-schema.json)\n- Configuration Schema: [`config-schema.json`](config-schema.json)\n- Operations: %d\n\n", len(contract.Operations)))
	b.WriteString("| Method | Path | Operation ID | Auth | Side effects |\n| --- | --- | --- | --- | --- |\n")
	for _, operation := range contract.Operations {
		b.WriteString(fmt.Sprintf("| `%s` | `%s` | `%s` | `%s` | `%t` |\n", operation.Method, operation.Path, operation.OperationID, operation.Auth, operation.SideEffects))
	}
	return b.String()
}
