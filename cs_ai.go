package cs_ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
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

type Options struct {
	Redis          *redis.Client //Cache Messages
	UseTool        bool          //Gunakan tool handler api
	LogChatFile    bool          //Simpan chat ke file
	EnableLearning bool          //Aktifkan learning manager
	ResponseType   ResponseType  //Tipe response
}

func New(ApiKey string, modeler Modeler, o ...Options) *CsAI {
	cs := &CsAI{
		ApiKey:          ApiKey,
		Model:           modeler,
		middlewareChain: NewMiddlewareChain(),
	}
	if len(o) > 0 {
		cs.options = o[0]
		if cs.options.EnableLearning && cs.options.Redis != nil {
			cs.learningManager = NewLearningManager(cs.options.Redis)
		}
	}
	return cs
}

type CsAI struct {
	ApiKey          string
	Model           Modeler
	intents         []Intent
	options         Options
	learningManager *LearningManager
	middlewareChain *MiddlewareChain
}

type UserMessage struct {
	Message         string `json:"message"`
	ParticipantName string `json:"participant_name"`
}
type AIResponse struct {
	Response interface{} `json:"response"`
}

// ResponseProcessor interface untuk memproses response dari tool
type ResponseProcessor interface {
	Process(data interface{}) (string, error)
}

// DefaultResponseProcessor implementasi default dari ResponseProcessor
type DefaultResponseProcessor struct{}

func (p *DefaultResponseProcessor) Process(data interface{}) (string, error) {
	switch v := data.(type) {
	case string:
		return v, nil
	case map[string]interface{}, []interface{}:
		// Format JSON dengan indentasi yang lebih baik
		processedData, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return "", err
		}
		return string(processedData), nil
	default:
		// Untuk tipe data lain, gunakan format default
		processedData, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(processedData), nil
	}
}

// isValidResponse memvalidasi response sebelum dikembalikan ke user
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

// LearningData menyimpan data pembelajaran
type LearningData struct {
	Query     string                 `json:"query"`
	Response  string                 `json:"response"`
	Tools     []string               `json:"tools"`
	Context   map[string]interface{} `json:"context"`
	Timestamp time.Time              `json:"timestamp"`
	Feedback  int                    `json:"feedback"` // -1: negative, 0: neutral, 1: positive
}

// LearningManager mengelola proses pembelajaran AI
type LearningManager struct {
	redis *redis.Client
}

func NewLearningManager(redis *redis.Client) *LearningManager {
	return &LearningManager{
		redis: redis,
	}
}

func (lm *LearningManager) SaveLearningData(ctx context.Context, data LearningData) error {
	if lm.redis == nil {
		return nil
	}

	key := fmt.Sprintf("ai:learning:%s", time.Now().Format("2006-01-02"))
	dataJson, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return lm.redis.RPush(ctx, key, dataJson).Err()
}

func (lm *LearningManager) GetLearningData(ctx context.Context, days int) ([]LearningData, error) {
	if lm.redis == nil {
		return nil, nil
	}

	var allData []LearningData
	for i := 0; i < days; i++ {
		date := time.Now().AddDate(0, 0, -i)
		key := fmt.Sprintf("ai:learning:%s", date.Format("2006-01-02"))

		data, err := lm.redis.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			continue
		}

		for _, item := range data {
			var learningData LearningData
			if err := json.Unmarshal([]byte(item), &learningData); err != nil {
				continue
			}
			allData = append(allData, learningData)
		}
	}

	return allData, nil
}

// Tambahkan struct untuk cache key
type ToolCacheKey struct {
	FunctionName string
	Arguments    string
}

