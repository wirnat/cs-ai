package cs_ai

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
)

// convertParam mengubah struct menjadi JSON dengan format tertentu
func convertParam(param interface{}) (result map[string]interface{}, err error) {
	if param == nil {
		return
	}
	t := reflect.TypeOf(param)
	result = buildJSONSchemaForType(t)
	return result, nil
}

func buildJSONSchemaForType(t reflect.Type) map[string]interface{} {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		return map[string]interface{}{"type": "string"}
	case reflect.Bool:
		return map[string]interface{}{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]interface{}{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]interface{}{"type": "number"}
	case reflect.Slice, reflect.Array:
		return map[string]interface{}{
			"type":  "array",
			"items": buildJSONSchemaForType(t.Elem()),
		}
	case reflect.Map:
		schema := map[string]interface{}{
			"type": "object",
		}
		if t.Elem().Kind() == reflect.Interface {
			schema["additionalProperties"] = true
			return schema
		}
		schema["additionalProperties"] = buildJSONSchemaForType(t.Elem())
		return schema
	case reflect.Struct:
		if isJSONTimeType(t) {
			return map[string]interface{}{
				"type":   "string",
				"format": "date-time",
			}
		}

		properties := map[string]interface{}{}
		required := make([]interface{}, 0)
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.PkgPath != "" && !field.Anonymous {
				continue
			}

			jsonKey := getJSONFieldName(field)
			if jsonKey == "" {
				continue
			}

			fieldSchema := buildJSONSchemaForType(field.Type)
			if desc := strings.TrimSpace(field.Tag.Get("description")); desc != "" {
				fieldSchema["description"] = desc
			}
			if format := extractValidationFormat(field.Tag.Get("validate")); format != "" {
				fieldSchema["format"] = format
			}
			properties[jsonKey] = fieldSchema

			if fieldIsRequired(field) {
				required = append(required, jsonKey)
			}
		}

		return map[string]interface{}{
			"type":                 "object",
			"properties":           properties,
			"required":             required,
			"additionalProperties": false,
		}
	default:
		return map[string]interface{}{}
	}
}

func getJSONFieldName(field reflect.StructField) string {
	tag := strings.TrimSpace(field.Tag.Get("json"))
	if tag == "-" {
		return ""
	}
	if tag == "" {
		return field.Name
	}

	parts := strings.Split(tag, ",")
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return field.Name
	}
	return name
}

func fieldIsRequired(field reflect.StructField) bool {
	validateTag := strings.ToLower(strings.TrimSpace(field.Tag.Get("validate")))
	if strings.Contains(validateTag, "required") {
		return true
	}
	return false
}

func extractValidationFormat(validateTag string) string {
	normalized := strings.ToLower(strings.TrimSpace(validateTag))
	switch {
	case strings.Contains(normalized, "datetime"):
		return "date-time"
	case strings.Contains(normalized, "date"):
		return "date"
	case strings.Contains(normalized, "email"):
		return "email"
	case strings.Contains(normalized, "uuid"):
		return "uuid"
	}
	return ""
}

func isJSONTimeType(t reflect.Type) bool {
	return t.PkgPath() == "time" && t.Name() == "Time"
}

// generateToolDefinitionHash generates a hash from tool definition to detect changes
func generateToolDefinitionHash(intent Intent) (string, error) {
	// Get tool definition
	param, err := convertParam(intent.Param())
	if err != nil {
		return "", err
	}

	// Create tool definition structure
	toolDef := map[string]interface{}{
		"name":        intent.Code(),
		"description": strings.Join(intent.Description(), ", "),
		"parameters":  param,
	}

	// Convert to JSON for consistent hashing
	jsonBytes, err := json.Marshal(toolDef)
	if err != nil {
		return "", err
	}

	// Generate SHA256 hash
	hash := sha256.Sum256(jsonBytes)
	return hex.EncodeToString(hash[:]), nil
}
