package cs_ai

import "testing"

type normalizerStrictOptionalParam struct {
	Date         string  `json:"date" validate:"required"`
	Time         string  `json:"time" validate:"required"`
	Confirm      bool    `json:"confirm"`
	AutoAssign   bool    `json:"auto_assign"`
	PriceOption  float64 `json:"price_option"`
	ServiceName  string  `json:"service_name"`
	CustomerName string  `json:"customer_name"`
}

func TestNormalizeToolArguments_OptionalNullScalarIsOmitted(t *testing.T) {
	args := map[string]interface{}{
		"date":          "2026-03-28",
		"time":          "15:00",
		"confirm":       false,
		"auto_assign":   nil,
		"price_option":  nil,
		"service_name":  nil,
		"customer_name": nil,
	}

	normalized, err := normalizeToolArguments(normalizerStrictOptionalParam{}, args)
	if err != nil {
		t.Fatalf("expected optional null scalar values to be accepted, got %v", err)
	}

	if _, exists := normalized["auto_assign"]; exists {
		t.Fatalf("expected auto_assign to be omitted when null")
	}
	if _, exists := normalized["price_option"]; exists {
		t.Fatalf("expected price_option to be omitted when null")
	}
	if _, exists := normalized["service_name"]; exists {
		t.Fatalf("expected service_name to be omitted when null")
	}
	if _, exists := normalized["customer_name"]; exists {
		t.Fatalf("expected customer_name to be omitted when null")
	}
}

func TestNormalizeToolArguments_RequiredNullStillFails(t *testing.T) {
	args := map[string]interface{}{
		"date": nil,
		"time": "15:00",
	}

	if _, err := normalizeToolArguments(normalizerStrictOptionalParam{}, args); err == nil {
		t.Fatalf("expected required null value to fail normalization")
	}
}
