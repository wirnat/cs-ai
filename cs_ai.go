package cs_ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
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

		// Validate penalty values
		if err := validatePenaltyValues(cs.options); err != nil {
			// Log warning but don't fail initialization
			fmt.Printf("Warning: Invalid penalty values: %v\n", err)
		}

		// Initialize storage provider
		if cs.options.StorageProvider != nil {
			// Use provided storage provider
		} else if cs.options.StorageConfig != nil {
			// Create storage provider from config
			storageProvider, err := NewStorageProvider(*cs.options.StorageConfig)
			if err != nil {
				// Fallback to Redis if available
				if cs.options.Redis != nil {
					// Create Redis storage provider
					redisConfig := StorageConfig{
						Type:          StorageTypeRedis,
						RedisAddress:  "localhost:6379", // Default fallback
						RedisPassword: "",
						RedisDB:       0,
						SessionTTL:    cs.options.SessionTTL,
						Timeout:       5 * time.Second,
					}
					cs.options.StorageProvider, _ = NewRedisStorageProvider(redisConfig)
				}
			} else {
				cs.options.StorageProvider = storageProvider
			}
		} else if cs.options.Redis != nil {
			// Legacy Redis support - create Redis storage provider
			redisConfig := StorageConfig{
				Type:          StorageTypeRedis,
				RedisAddress:  "localhost:6379", // Default fallback
				RedisPassword: "",
				RedisDB:       0,
				SessionTTL:    cs.options.SessionTTL,
				Timeout:       5 * time.Second,
			}
			cs.options.StorageProvider, _ = NewRedisStorageProvider(redisConfig)
		}

		if cs.options.EnableLearning && cs.options.StorageProvider != nil {
			// cs.learningManager = NewLearningManager(cs.options.StorageProvider) // This line is removed
		}
	}

	// Set default SessionTTL jika tidak diatur
	if cs.options.SessionTTL == 0 {
		cs.options.SessionTTL = 12 * time.Hour // Default 12 jam
	}

	// Initialize security manager if security options are provided
	if len(o) > 0 && o[0].SecurityOptions != nil {
		cs.securityManager = NewSecurityManager(o[0].SecurityOptions)
	} else {
		// Default security manager with basic protection
		defaultSecurity := &SecurityOptions{
			MaxRequestsPerMinute:  10,
			MaxRequestsPerHour:    100,
			MaxRequestsPerDay:     1000,
			SpamThreshold:         0.5,
			EnableSecurityLogging: true,
			UserIDField:           "ParticipantName",
		}
		cs.securityManager = NewSecurityManager(defaultSecurity)
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
	securityManager *SecurityManager
}

