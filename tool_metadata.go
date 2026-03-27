package cs_ai

type ToolSchemaOptions struct {
	Strict   bool
	Examples []interface{}
}

type ToolSchemaOptionsProvider interface {
	ToolSchemaOptions() ToolSchemaOptions
}
