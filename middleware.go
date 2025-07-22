package cs_ai

import (
	"context"
	"time"
)

// ============================== MIDDLEWARE SYSTEM ==============================

// MiddlewareContext contains context information for middleware execution
type MiddlewareContext struct {
	SessionID       string                 `json:"session_id"`
	IntentCode      string                 `json:"intent_code"`
	UserMessage     UserMessage            `json:"user_message"`
	Parameters      map[string]interface{} `json:"parameters"`
	StartTime       time.Time              `json:"start_time"`
	Metadata        map[string]interface{} `json:"metadata"`
	PreviousResults []interface{}          `json:"previous_results"`
}

// MiddlewareNext defines the function signature for next middleware in chain
type MiddlewareNext func(ctx context.Context, mctx *MiddlewareContext) (interface{}, error)

// Middleware interface for intent middleware
type Middleware interface {
	Handle(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error)
	Name() string        // Name of the middleware for logging/debugging
	AppliesTo() []string // Intent codes this middleware applies to (empty = global)
	Priority() int       // Priority for ordering (lower = earlier execution)
}

// MiddlewareFunc is a function-based middleware implementation
type MiddlewareFunc struct {
	name      string
	appliesTo []string
	priority  int
	handler   func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error)
}

func (m *MiddlewareFunc) Handle(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
	return m.handler(ctx, mctx, next)
}

func (m *MiddlewareFunc) Name() string {
	return m.name
}

func (m *MiddlewareFunc) AppliesTo() []string {
	return m.appliesTo
}

func (m *MiddlewareFunc) Priority() int {
	return m.priority
}

// NewMiddlewareFunc creates a new function-based middleware
func NewMiddlewareFunc(name string, appliesTo []string, priority int, handler func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error)) *MiddlewareFunc {
	return &MiddlewareFunc{
		name:      name,
		appliesTo: appliesTo,
		priority:  priority,
		handler:   handler,
	}
}

// MiddlewareChain manages and executes middleware chain
type MiddlewareChain struct {
	middlewares []Middleware
}

// NewMiddlewareChain creates a new middleware chain
func NewMiddlewareChain() *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: make([]Middleware, 0),
	}
}

// Add adds a middleware to the chain
func (mc *MiddlewareChain) Add(middleware Middleware) {
	mc.middlewares = append(mc.middlewares, middleware)
	// Sort by priority (lower priority executes first)
	for i := len(mc.middlewares) - 1; i > 0; i-- {
		if mc.middlewares[i].Priority() < mc.middlewares[i-1].Priority() {
			mc.middlewares[i], mc.middlewares[i-1] = mc.middlewares[i-1], mc.middlewares[i]
		} else {
			break
		}
	}
}

// Execute executes the middleware chain for a specific intent
func (mc *MiddlewareChain) Execute(ctx context.Context, mctx *MiddlewareContext, finalHandler MiddlewareNext) (interface{}, error) {
	// Filter middlewares that apply to this intent
	applicableMiddlewares := make([]Middleware, 0)
	for _, middleware := range mc.middlewares {
		appliesTo := middleware.AppliesTo()
		if len(appliesTo) == 0 || containsString(appliesTo, mctx.IntentCode) {
			applicableMiddlewares = append(applicableMiddlewares, middleware)
		}
	}

	// If no middlewares, execute final handler directly
	if len(applicableMiddlewares) == 0 {
		return finalHandler(ctx, mctx)
	}

	// Build the chain recursively
	return mc.buildChain(ctx, mctx, applicableMiddlewares, 0, finalHandler)
}

// buildChain builds the middleware chain recursively
func (mc *MiddlewareChain) buildChain(ctx context.Context, mctx *MiddlewareContext, middlewares []Middleware, index int, finalHandler MiddlewareNext) (interface{}, error) {
	if index >= len(middlewares) {
		return finalHandler(ctx, mctx)
	}

	currentMiddleware := middlewares[index]
	next := func(ctx context.Context, mctx *MiddlewareContext) (interface{}, error) {
		return mc.buildChain(ctx, mctx, middlewares, index+1, finalHandler)
	}

	return currentMiddleware.Handle(ctx, mctx, next)
}

// containsString checks if a string slice contains a specific string
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ============================== END MIDDLEWARE SYSTEM ==============================
