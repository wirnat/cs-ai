package cs_ai

import (
	"encoding/json"
	"fmt"
	"reflect"
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
			if responseValue, exists := v[key]; exists {
				// Handle numeric type coercion (JSON unmarshal converts all numbers to float64)
				if equal, bothNumeric := isNumericEqual(responseValue, paramValue); bothNumeric {
					if !equal {
						return fmt.Errorf("nilai response untuk parameter %s tidak sesuai", key)
					}
					continue
				}
				// Non-numeric: validasi tipe data
				if reflect.TypeOf(responseValue) != reflect.TypeOf(paramValue) {
					return fmt.Errorf("tipe data response untuk parameter %s tidak sesuai", key)
				}
				// Validasi nilai
				if !reflect.DeepEqual(responseValue, paramValue) {
					return fmt.Errorf("nilai response untuk parameter %s tidak sesuai", key)
				}
			}
		}
	case []interface{}:
		// Validasi untuk response berupa array
		if len(v) > 0 {
			if firstItem, ok := v[0].(map[string]interface{}); ok {
				for key, paramValue := range params {
					if responseValue, exists := firstItem[key]; exists {
						// Handle numeric type coercion
						if equal, bothNumeric := isNumericEqual(responseValue, paramValue); bothNumeric {
							if !equal {
								return fmt.Errorf("nilai response untuk parameter %s tidak sesuai", key)
							}
							continue
						}
						// Non-numeric: validasi tipe data
						if reflect.TypeOf(responseValue) != reflect.TypeOf(paramValue) {
							return fmt.Errorf("tipe data response untuk parameter %s tidak sesuai", key)
						}
						// Validasi nilai
						if !reflect.DeepEqual(responseValue, paramValue) {
							return fmt.Errorf("nilai response untuk parameter %s tidak sesuai", key)
						}
					}
				}
			}
		}
	}
	return nil
}
