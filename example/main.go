package main

import (
	"context"
	cs_ai "git.aksaratech.com/aksaratech/project/aksara-ai"
	"git.aksaratech.com/aksaratech/project/aksara-ai/example/intents"
	"git.aksaratech.com/aksaratech/project/aksara-ai/model"
	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
)

const (
	RedisDB       = 0
	RedisAddr     = "localhost:6379"
	RedisPassword = ""
)

func main() {
	e := echo.New()
	e.Static("/static", "example/assets")

	redis := redis.NewClient(&redis.Options{
		DB:       RedisDB,
		Addr:     RedisAddr,
		Password: RedisPassword,
	})
	redis.FlushDB(context.Background())
	csAI := cs_ai.New(cs_ai.APIKEY, model.NewDeepSeekChat(), cs_ai.Options{
		UseTool:     true,
		Redis:       redis,
		LogChatFile: true,
	})
	csAI.Add(intents.BrandInfo{})
	csAI.Add(intents.ProductCatalog{})
	csAI.Add(intents.StockProduct{})
	csAI.Add(intents.Report{})
	csAI.Add(intents.AvailabilityCapster{})
	csAI.Add(intents.BookingCapster{})
	csAI.Add(intents.ListService{})

	e.POST("/chat", func(ctx echo.Context) error {
		var userMessage cs_ai.UserMessage
		err := ctx.Bind(&userMessage)
		if err != nil {
			return err
		}

		systemMessage := []string{
			"nama kamu adalah Hairo",
			"pelanggan yg menghubungi bernama " + userMessage.ParticipantName,
		}

		message1, err2 := csAI.Exec(userMessage.ParticipantName, userMessage, systemMessage...)
		if err2 != nil {
			return err2
		}

		return ctx.JSON(200, Res{
			Message: message1.Content,
		})
	})

	e.GET("/chat-ui", chatUI)

	e.POST("/chat-ui/report/:sessionID", func(ctx echo.Context) error {
		sessionID := ctx.Param("sessionID")
		err := csAI.Report(sessionID)
		if err != nil {
			return err
		}

		return ctx.JSON(200, Res{
			Message: "Report sent successfully",
		})
	})

	err := e.Start(":8080")
	if err != nil {
		panic(err)
	}
}

type Res struct {
	Message string `json:"message"`
}
