package cs_ai

type Intent interface {
	Code() string
	Handle(map[string]interface{}) (interface{}, error)
	Description() []string
	Param() interface{}
}
