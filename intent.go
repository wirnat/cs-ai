package cs_ai

import "context"

type Intent interface {
	Code() string
	Handle(ctx context.Context, req map[string]interface{}) (interface{}, error)
	Description() []string
	Param() interface{}
}
