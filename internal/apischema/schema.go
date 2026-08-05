package apischema

import (
	"reflect"
	"strings"
	"time"

	"github.com/spare-run/spare/internal/model"
)

const ID = "https://spare.run/schema/api-v1.schema.json"

type Endpoint struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	Access   string `json:"access"`
	Response string `json:"response,omitempty"`
}

func Document() map[string]any {
	models := []any{
		model.Machine{}, model.Capabilities{}, model.ResourceGuidance{},
		model.Compatibility{}, model.ConfigField{}, model.PermissionGrant{},
		model.Recipe{}, model.JobPackage{}, model.JobPackageReview{},
		model.JobProfile{}, model.Problem{}, model.Instance{}, model.Event{},
		model.APIError{}, model.ErrorEnvelope{},
	}
	definitions := map[string]any{}
	for _, value := range models {
		typeOf := reflect.TypeOf(value)
		definitions[typeOf.Name()] = schemaFor(typeOf)
	}
	return map[string]any{
		"$schema":           "https://json-schema.org/draft/2020-12/schema",
		"$id":               ID,
		"title":             "Spare local API v1",
		"description":       "Stable public response models for Spare's authenticated loopback API.",
		"$defs":             definitions,
		"x-spare-endpoints": Endpoints(),
	}
}

func Endpoints() []Endpoint {
	return []Endpoint{
		{Method: "GET", Path: "/api/v1/health", Access: "authenticated", Response: "Health"},
		{Method: "GET", Path: "/api/v1/schema", Access: "authenticated", Response: "JSON Schema 2020-12"},
		{Method: "GET", Path: "/api/v1/machine", Access: "authenticated", Response: "Machine"},
		{Method: "GET", Path: "/api/v1/recipes", Access: "authenticated", Response: "Recipe[]"},
		{Method: "GET", Path: "/api/v1/instances", Access: "authenticated", Response: "Instance[]"},
		{Method: "POST", Path: "/api/v1/instances", Access: "local-bearer", Response: "Instance"},
		{Method: "POST", Path: "/api/v1/instances/switch", Access: "local-bearer", Response: "Instance"},
		{Method: "GET", Path: "/api/v1/instances/{id}", Access: "authenticated", Response: "Instance"},
		{Method: "DELETE", Path: "/api/v1/instances/{id}", Access: "local-bearer"},
		{Method: "POST", Path: "/api/v1/instances/{id}/start", Access: "authenticated", Response: "Instance"},
		{Method: "POST", Path: "/api/v1/instances/{id}/stop", Access: "authenticated", Response: "Instance"},
		{Method: "POST", Path: "/api/v1/instances/{id}/heartbeat", Access: "local-bearer"},
		{Method: "POST", Path: "/api/v1/instances/{id}/promote", Access: "local-bearer", Response: "Instance"},
		{Method: "POST", Path: "/api/v1/instances/{id}/configure", Access: "local-bearer", Response: "Instance"},
		{Method: "GET", Path: "/api/v1/events", Access: "authenticated", Response: "Event[]"},
		{Method: "GET", Path: "/api/v1/activity/stream", Access: "authenticated", Response: "Event stream"},
		{Method: "POST", Path: "/api/v1/browser-sessions", Access: "local-bearer", Response: "BrowserSession"},
		{Method: "GET", Path: "/api/v1/job-packages", Access: "authenticated", Response: "JobPackage[]"},
		{Method: "POST", Path: "/api/v1/job-packages/review", Access: "local-bearer", Response: "JobPackageReview"},
		{Method: "POST", Path: "/api/v1/job-packages/install", Access: "local-bearer", Response: "JobPackage"},
		{Method: "DELETE", Path: "/api/v1/job-packages/{id}", Access: "local-bearer"},
		{Method: "GET", Path: "/api/v1/job-profiles/{id}", Access: "local-bearer", Response: "JobProfile"},
		{Method: "POST", Path: "/api/v1/desktop/backups/export", Access: "local-bearer"},
		{Method: "POST", Path: "/api/v1/desktop/backups/restore", Access: "local-bearer", Response: "Instance"},
		{Method: "POST", Path: "/api/v1/desktop/drop-files", Access: "local-bearer"},
	}
}

func schemaFor(typeOf reflect.Type) map[string]any {
	if typeOf == reflect.TypeOf(time.Time{}) {
		return map[string]any{"type": "string", "format": "date-time"}
	}
	switch typeOf.Kind() {
	case reflect.Pointer:
		return schemaFor(typeOf.Elem())
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Slice, reflect.Array:
		if typeOf.Elem().Kind() == reflect.Uint8 {
			return map[string]any{"type": "string", "contentEncoding": "base64"}
		}
		return map[string]any{"type": "array", "items": schemaReference(typeOf.Elem())}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": schemaReference(typeOf.Elem())}
	case reflect.Interface:
		return map[string]any{}
	case reflect.Struct:
		properties := map[string]any{}
		required := []string{}
		for index := 0; index < typeOf.NumField(); index++ {
			field := typeOf.Field(index)
			name, options, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			properties[name] = schemaReference(field.Type)
			if !strings.Contains(options, "omitempty") {
				required = append(required, name)
			}
		}
		result := map[string]any{"type": "object", "additionalProperties": false, "properties": properties}
		if len(required) > 0 {
			result["required"] = required
		}
		return result
	default:
		return map[string]any{}
	}
}

func schemaReference(typeOf reflect.Type) map[string]any {
	for typeOf.Kind() == reflect.Pointer {
		typeOf = typeOf.Elem()
	}
	if typeOf == reflect.TypeOf(time.Time{}) {
		return schemaFor(typeOf)
	}
	if typeOf.Kind() == reflect.Struct && typeOf.PkgPath() == reflect.TypeOf(model.Machine{}).PkgPath() {
		return map[string]any{"$ref": "#/$defs/" + typeOf.Name()}
	}
	return schemaFor(typeOf)
}
