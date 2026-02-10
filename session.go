package cs_ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (c *CsAI) GetSessionMessages(sessionID string) (ms []Message, err error) {
	if c.options.StorageProvider != nil {
		ctx := context.Background()
		return c.options.StorageProvider.GetSessionMessages(ctx, sessionID)
	}

	// Legacy Redis support
	if c.options.Redis != nil {
		key := fmt.Sprintf("ai:session:%s", sessionID)
		data, err := c.options.Redis.Get(c.options.Redis.Context(), key).Result()
		if err != nil {
			return nil, err
		}
		var messages []Message
		err = json.Unmarshal([]byte(data), &messages)
		if err != nil {
			return nil, err
		}
		return messages, nil
	}

	return nil, nil
}

// GetSessionMessageCount returns the total number of messages in a session.
// This is useful for determining the 1-based message ID of the last message.
func (c *CsAI) GetSessionMessageCount(sessionID string) (int, error) {
	messages, err := c.GetSessionMessages(sessionID)
	if err != nil {
		return 0, err
	}
	return len(messages), nil
}

func (c *CsAI) SaveSessionMessages(sessionID string, m []Message) ([]Message, error) {
	if c.options.StorageProvider != nil {
		ctx := context.Background()
		ttl := c.options.SessionTTL
		if ttl == 0 {
			ttl = 12 * time.Hour // Default TTL 12 jam
		}
		err := c.options.StorageProvider.SaveSessionMessages(ctx, sessionID, m, ttl)
		if err != nil {
			return m, err
		}
		return m, nil
	}

	// Legacy Redis support
	if c.options.Redis != nil {
		key := fmt.Sprintf("ai:session:%s", sessionID)
		data, err := json.Marshal(m)
		if err != nil {
			return m, err
		}

		// Set TTL dari Options atau default 12 jam untuk barbershop
		var ttl time.Duration
		if c.options.SessionTTL > 0 {
			ttl = c.options.SessionTTL
		} else {
			ttl = 12 * time.Hour // Default TTL 12 jam
		}
		err = c.options.Redis.Set(c.options.Redis.Context(), key, data, ttl).Err()
		if err != nil {
			return m, err
		}
		return m, nil
	}

	return m, nil
}

func WriteMessagesToLog(sessionID string, dir string, messages []Message) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	filePath := filepath.Join(dir, fmt.Sprintf("%s.json", sessionID))
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(messages)
}

// GetSystemMessages retrieves pre-chat/default system messages for a session
func (c *CsAI) GetSystemMessages(sessionID string) ([]Message, error) {
	if c.options.StorageProvider != nil {
		ctx := context.Background()
		return c.options.StorageProvider.GetSystemMessages(ctx, sessionID)
	}
	return nil, nil
}

// SaveSystemMessages saves pre-chat/default system messages for a session
// These messages are stored separately from conversation messages and
// can be used to persist custom system prompts per session
func (c *CsAI) SaveSystemMessages(sessionID string, messages []Message) error {
	if c.options.StorageProvider != nil {
		ctx := context.Background()
		ttl := c.options.SessionTTL
		if ttl == 0 {
			ttl = 12 * time.Hour // Default TTL 12 jam
		}
		return c.options.StorageProvider.SaveSystemMessages(ctx, sessionID, messages, ttl)
	}
	return nil
}

// ClearSession deletes all session data (conversation messages and system messages)
func (c *CsAI) ClearSession(sessionID string) error {
	if c.options.StorageProvider != nil {
		ctx := context.Background()
		return c.options.StorageProvider.DeleteSession(ctx, sessionID)
	}

	// Legacy Redis support
	if c.options.Redis != nil {
		key := fmt.Sprintf("ai:session:%s", sessionID)
		systemKey := fmt.Sprintf("ai:system:%s", sessionID)
		return c.options.Redis.Del(c.options.Redis.Context(), key, systemKey).Err()
	}

	return nil
}

// DeleteMessageFromSession removes a specific message and all subsequent messages
// from a session by its 1-based index (truncate from that point onward).
// It ensures the truncation point is safe for the AI API by removing any
// trailing assistant messages with tool_calls that lack their corresponding
// tool response messages.
func (c *CsAI) DeleteMessageFromSession(sessionID string, messageIndex int) error {
	if sessionID == "" {
		return fmt.Errorf("sessionID cannot be empty")
	}

	messages, err := c.GetSessionMessages(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session messages: %v", err)
	}

	// Convert 1-based index to 0-based
	idx := messageIndex - 1
	if idx < 0 || idx >= len(messages) {
		return fmt.Errorf("message index %d out of range (session has %d messages)", messageIndex, len(messages))
	}

	// Truncate: keep only messages before idx
	messages = messages[:idx]

	// Walk backwards to find a safe boundary:
	// Remove trailing tool responses and assistant messages with tool_calls
	// to prevent "insufficient tool messages following tool_calls" API errors.
	for len(messages) > 0 {
		last := messages[len(messages)-1]

		// If last message is a tool response, remove it (orphaned without context)
		if last.Role == Tool {
			messages = messages[:len(messages)-1]
			continue
		}

		// If last message is an assistant with tool_calls, remove it
		if last.Role == Assistant && len(last.ToolCalls) > 0 {
			messages = messages[:len(messages)-1]
			continue
		}

		// Safe boundary found
		break
	}

	// Save updated messages
	_, err = c.SaveSessionMessages(sessionID, messages)
	if err != nil {
		return fmt.Errorf("failed to save session after deleting message: %v", err)
	}

	return nil
}
