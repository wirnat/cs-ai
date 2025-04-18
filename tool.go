package cs_ai

import (
	"reflect"
	"strings"
)

func convertProperties(input map[string]map[string]string) map[string]map[string]interface{} {
	output := make(map[string]map[string]interface{})

	for key, innerMap := range input {
		convertedInnerMap := make(map[string]interface{})
		for innerKey, value := range innerMap {
			convertedInnerMap[innerKey] = value
		}
		output[key] = convertedInnerMap
	}

	return output
}

// convertParam mengubah struct menjadi JSON dengan format tertentu
func convertParam(param interface{}) (result map[string]interface{}, err error) {
	if param == nil {
		return
	}
	t := reflect.TypeOf(param)
	if t.Kind() == reflect.Ptr {
		t = t.Elem() // Jika pointer, ambil nilai asli
	}

	result = map[string]interface{}{
		"type":       "object",
		"properties": map[string]map[string]string{},
		"required":   []string{},
	}

	properties := convertProperties(result["properties"].(map[string]map[string]string))

	requiredFields := result["required"].([]string)

	// Loop setiap field dalam struct
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldType := field.Type

		// Jika field adalah pointer, ambil tipe aslinya
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}

		// Dapatkan nama JSON
		jsonKey := field.Tag.Get("json")
		if jsonKey == "" {
			jsonKey = field.Name
		}

		// Deteksi tipe data
		var jsonType string
		switch fieldType.Kind() {
		case reflect.String:
			jsonType = "string"
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			jsonType = "integer"
		case reflect.Float32, reflect.Float64:
			jsonType = "number"
		case reflect.Bool:
			jsonType = "boolean"
		case reflect.Slice, reflect.Array:
			jsonType = "array"
		case reflect.Map, reflect.Struct:
			jsonType = "object"
		default:
			jsonType = "unknown"
		}

		// Tambahkan ke properties
		properties[jsonKey] = map[string]interface{}{
			"type":    jsonType,
			"default": nil,
		}

		// Cek apakah ada tag description
		if desc := field.Tag.Get("description"); desc != "" {
			properties[jsonKey]["description"] = desc
		}

		// Cek apakah field memiliki tag `validate:"required"`
		if validateTag := field.Tag.Get("validate"); strings.Contains(validateTag, "required") {
			requiredFields = append(requiredFields, jsonKey)
		}
	}

	result["required"] = requiredFields
	result["properties"] = properties

	return
}
