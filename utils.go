package cs_ai

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

func isValidResponse(msg Message) bool {
	if msg.Content == "" {
		return false
	}

	// Cek apakah content adalah JSON yang valid
	var js json.RawMessage
	if err := json.Unmarshal([]byte(msg.Content), &js); err != nil {
		// Jika bukan JSON, tetap valid selama ada content
		return true
	}
	return true
}

// isNumericEqual compares two values as float64 if both are numeric types.
// This handles the case where JSON unmarshal converts all numbers to float64.
func isNumericEqual(a, b interface{}) (equal bool, bothNumeric bool) {
	aFloat, aOk := toFloat64(a)
	bFloat, bOk := toFloat64(b)
	if aOk && bOk {
		return aFloat == bFloat, true
	}
	return false, false
}

// toFloat64 converts numeric types to float64
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case int:
		return float64(val), true
	case int8:
		return float64(val), true
	case int16:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint8:
		return float64(val), true
	case uint16:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint64:
		return float64(val), true
	case float32:
		return float64(val), true
	case float64:
		return val, true
	default:
		return 0, false
	}
}

func validateResponse(data interface{}, params map[string]interface{}) error {
	// Validasi tipe data response
	switch v := data.(type) {
	case map[string]interface{}:
		// Validasi untuk response berupa map
		for key, paramValue := range params {
			if !shouldValidateEchoParam(paramValue) {
				continue
			}
			if responseValue, exists := v[key]; exists {
				if err := validateEchoValue(key, responseValue, paramValue); err != nil {
					return err
				}
			}
		}
	case []interface{}:
		// Validasi untuk response berupa array
		if len(v) > 0 {
			if firstItem, ok := v[0].(map[string]interface{}); ok {
				for key, paramValue := range params {
					if !shouldValidateEchoParam(paramValue) {
						continue
					}
					if responseValue, exists := firstItem[key]; exists {
						if err := validateEchoValue(key, responseValue, paramValue); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

// shouldValidateEchoParam keeps response validation focused on explicit scalar
// inputs. Tool responses often enrich optional or complex fields (for example
// service_name/services), so validating those as exact echoes creates false
// negatives during execution.
func shouldValidateEchoParam(paramValue interface{}) bool {
	if paramValue == nil {
		return false
	}

	switch v := paramValue.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case bool:
		// Boolean params are often optional control flags; enforcing exact echo
		// here creates false negatives when tools normalize or enrich payloads.
		return false
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		// Numeric params are frequently defaulted/normalized by tools
		// (e.g. quantity fallback), so skip strict echo validation.
		return false
	case []interface{}:
		return false
	case map[string]interface{}:
		return false
	}

	rv := reflect.ValueOf(paramValue)
	switch rv.Kind() {
	case reflect.Invalid:
		return false
	case reflect.Slice, reflect.Array, reflect.Map, reflect.Struct:
		return false
	default:
		return true
	}
}

func validateEchoValue(key string, responseValue interface{}, paramValue interface{}) error {
	// Handle numeric type coercion (JSON unmarshal converts all numbers to float64)
	if equal, bothNumeric := isNumericEqual(responseValue, paramValue); bothNumeric {
		if !equal {
			return fmt.Errorf("nilai response untuk parameter %s tidak sesuai", key)
		}
		return nil
	}

	// Non-numeric: validasi tipe data
	if reflect.TypeOf(responseValue) != reflect.TypeOf(paramValue) {
		return fmt.Errorf("tipe data response untuk parameter %s tidak sesuai", key)
	}

	// Validasi nilai
	if !reflect.DeepEqual(responseValue, paramValue) {
		return fmt.Errorf("nilai response untuk parameter %s tidak sesuai", key)
	}

	return nil
}
