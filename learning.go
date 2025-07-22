package cs_ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

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

func (c *CsAI) GetLearningData(ctx context.Context, days int) ([]LearningData, error) {
	if c.options.Redis == nil {
		return nil, nil
	}
	lm := NewLearningManager(c.options.Redis)
	return lm.GetLearningData(ctx, days)
}

func (c *CsAI) AddFeedback(ctx context.Context, sessionID string, feedback int) error {
	if c.options.Redis == nil {
		return nil
	}
	lm := NewLearningManager(c.options.Redis)
	learningData, err := lm.GetLearningData(ctx, 1)
	if err != nil {
		return err
	}
	for i := range learningData {
		if learningData[i].Query == sessionID {
			learningData[i].Feedback = feedback
			_ = lm.SaveLearningData(ctx, learningData[i])
		}
	}
	return nil
}