// Exec mengeksekusi pesan ke AI
func (c *CsAI) Exec(ctx context.Context, sessionID string, userMessage UserMessage, additionalSystemMessage ...string) (Message, error) {
	// ambil pesan lama (jika ada)
	oldMessages, _ := c.getSessionMessages(sessionID) // error bisa diabaikan
	messages := make(Messages, 0)
	if oldMessages != nil {
		messages = append(messages, oldMessages...)
	}

	//cek kembali messages,jika pada list messages ada pesan dengan role string kosong maka hapus dari list
	// Filter ulang: buang pesan dengan role kosong
	filteredMessages := make(Messages, 0, len(messages))
	for _, msg := range messages {
		if msg.Role != "" {
			filteredMessages = append(filteredMessages, msg)
		}
	}

	// replace messages dengan hasil yang sudah difilter
	messages = filteredMessages

	// tambahkan pesan baru ke message list
	conversationMessages := Message{
		Content: userMessage.Message,
		Name:    userMessage.ParticipantName,
		Role:    User,
	}
	messages.Add(conversationMessages)

	// Kirim request pertama ke AI
	aiResponse, err := c.Send(messages, additionalSystemMessage...)
	if err != nil {
		return Message{}, err
	}
	messages.Add(aiResponse)

	// Cek apakah AI langsung memberikan jawaban tanpa tool_calls
	if aiResponse.Role == Assistant && len(aiResponse.ToolCalls) == 0 {
		// Validasi response type jika diatur
		if c.options.ResponseType != "" && !validateResponseType(aiResponse.Content, c.options.ResponseType) {
			// Jika response type tidak sesuai, kirim pesan system untuk mengubah format
			formatMessage := fmt.Sprintf("Mohon ubah format response menjadi %s", c.options.ResponseType)
			messages.Add(Message{
				Content: formatMessage,
				Role:    User,
			})

			// Kirim ulang request dengan instruksi format
			aiResponse, err = c.Send(messages, additionalSystemMessage...)
			if err != nil {
				return Message{}, err
			}
			messages.Add(aiResponse)
		}

		_, _ = c.saveSessionMessages(sessionID, messages) // simpan percakapan
		return aiResponse, nil
	}

	// ============================== HANDLE TOOL CALLS ==============================
	toolCache := make(map[ToolCacheKey]Message)
	maxLoop := 10
	loopCount := 0
	processor := &DefaultResponseProcessor{}

	for len(aiResponse.ToolCalls) > 0 {
		loopCount++
		if loopCount > maxLoop {
			fmt.Println("Max loop reached, returning last known response.")
			// Cek semua tool calls yang ada di cache
			var validResponses []Message
			for _, tool := range aiResponse.ToolCalls {
				cacheKey := ToolCacheKey{
					FunctionName: tool.Function.Name,
					Arguments:    tool.Function.Arguments,
				}
				if response, exists := toolCache[cacheKey]; exists && isValidResponse(response) {
					validResponses = append(validResponses, response)
				}
			}

			if len(validResponses) > 0 {
				// Gabungkan semua valid responses dengan format yang lebih jelas
				combinedContent := make([]string, 0)
				for _, resp := range validResponses {
					combinedContent = append(combinedContent, resp.Content)
				}

				finalResponse := Message{
					Content: strings.Join(combinedContent, "\n\n"),
					Name:    userMessage.ParticipantName,
					Role:    "assistant",
				}
				_, _ = c.saveSessionMessages(sessionID, messages)
				return finalResponse, nil
			}

			return Message{
				Content: "Maaf, sistem sedang sibuk. Silakan coba lagi dalam beberapa saat.",
				Name:    userMessage.ParticipantName,
				Role:    "assistant",
			}, nil
		}

		newMessages := make(Messages, 0)
		processedTools := make(map[ToolCacheKey]bool)
		toolCallResponses := make(map[string]Message)

		// Proses semua tool calls dalam satu iterasi
		for _, tool := range aiResponse.ToolCalls {
			cacheKey := ToolCacheKey{
				FunctionName: tool.Function.Name,
				Arguments:    tool.Function.Arguments,
			}

			if processedTools[cacheKey] {
				continue
			}
			processedTools[cacheKey] = true

			if cachedResponse, exists := toolCache[cacheKey]; exists {
				if isValidResponse(cachedResponse) {
					// Pastikan tool_call_id sesuai
					cachedResponse.ToolCallID = tool.Id
					toolCallResponses[tool.Id] = cachedResponse
					newMessages.Add(cachedResponse)
					continue
				}
				delete(toolCache, cacheKey)
			}

			for _, intent := range c.intents {
				if intent.Code() == tool.Function.Name {
					p := intent.Param()
					err := json.Unmarshal([]byte(tool.Function.Arguments), &p)
					if err != nil {
						return Message{}, fmt.Errorf("failed to parse function arguments: %v", err)
					}

					paramMap, ok := p.(map[string]interface{})
					if !ok {
						return Message{}, fmt.Errorf("invalid function argument format")
					}

					// Create middleware context
					middlewareCtx := &MiddlewareContext{
						SessionID:       sessionID,
						IntentCode:      intent.Code(),
						UserMessage:     userMessage,
						Parameters:      paramMap,
						StartTime:       time.Now(),
						Metadata:        make(map[string]interface{}),
						PreviousResults: make([]interface{}, 0),
					}

					// Execute middleware chain with intent handler as final handler
					finalHandler := func(ctx context.Context, mctx *MiddlewareContext) (interface{}, error) {
						return intent.Handle(ctx, mctx.Parameters)
					}

					data, err := c.middlewareChain.Execute(ctx, middlewareCtx, finalHandler)
					if err != nil {
						return Message{}, err
					}

					// Validasi response sesuai dengan parameter
					if err := validateResponse(data, paramMap); err != nil {
						return Message{}, fmt.Errorf("invalid response for parameters: %v", err)
					}

					processedContent, err := processor.Process(data)
					if err != nil {
						return Message{}, fmt.Errorf("failed to process tool response: %v", err)
					}

					toolResponse := Message{
						Content:    processedContent,
						Role:       Tool,
						ToolCallID: tool.Id,
					}
					toolCache[cacheKey] = toolResponse
					toolCallResponses[tool.Id] = toolResponse
					newMessages.Add(toolResponse)
				}
			}
		}

		// Verifikasi bahwa setiap tool call memiliki response
		for _, tool := range aiResponse.ToolCalls {
			if _, exists := toolCallResponses[tool.Id]; !exists {
				// Jika ada tool call yang tidak memiliki response, tambahkan response kosong
				emptyResponse := Message{
					Content:    "{}",
					Role:       Tool,
					ToolCallID: tool.Id,
				}
				newMessages.Add(emptyResponse)
			}
		}

		if len(newMessages) == 0 {
			break
		}

		messages.Add(newMessages...)
		aiResponse, err = c.Send(messages, additionalSystemMessage...)
		if err != nil {
			return Message{}, err
		}
		messages.Add(aiResponse)

		if aiResponse.Role == Assistant && len(aiResponse.ToolCalls) == 0 {
			break
		}
	}

	// simpan semua percakapan ke redis setelah selesai
	_, _ = c.saveSessionMessages(sessionID, messages)

	// Simpan data pembelajaran
	if c.learningManager != nil {
		learningData := LearningData{
			Query:     userMessage.Message,
			Response:  aiResponse.Content,
			Tools:     make([]string, 0),
			Context:   make(map[string]interface{}),
			Timestamp: time.Now(),
		}

		// Kumpulkan tools yang digunakan
		for _, tool := range aiResponse.ToolCalls {
			learningData.Tools = append(learningData.Tools, tool.Function.Name)
		}

		// Simpan context dari messages
		for _, msg := range messages {
			if msg.Role == System {
				learningData.Context[msg.Content] = true
			}
		}

		_ = c.learningManager.SaveLearningData(ctx, learningData)
	}

	// Validasi response type jika diatur
	if c.options.ResponseType != "" && !validateResponseType(aiResponse.Content, c.options.ResponseType) {
		// Jika response type tidak sesuai, kirim pesan system untuk mengubah format
		formatMessage := fmt.Sprintf("Mohon ubah format response menjadi %s", c.options.ResponseType)
		messages.Add(Message{
			Content: formatMessage,
			Role:    User,
		})

		// Kirim ulang request dengan instruksi format
		aiResponse, err = c.Send(messages, additionalSystemMessage...)
		if err != nil {
			return Message{}, err
		}
		messages.Add(aiResponse)

		// Simpan ulang percakapan setelah format diperbaiki
		_, _ = c.saveSessionMessages(sessionID, messages)
	}

	return aiResponse, nil
}

