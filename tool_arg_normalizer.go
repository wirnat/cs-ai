package cs_ai

import (
	"fmt"
	"reflect"
	"strings"
)

// normalizeToolArguments coerces tool-call arguments to match the expected
// primitive field types declared in intent.Param() struct tags.
func normalizeToolArguments(paramTemplate interface{}, args map[string]interface{}) (map[string]interface{}, error) {
	if paramTemplate == nil || len(args) == 0 {
		return args, nil
	}

	typ := reflect.TypeOf(paramTemplate)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return args, nil
	}

	normalized := make(map[string]interface{}, len(args))
	for key, value := range args {
		normalized[key] = value
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		jsonKey := fieldJSONKey(field)
		if jsonKey == "" {
			continue
		}

		rawValue, exists := normalized[jsonKey]
		if !exists {
			continue
		}
		if rawValue == nil {
			if toolFieldIsSemanticallyRequired(field) {
				return nil, fmt.Errorf("invalid value for field %s: required value cannot be null", jsonKey)
			}
			delete(normalized, jsonKey)
			continue
		}

		coercedValue, err := coerceToolArgValue(rawValue, field.Type, toolFieldIsSemanticallyRequired(field))
		if err != nil {
			return nil, fmt.Errorf("invalid value for field %s: %w", jsonKey, err)
		}
		normalized[jsonKey] = coercedValue
	}

	return normalized, nil
}

func fieldJSONKey(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return ""
	}
	if tag == "" {
		return field.Name
	}

	key := strings.Split(tag, ",")[0]
	if key == "" {
		return field.Name
	}
	return key
}

func toolFieldIsSemanticallyRequired(field reflect.StructField) bool {
	validateTag := strings.ToLower(strings.TrimSpace(field.Tag.Get("validate")))
	return strings.Contains(validateTag, "required")
}

func coerceToolArgValue(value interface{}, targetType reflect.Type, required bool) (interface{}, error) {
	if targetType.Kind() == reflect.Ptr {
		if value == nil {
			return nil, nil
		}
		return coerceToolArgValue(value, targetType.Elem(), required)
	}

	if targetType.Kind() != reflect.Bool {
		return value, nil
	}

	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "y":
			return true, nil
		case "false", "0", "no", "n":
			return false, nil
		default:
			return nil, fmt.Errorf("expected boolean string, got %q", v)
		}
	case float64:
		if v == 1 {
			return true, nil
		}
		if v == 0 {
			return false, nil
		}
		return nil, fmt.Errorf("expected 0 or 1 for boolean, got %v", v)
	default:
		return nil, fmt.Errorf("expected boolean, got %T", value)
	}
}
