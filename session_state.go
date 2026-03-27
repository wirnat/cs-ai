package cs_ai

import "encoding/json"

func cloneSessionStateMap(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return map[string]interface{}{}
	}

	payload, err := json.Marshal(input)
	if err != nil {
		clone := make(map[string]interface{}, len(input))
		for key, value := range input {
			clone[key] = value
		}
		return clone
	}

	var out map[string]interface{}
	if err := json.Unmarshal(payload, &out); err != nil || out == nil {
		clone := make(map[string]interface{}, len(input))
		for key, value := range input {
			clone[key] = value
		}
		return clone
	}

	return out
}
