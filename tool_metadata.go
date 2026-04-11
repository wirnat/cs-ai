package cs_ai

type ToolAccessMode string

const (
	ToolAccessModeReadOnly   ToolAccessMode = "read_only"
	ToolAccessModeSideEffect ToolAccessMode = "side_effect"
)

type ToolUserVisibleTextPolicy string

const (
	ToolUserVisibleTextPolicyDefault   ToolUserVisibleTextPolicy = "default"
	ToolUserVisibleTextPolicyFactsOnly ToolUserVisibleTextPolicy = "facts_only"
	ToolUserVisibleTextPolicyHidden    ToolUserVisibleTextPolicy = "hidden"
)

type ToolSchemaOptions struct {
	Strict   bool
	Examples []interface{}
}

type ToolSchemaOptionsProvider interface {
	ToolSchemaOptions() ToolSchemaOptions
}

type ToolMetadata struct {
	AccessMode                   ToolAccessMode            `json:"access_mode,omitempty"`
	RequiresExplicitConfirmation bool                      `json:"requires_explicit_confirmation,omitempty"`
	IdempotencyScope             string                    `json:"idempotency_scope,omitempty"`
	UserVisibleTextPolicy        ToolUserVisibleTextPolicy `json:"user_visible_text_policy,omitempty"`
}

type ToolMetadataProvider interface {
	ToolMetadata() ToolMetadata
}

func resolveToolMetadata(intent Intent) ToolMetadata {
	if provider, ok := intent.(ToolMetadataProvider); ok {
		return normalizeToolMetadata(provider.ToolMetadata())
	}

	return ToolMetadata{
		AccessMode:            ToolAccessModeReadOnly,
		IdempotencyScope:      "best_effort",
		UserVisibleTextPolicy: ToolUserVisibleTextPolicyDefault,
	}
}

func normalizeToolMetadata(raw ToolMetadata) ToolMetadata {
	normalized := raw
	switch normalized.AccessMode {
	case ToolAccessModeReadOnly, ToolAccessModeSideEffect:
	default:
		normalized.AccessMode = ToolAccessModeReadOnly
	}

	switch normalized.UserVisibleTextPolicy {
	case ToolUserVisibleTextPolicyDefault, ToolUserVisibleTextPolicyFactsOnly, ToolUserVisibleTextPolicyHidden:
	default:
		normalized.UserVisibleTextPolicy = ToolUserVisibleTextPolicyDefault
	}

	if normalized.IdempotencyScope == "" {
		if normalized.AccessMode == ToolAccessModeReadOnly {
			normalized.IdempotencyScope = "best_effort"
		} else {
			normalized.IdempotencyScope = "none"
		}
	}

	return normalized
}