// validateResponse memvalidasi bahwa response sesuai dengan parameter yang diminta
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

// Report melaporkan sesi chat ketika terjadi percakapan diluar konteks
func (c *CsAI) Report(sessionID string) error {
	m := c.getModelMessage()
	sm, err := c.getSessionMessages(sessionID)
	if err != nil {
		return err
	}
	m.Add(sm...)
	return writeMessagesToLog(sessionID, "ai/report", m)
}

func (c *CsAI) Send(messages Messages, additionalSystemMessage ...string) (content Message, err error) {
	// messages dari setup model
	systemMessage := c.getModelMessage(additionalSystemMessage...)

	var roleMessage []map[string]interface{}
	//===============================USER MESSAGE=================================
	callMessages := append(systemMessage, messages...)
	for _, ms := range callMessages {
		msMap, err := ms.MessageToMap()
		if err != nil {
			return Message{}, err
		}
		roleMessage = append(roleMessage, msMap)
	}

	//===============================CALL API=================================
	reqBody := map[string]interface{}{
		"model":             c.Model.ModelName(),
		"messages":          roleMessage,
		"frequency_penalty": 0,
		"max_tokens":        1200,
		"presence_penalty":  -1.5,
		"stop":              nil,
		"stream":            false,
		"stream_options":    nil,
		"temperature":       0.2, // 🔥 Bikin jawaban lebih ringkas & to the point
		"top_p":             0.7, // 🔥 Kurangi variasi dalam jawaban
		"tools":             nil,
		"tool_choice":       "auto",
		"logprobs":          false,
		"top_logprobs":      nil,
	}

	if !c.options.UseTool {
		reqBody["tool_choice"] = "none"
	}

	//===============================ADD INTENT =================================
	var function []map[string]interface{}
	for _, intent := range c.intents {
		param, err2 := convertParam(intent.Param())
		if err2 != nil {
			return Message{}, err2
		}
		function = append(function, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        intent.Code(),
				"description": strings.Join(intent.Description(), ", "),
				"parameters":  param,
			},
		})
	}
	reqBody["tools"] = function

	//===============================REQUEST=================================
	result, err := Request(c.Model.ApiURL(), "POST", reqBody, func(request *http.Request) {
		request.Header.Set("Authorization", "Bearer "+c.ApiKey)
	})
	if err != nil {
		return Message{}, err
	}
	content, err = MessageFromMap(result)
	if err != nil {
		return Message{}, err
	}

	messages.Add(content)

	return content, nil
}

