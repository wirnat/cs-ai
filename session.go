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