// Exec mengeksekusi pesan ke AI
func (c *CsAI) Exec(ctx context.Context, sessionID string, userMessage UserMessage, additionalSystemMessage ...string) (Message, error) {
	// Security check
	userID := userMessage.ParticipantName
	if userID == "" {
		userID = "anonymous"
	}

	// Check security if enabled
	if c.securityManager != nil {
		if err := c.securityManager.CheckSecurity(userID, sessionID, userMessage.Message); err != nil {
			return Message{}, fmt.Errorf("security check failed: %v", err)
		}
	}

	// Save system messages if provided
	if len(additionalSystemMessage) > 0 {
		systemMessages := make([]Message, 0, len(additionalSystemMessage))
		for _, s := range additionalSystemMessage {
			systemMessages = append(systemMessages, Message{
				Content: s,
				Role:    System,
			})
		}
		if err := c.SaveSystemMessages(sessionID, systemMessages); err != nil {
			// Log warning but don't fail - system messages are optional
			fmt.Printf("Warning: Failed to save system messages: %v\n", err)
		}
	}

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
	invalidToolCalls := 0
	successfulToolCalls := 0
	maxInvalidToolCalls := 2
	processor := &DefaultResponseProcessor{}

	// Create tool lookup and definition hash map for cache invalidation
	toolDefinitionHashes := make(map[string]string)
	intentsByCode := make(map[string]Intent, len(c.intents))
	availableTools := make([]string, 0, len(c.intents))
	for _, intent := range c.intents {
		intentCode := intent.Code()
		intentsByCode[intentCode] = intent
		availableTools = append(availableTools, intentCode)

		if hash, err := generateToolDefinitionHash(intent); err == nil {
			toolDefinitionHashes[intentCode] = hash
		}
	}
	sort.Strings(availableTools)

	for len(aiResponse.ToolCalls) > 0 {
		loopCount++
		if loopCount > maxLoop {
			fmt.Println("Max loop reached, returning last known response.")
			// Cek semua tool calls yang ada di cache
			var validResponses []Message
			for _, tool := range aiResponse.ToolCalls {
				toolDefinitionHash := toolDefinitionHashes[tool.Function.Name]
				cacheKey := ToolCacheKey{
					FunctionName:       tool.Function.Name,
					Arguments:          tool.Function.Arguments,
					ToolDefinitionHash: toolDefinitionHash,
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
		toolCallResponses := make(map[string]Message)

		// Proses semua tool calls dalam satu iterasi
		for _, tool := range aiResponse.ToolCalls {
			toolDefinitionHash := toolDefinitionHashes[tool.Function.Name]
			cacheKey := ToolCacheKey{
				FunctionName:       tool.Function.Name,
				Arguments:          tool.Function.Arguments,
				ToolDefinitionHash: toolDefinitionHash,
			}

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

			intent, exists := intentsByCode[tool.Function.Name]
			if !exists {
				invalidToolCalls++
				unavailableToolResponse := buildToolErrorMessage(
					tool.Id,
					"tool_not_found",
					tool.Function.Name,
					availableTools,
				)
				toolCallResponses[tool.Id] = unavailableToolResponse
				newMessages.Add(unavailableToolResponse)
				continue
			}

			p := intent.Param()
			err := json.Unmarshal([]byte(tool.Function.Arguments), &p)
			if err != nil {
				invalidToolCalls++
				invalidArgsResponse := buildToolErrorMessage(
					tool.Id,
					"invalid_tool_arguments",
					tool.Function.Name,
					availableTools,
				)
				toolCallResponses[tool.Id] = invalidArgsResponse
				newMessages.Add(invalidArgsResponse)
				continue
			}

			paramMap, ok := p.(map[string]interface{})
			if !ok {
				invalidToolCalls++
				invalidArgsResponse := buildToolErrorMessage(
					tool.Id,
					"invalid_tool_argument_format",
					tool.Function.Name,
					availableTools,
				)
				toolCallResponses[tool.Id] = invalidArgsResponse
				newMessages.Add(invalidArgsResponse)
				continue
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
			successfulToolCalls++
			newMessages.Add(toolResponse)
		}

		// Verifikasi bahwa setiap tool call memiliki response
		for _, tool := range aiResponse.ToolCalls {
			if _, exists := toolCallResponses[tool.Id]; !exists {
				invalidToolCalls++
				unhandledToolResponse := buildToolErrorMessage(
					tool.Id,
					"tool_call_unhandled",
					tool.Function.Name,
					availableTools,
				)
				toolCallResponses[tool.Id] = unhandledToolResponse
				newMessages.Add(unhandledToolResponse)
			}
		}

		if len(newMessages) == 0 {
			break
		}

		messages.Add(newMessages...)
		if invalidToolCalls >= maxInvalidToolCalls && successfulToolCalls == 0 {
			safeResponse := buildToolSafetyFallbackMessage(userMessage.ParticipantName)
			messages.Add(safeResponse)
			_, _ = c.SaveSessionMessages(sessionID, messages)
			return safeResponse, nil
		}

		aiResponse, err = c.Send(messages, additionalSystemMessage...)
		if err != nil {
			return Message{}, err
		}
		messages.Add(aiResponse)

		if aiResponse.Role == Assistant && len(aiResponse.ToolCalls) == 0 {
			break
		}
	}

	if invalidToolCalls > 0 && successfulToolCalls == 0 && aiResponse.Role == Assistant && len(aiResponse.ToolCalls) == 0 {
		safeResponse := buildToolSafetyFallbackMessage(userMessage.ParticipantName)
		messages.Add(safeResponse)
		_, _ = c.SaveSessionMessages(sessionID, messages)
		return safeResponse, nil
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
		"frequency_penalty": 0.0,
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
	if c.options.FrequencyPenalty != 0 {
		reqBody["frequency_penalty"] = c.options.FrequencyPenalty
	}
	if c.options.PresencePenalty != 0 {
		reqBody["presence_penalty"] = c.options.PresencePenalty
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

	if len(c.intents) > 0 {
		toolNames := make([]string, 0, len(c.intents))
		for _, intent := range c.intents {
			toolNames = append(toolNames, intent.Code())
		}
		sort.Strings(toolNames)

		m.Add(Message{
			Content: fmt.Sprintf(
				"Daftar tool yang tersedia: %s. Hanya panggil tool dari daftar ini dengan nama persis sama. Jika tidak ada tool yang cocok, jangan memanggil tool.",
				strings.Join(toolNames, ", "),
			),
			Role: System,
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

func buildToolErrorMessage(toolCallID string, errorCode string, requestedTool string, availableTools []string) Message {
	payload := map[string]interface{}{
		"error": map[string]interface{}{
			"code":            errorCode,
			"requested_tool":  requestedTool,
			"available_tools": availableTools,
		},
	}

	content, err := json.Marshal(payload)
	if err != nil {
		content = []byte(`{"error":{"code":"tool_error_payload_failed"}}`)
	}

	return Message{
		Content:    string(content),
		Role:       Tool,
		ToolCallID: toolCallID,
	}
}

func buildToolSafetyFallbackMessage(participantName string) Message {
	return Message{
		Content: "Maaf, saya tidak dapat melanjutkan karena ada ketidaksesuaian pemanggilan tool internal. Silakan coba lagi.",
		Name:    participantName,
		Role:    Assistant,
	}
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

// GetUsageAnalytics returns aggregated usage analytics for a given time range
func (c *CsAI) GetUsageAnalytics(from, to time.Time) ([]UserAnalytics, error) {
	if c.securityManager == nil {
		return nil, fmt.Errorf("security manager not initialized")
	}

	analytics := c.securityManager.GetAllAnalytics(from, to)
	return analytics, nil
}

// GetSecurityEvents returns security events for a given time range
func (c *CsAI) GetSecurityEvents(from, to time.Time) []SecurityLog {
	if c.securityManager == nil {
		return []SecurityLog{}
	}

	var events []SecurityLog
	analytics := c.securityManager.GetAllAnalytics(from, to)

	for _, userAnalytics := range analytics {
		userID := userAnalytics.UserID

		// Convert UserAnalytics to SecurityLog format
		// This is a simplified conversion - in real implementation, you'd need more detailed logs
		events = append(events, SecurityLog{
			UserID:    userID,
			Timestamp: userAnalytics.StartTime,
			Allowed:   userAnalytics.AllowedRequests > 0,
			SpamScore: userAnalytics.AverageSpamScore,
		})
	}

	return events
}

// GetTotalUsers returns the count of unique users in the given time range
func (c *CsAI) GetTotalUsers(from, to time.Time) int64 {
	if c.securityManager == nil {
		return 0
	}

	analytics := c.securityManager.GetAllAnalytics(from, to)
	return int64(len(analytics))
}

// GetRequestCounts returns total request counts for the given time range
func (c *CsAI) GetRequestCounts(from, to time.Time) (total int64, allowed int64, denied int64) {
	if c.securityManager == nil {
		return 0, 0, 0
	}

	analytics := c.securityManager.GetAllAnalytics(from, to)

	for _, userAnalytics := range analytics {
		total += int64(userAnalytics.TotalRequests)
		allowed += int64(userAnalytics.AllowedRequests)
		denied += int64(userAnalytics.BlockedRequests)
	}

	return total, allowed, denied
}

// GetSpamAttempts returns the count of spam attempts in the given time range
func (c *CsAI) GetSpamAttempts(from, to time.Time) int64 {
	if c.securityManager == nil {
		return 0
	}

	var spamAttempts int64
	analytics := c.securityManager.GetAllAnalytics(from, to)

	for _, userAnalytics := range analytics {
		if userAnalytics.AverageSpamScore > 0 {
			spamAttempts += int64(userAnalytics.TotalRequests)
		}
	}

	return spamAttempts
}

// GetRateLimitHits returns the count of rate limit hits in the given time range
func (c *CsAI) GetRateLimitHits(from, to time.Time) int64 {
	if c.securityManager == nil {
		return 0
	}

	// This would need to be implemented based on rate limiter logs
	// For now, return 0 as the SecurityManager doesn't expose this directly
	return 0
}

// ClearToolCache clears the tool cache for a specific session or all sessions
// This is useful when tool definitions have changed and you want to force fresh execution
func (c *CsAI) ClearToolCache(sessionID ...string) error {
	// Note: This method is for future implementation when we have persistent tool cache
	// Currently tool cache is only in-memory per Exec() call, so this is a placeholder
	// for when we implement persistent tool caching across sessions

	if len(sessionID) > 0 {
		// Clear cache for specific session
		// Implementation would depend on storage provider
		return nil
	}

	// Clear all tool caches
	// Implementation would depend on storage provider
	return nil
}

// InvalidateToolDefinitionCache forces regeneration of tool definitions
// Call this method after modifying tool parameters, required fields, or descriptions
func (c *CsAI) InvalidateToolDefinitionCache() {
	// This method can be used to signal that tool definitions have changed
	// In the current implementation, tool definitions are regenerated on each Send() call
	// But this provides a hook for future persistent caching implementations
}

// validatePenaltyValues validates that penalty values are within valid ranges
func validatePenaltyValues(options Options) error {
	if options.FrequencyPenalty != 0 && (options.FrequencyPenalty < -2.0 || options.FrequencyPenalty > 2.0) {
		return fmt.Errorf("frequency_penalty must be between -2.0 and 2.0, got %f", options.FrequencyPenalty)
	}
	if options.PresencePenalty != 0 && (options.PresencePenalty < -2.0 || options.PresencePenalty > 2.0) {
		return fmt.Errorf("presence_penalty must be between -2.0 and 2.0, got %f", options.PresencePenalty)
	}
	return nil
}