func (c *CsAI) Add(h Intent) {
	//jika intent tidak pernah ditambahkan lakukan append ke c.intents
	if !c.containsIntent(h) {
		c.intents = append(c.intents, h)
	}
}

// AddMiddleware adds a middleware to the chain
func (c *CsAI) AddMiddleware(middleware Middleware) {
	c.middlewareChain.Add(middleware)
}

// AddMiddlewareFunc adds a function-based middleware to the chain
func (c *CsAI) AddMiddlewareFunc(name string, appliesTo []string, priority int, handler func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error)) {
	middleware := NewMiddlewareFunc(name, appliesTo, priority, handler)
	c.middlewareChain.Add(middleware)
}

// AddGlobalMiddleware adds a global middleware that applies to all intents
func (c *CsAI) AddGlobalMiddleware(name string, priority int, handler func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error)) {
	middleware := NewMiddlewareFunc(name, []string{}, priority, handler)
	c.middlewareChain.Add(middleware)
}

// Adds adds multiple intents with a shared middleware
func (c *CsAI) Adds(intents []Intent, middleware Middleware) {
	// Add all intents first
	for _, intent := range intents {
		c.Add(intent)
	}

	// Add middleware that applies to these intents
	c.middlewareChain.Add(middleware)
}

// AddsWithMiddleware adds multiple intents with multiple middlewares
func (c *CsAI) AddsWithMiddleware(intents []Intent, middlewares []Middleware) {
	// Add all intents first
	for _, intent := range intents {
		c.Add(intent)
	}

	// Add all middlewares
	for _, middleware := range middlewares {
		c.middlewareChain.Add(middleware)
	}
}

