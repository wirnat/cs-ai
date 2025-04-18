package cs_ai

import (
	"encoding/json"
	"fmt"
)

type Role string

// consts role
const (
	System    Role = "system"
	User      Role = "user"
	Assistant Role = "assistant"
	Tool      Role = "tool"
)

type Message struct {
	Content    string     `json:"content"`
	Name       string     `json:"name"`
	Role       Role       `json:"role"`
	ToolCalls  []ToolCall `json:"tool_calls"`
	ToolCallID string     `json:"tool_call_id"`
}

type ToolCall struct {
	Index    int    `json:"index"`
	Id       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func MessageFromMap(result map[string]interface{}) (content Message, err error) {
	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return Message{}, nil
	}

	jsonChoices, err := json.Marshal(choices[0].(map[string]interface{})["message"])
	if err != nil {
		fmt.Println("Error marshaling choices:", err)
		return Message{}, nil
	}

	err = json.Unmarshal(jsonChoices, &content)
	if err != nil {
		fmt.Println("Error unmarshaling choices:", err)
		return Message{}, nil
	}

	return
}

func (m Message) MessageToMap() (map[string]interface{}, error) {
	// Konversi struct Message ke JSON string
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	// Konversi JSON string ke map[string]interface{}
	var result map[string]interface{}
	err = json.Unmarshal(jsonBytes, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

type Messages []Message

func (m *Messages) Add(messages ...Message) {
	*m = append(*m, messages...)
}
