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

func validateResponse(data interface{}, params map[string]interface{}) error {
	// Validasi tipe data response
	switch v := data.(type) {
	case map[string]interface{}:
		// Validasi untuk response berupa map
		for key, paramValue := range params {
			if responseValue, exists := v[key]; exists {
				// Validasi tipe data
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
						// Validasi tipe data
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