// AddsWithFunc adds multiple intents with a function-based middleware
func (c *CsAI) AddsWithFunc(intents []Intent, middlewareName string, priority int, handler func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error)) {
	// Extract intent codes
	intentCodes := make([]string, len(intents))
	for i, intent := range intents {
		intentCodes[i] = intent.Code()
		c.Add(intent)
	}

	// Create middleware that applies to these specific intents
	middleware := NewMiddlewareFunc(middlewareName, intentCodes, priority, handler)
	c.middlewareChain.Add(middleware)
}

// AddMiddlewareToIntents adds a middleware to specific intent codes
func (c *CsAI) AddMiddlewareToIntents(intentCodes []string, middleware Middleware) {
	// Create a new middleware that only applies to these intent codes
	specificMiddleware := &MiddlewareFunc{
		name:      middleware.Name() + "_specific",
		appliesTo: intentCodes,
		priority:  middleware.Priority(),
		handler: func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error) {
			return middleware.Handle(ctx, mctx, next)
		},
	}
	c.middlewareChain.Add(specificMiddleware)
}

// AddMiddlewareFuncToIntents adds a function-based middleware to specific intent codes
func (c *CsAI) AddMiddlewareFuncToIntents(intentCodes []string, middlewareName string, priority int, handler func(ctx context.Context, mctx *MiddlewareContext, next MiddlewareNext) (interface{}, error)) {
	middleware := NewMiddlewareFunc(middlewareName, intentCodes, priority, handler)
	c.middlewareChain.Add(middleware)
}

// AddWithAuth adds multiple intents with authentication middleware
func (c *CsAI) AddWithAuth(intents []Intent, requiredRole string) {
	// Add all intents
	intentCodes := make([]string, len(intents))
	for i, intent := range intents {
		intentCodes[i] = intent.Code()
		c.Add(intent)
	}

	// Add authentication middleware for these intents
	authMiddleware := NewAuthenticationMiddleware(requiredRole)
	c.AddMiddlewareToIntents(intentCodes, authMiddleware)
}

// AddWithRateLimit adds multiple intents with rate limiting
func (c *CsAI) AddWithRateLimit(intents []Intent, maxRequests int, timeWindow time.Duration) {
	// Add all intents
	intentCodes := make([]string, len(intents))
	for i, intent := range intents {
		intentCodes[i] = intent.Code()
		c.Add(intent)
	}

	// Add rate limiting middleware for these intents
	rateLimitMiddleware := NewRateLimitMiddleware(maxRequests, timeWindow)
	c.AddMiddlewareToIntents(intentCodes, rateLimitMiddleware)
}

// AddWithCache adds multiple intents with caching middleware
func (c *CsAI) AddWithCache(intents []Intent, ttl time.Duration) {
	// Add all intents
	intentCodes := make([]string, len(intents))
	for i, intent := range intents {
		intentCodes[i] = intent.Code()
		c.Add(intent)
	}

	// Add caching middleware for these intents
	cacheMiddleware := NewCacheMiddleware(ttl, intentCodes)
	c.middlewareChain.Add(cacheMiddleware)
}

// AddWithLogging adds multiple intents with logging middleware
func (c *CsAI) AddWithLogging(intents []Intent, logger *log.Logger) {
	// Add all intents
	intentCodes := make([]string, len(intents))
	for i, intent := range intents {
		intentCodes[i] = intent.Code()
		c.Add(intent)
	}

	// Add logging middleware for these intents
	loggingMiddleware := NewLoggingMiddleware(logger)
	c.AddMiddlewareToIntents(intentCodes, loggingMiddleware)
}

func (c *CsAI) containsIntent(i Intent) bool {
	for _, intent := range c.intents {
		if intent.Code() == i.Code() {
			return true
		}
	}
	return false
}

