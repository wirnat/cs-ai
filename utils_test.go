package cs_ai

import "testing"

func TestValidateResponse_SkipsNilOptionalParams(t *testing.T) {
	params := map[string]interface{}{
		"service_name": nil,
		"provider_uid": nil,
	}
	response := map[string]interface{}{
		"service_name": "Haircut",
		"provider_uid": "provider-1",
	}

	if err := validateResponse(response, params); err != nil {
		t.Fatalf("expected nil optional params to be ignored, got %v", err)
	}
}

func TestValidateResponse_SkipsComplexParamsThatCanBeEnriched(t *testing.T) {
	params := map[string]interface{}{
		"services": []interface{}{
			map[string]interface{}{
				"service_uid": "svc-1",
				"time":        "19:00",
			},
		},
	}
	response := map[string]interface{}{
		"services": []interface{}{
			map[string]interface{}{
				"service_uid":  "svc-1",
				"service_name": "Haircut",
				"time":         "19:00",
			},
		},
	}

	if err := validateResponse(response, params); err != nil {
		t.Fatalf("expected complex params to be ignored, got %v", err)
	}
}

func TestValidateResponse_StillRejectsContradictoryScalarEcho(t *testing.T) {
	params := map[string]interface{}{
		"date": "2026-03-28",
	}
	response := map[string]interface{}{
		"date": "2026-03-29",
	}

	if err := validateResponse(response, params); err == nil {
		t.Fatalf("expected contradictory scalar echo to fail validation")
	}
}

func TestValidateResponse_AllowsNumericTypeCoercion(t *testing.T) {
	params := map[string]interface{}{
		"qty": 1,
	}
	response := map[string]interface{}{
		"qty": 1.0,
	}

	if err := validateResponse(response, params); err != nil {
		t.Fatalf("expected numeric coercion to pass, got %v", err)
	}
}

func TestValidateResponse_SkipsNumericEchoValidationToAvoidFalseFailure(t *testing.T) {
	params := map[string]interface{}{
		"quantity": 0,
	}
	response := map[string]interface{}{
		"quantity": 1,
	}

	if err := validateResponse(response, params); err != nil {
		t.Fatalf("expected numeric echo validation to be skipped, got %v", err)
	}
}

func TestValidateResponse_SkipsBooleanEchoValidationToAvoidFalseFailure(t *testing.T) {
	params := map[string]interface{}{
		"auto_assign": false,
	}
	response := map[string]interface{}{
		"auto_assign": true,
	}

	if err := validateResponse(response, params); err != nil {
		t.Fatalf("expected boolean echo validation to be skipped, got %v", err)
	}
}
