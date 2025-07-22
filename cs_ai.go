package cs_ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// ============================== TYPES ==============================
// (moved to types.go)
// ============================== END TYPES ==============================

func New(ApiKey string, modeler Modeler, o ...Options) *CsAI {
	cs := &CsAI{
		ApiKey:          ApiKey,
		Model:           modeler,
		middlewareChain: NewMiddlewareChain(),
	}
	if len(o) > 0 {
		cs.options = o[0]
		if cs.options.EnableLearning && cs.options.Redis != nil {
			// cs.learningManager = NewLearningManager(cs.options.Redis) // This line is removed
		}
	}
	return cs
}

type CsAI struct {
	ApiKey  string
	Model   Modeler
	intents []Intent
	options Options
	// learningManager *LearningManager // This line is removed
	middlewareChain *MiddlewareChain
}

// Exec mengeksekusi pesan ke AI
func (c *CsAI) Exec(ctx context.Context, sessionID string, userMessage UserMessage, additionalSystemMessage ...string) (Message, error) {
	// ambil pesan lama (jika ada)
	oldMessages, _ := c.GetSessionMessages(sessionID) // error bisa diabaikan
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

		_, _ = c.SaveSessionMessages(sessionID, messages) // simpan percakapan
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
				_, _ = c.SaveSessionMessages(sessionID, messages)
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
	_, _ = c.SaveSessionMessages(sessionID, messages)

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
		_, _ = c.SaveSessionMessages(sessionID, messages)
	}

	return aiResponse, nil
}

// Report melaporkan sesi chat ketika terjadi percakapan diluar konteks
func (c *CsAI) Report(sessionID string) error {
	m := c.getModelMessage()
	sm, err := c.GetSessionMessages(sessionID)
	if err != nil {
		return err
	}
	m.Add(sm...)
	return WriteMessagesToLog(sessionID, "ai/report", m)
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
		// Use configurable temperature and top_p if set, else fallback to defaults
		"temperature":  0.2,
		"top_p":        0.7,
		"tools":        nil,
		"tool_choice":  "auto",
		"logprobs":     false,
		"top_logprobs": nil,
	}
	if c.options.Temperature != 0 {
		reqBody["temperature"] = c.options.Temperature
	}
	if c.options.TopP != 0 {
		reqBody["top_p"] = c.options.TopP
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

// AddMessageToSession adds a message to the session history without triggering LLM or tool call logic.
// This is useful for logging, manual interventions, or injecting system/user/assistant/tool messages from outside the AI engine.
// It loads the current session messages, appends the new message, and saves the updated session.
func (c *CsAI) AddMessageToSession(sessionID string, msg Message) error {
	if sessionID == "" {
		return fmt.Errorf("sessionID cannot be empty")
	}
	messages, err := c.GetSessionMessages(sessionID)
	if err != nil && messages == nil {
		// If not found, start a new session
		messages = make(Messages, 0)
	} else if err != nil {
		return err
	}
	messages = append(messages, msg)
	_, err = c.SaveSessionMessages(sessionID, messages)
	return err
}
