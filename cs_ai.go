package cs_ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func New(ApiKey string, modeler Modeler, o ...Options) *csAI {
	cs := &csAI{
		ApiKey: ApiKey,
		Model:  modeler,
	}
	if len(o) > 0 {
		cs.options = o[0]
	}
	return cs
}

type csAI struct {
	ApiKey  string
	Model   Modeler
	intents []Intent
	options Options
}

type UserMessage struct {
	Message         string `json:"message"`
	ParticipantName string `json:"participant_name"`
}
type AIResponse struct {
	Response interface{} `json:"response"`
}

// Exec mengeksekusi pesan ke AI
func (c *csAI) Exec(sessionID string, userMessage UserMessage, additionalSystemMessage ...string) (Message, error) {
	// ambil pesan lama (jika ada)
	oldMessages, _ := c.getSessionMessages(sessionID) // error bisa diabaikan
	messages := make(Messages, 0)
	if oldMessages != nil {
		messages = append(messages, oldMessages...)
	}

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
		_, _ = c.saveSessionMessages(sessionID, messages) // simpan percakapan
		return aiResponse, nil
	}

	// ============================== HANDLE TOOL CALLS ==============================
	toolCache := make(map[string]Message)
	maxLoop := 3
	loopCount := 0

	for len(aiResponse.ToolCalls) > 0 {
		loopCount++
		if loopCount > maxLoop {
			fmt.Println("Max loop reached, returning last known response.")
			lastResponse, exists := toolCache[aiResponse.ToolCalls[0].Function.Name]
			if exists {
				_, _ = c.saveSessionMessages(sessionID, messages)
				return lastResponse, nil
			}
			return Message{
				Content: "tunggu sebentar yaa kak",
				Name:    userMessage.ParticipantName,
				Role:    "assistant",
			}, nil
		}

		newMessages := make(Messages, 0)

		for _, tool := range aiResponse.ToolCalls {
			if cachedResponse, exists := toolCache[tool.Function.Name]; exists {
				return cachedResponse, nil
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

					data, err := intent.Handle(paramMap)
					if err != nil {
						return Message{}, err
					}

					dataJson, err := json.Marshal(data)
					if err != nil {
						return Message{}, err
					}

					toolResponse := Message{
						Content:    string(dataJson),
						Role:       Tool,
						ToolCallID: tool.Id,
					}
					toolCache[tool.Function.Name] = toolResponse
					newMessages.Add(toolResponse)
				}
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

	return aiResponse, nil
}

// Report melaporkan sesi chat ketika terjadi percakapan diluar konteks
func (c *csAI) Report(sessionID string) error {
	m := c.getModelMessage()
	sm, err := c.getSessionMessages(sessionID)
	if err != nil {
		return err
	}
	m.Add(sm...)
	return writeMessagesToLog(sessionID, "ai/report", m)
}

func (c *csAI) Send(messages Messages, additionalSystemMessage ...string) (content Message, err error) {
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
		"max_tokens":        500,
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

func (c *csAI) Add(h Intent) {
	//jika intent tidak pernah ditambahkan lakukan append ke c.intents
	if !c.containsIntent(h) {
		c.intents = append(c.intents, h)
	}
}

func (c *csAI) containsIntent(i Intent) bool {
	for _, intent := range c.intents {
		if intent.Code() == i.Code() {
			return true
		}
	}
	return false
}

func (c *csAI) getModelMessage(additionalSystemMessage ...string) (m Messages) {
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
	//parse today to 2023 February 01
	date := today.Format("2006 January 02")

	m.Add(Message{
		Content: fmt.Sprintf("Hari ini adalah tanggal %v", date),
		Role:    System,
	})
	return
}

func (c *csAI) getSessionMessages(sessionID string) (ms []Message, err error) {
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

func (c *csAI) saveSessionMessages(sessionID string, m []Message) ([]Message, error) {
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

type Options struct {
	Redis       *redis.Client //Cache Messages
	UseTool     bool          //Gunakan tool handler api
	LogChatFile bool          //Simpan chat ke file
}