func (c *CsAI) getModelMessage(additionalSystemMessage ...string) (m Messages) {
	m = make(Messages, 0)

	for _, s := range c.Model.Train() {
		m.Add(Message{
			Content: s,
			Role:    System,
		})
	}

	for _, s := range additionalSystemMessage {
		m.Add(Message{
			Content: s,
			Role:    System,
		})
	}

	today := time.Now()
	//parse today to Wendesday, 2023 February 01
	date := today.Format("Monday, 2006 January 02")

	m.Add(Message{
		Content: fmt.Sprintf("Hari ini adalah tanggal %v", date),
		Role:    System,
	})
	return
}

func (c *CsAI) getSessionMessages(sessionID string) (ms []Message, err error) {
	if c.options.Redis == nil || sessionID == "" {
		return
	}

	ctx := context.Background()
	val, err := c.options.Redis.Get(ctx, sessionID).Result()
	if errors.Is(err, redis.Nil) {
		// Key tidak ditemukan di Redis
		return nil, nil
	} else if err != nil {
		// Error lain saat mengambil data
		return nil, err
	}

	//ubah val ke list message
	err = json.Unmarshal([]byte(val), &ms)
	if err != nil {
		return nil, err
	}

	return ms, nil
	return
}

func (c *CsAI) saveSessionMessages(sessionID string, m []Message) ([]Message, error) {
	if c.options.Redis == nil || sessionID == "" {
		return m, nil
	}

	ctx := context.Background()
	data, err := json.Marshal(m)
	if err != nil {
		// optional: log error
		return m, err
	}

	err = c.options.Redis.Set(ctx, sessionID, data, 12*time.Hour).Err()
	if err != nil {
		return nil, err
	}

	// 🔥 Simpan juga ke file Text
	if c.options.LogChatFile {
		err = writeMessagesToLog(sessionID, "", m)
		if err != nil {
			return m, err // optional: bisa juga di-log aja kalau ga pengen ganggu redis ops
		}
	}

	return m, err
}
func writeMessagesToLog(sessionID string, dir string, messages []Message) error {
	logDir := "ai/log"
	if dir != "" {
		logDir = dir
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	date := time.Now().Format("2006-01-02")
	fileName := fmt.Sprintf("%s_%s.txt", sessionID, date)

	filePath := filepath.Join(logDir, fileName)
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, msg := range messages {
		// Kalau role bukan "user", pakai role sebagai nama
		participantName := msg.Name
		if msg.Role != User || participantName == "" {
			participantName = string(msg.Role)
		}

		line := fmt.Sprintf("[%s]: %s", participantName, msg.Content)
		if msg.ToolCallID != "" {
			line += fmt.Sprintf(" (%s)", msg.ToolCallID)
		}
		line += "\n"

		if _, err := file.WriteString(line); err != nil {
			return err
		}
	}

	return nil
}

// Tambahkan fungsi untuk mendapatkan data pembelajaran
func (c *CsAI) GetLearningData(ctx context.Context, days int) ([]LearningData, error) {
	if c.learningManager == nil {
		return nil, nil
	}
	return c.learningManager.GetLearningData(ctx, days)
}

// Tambahkan fungsi untuk memberikan feedback
func (c *CsAI) AddFeedback(ctx context.Context, sessionID string, feedback int) error {
	if c.learningManager == nil {
		return nil
	}

	// Ambil data pembelajaran terakhir untuk session ini
	messages, err := c.getSessionMessages(sessionID)
	if err != nil || len(messages) < 2 {
		return err
	}

	// Ambil query dan response terakhir
	var query, response string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == User {
			query = messages[i].Content
		}
		if messages[i].Role == Assistant {
			response = messages[i].Content
			break
		}
	}

	learningData := LearningData{
		Query:     query,
		Response:  response,
		Tools:     make([]string, 0),
		Context:   make(map[string]interface{}),
		Timestamp: time.Now(),
		Feedback:  feedback,
	}

	return c.learningManager.SaveLearningData(ctx, learningData)
}
