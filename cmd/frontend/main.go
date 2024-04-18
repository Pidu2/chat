package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/Pidu2/chat/internal/messaging"
	"github.com/Pidu2/chat/internal/middleware"
	"github.com/gin-gonic/gin"
)

type Message struct {
	Message string `json:"message" binding:"required"`
}

type UserMessage struct {
	Username string  `json:"username" binding:"required"`
	Message  Message `json:"message" binding:"required"`
}

func main() {
	// INIT RabbitMQ Connection IN
	client, err := messaging.NewClient("amqp://user:password@localhost:5672/", "inQueue")
	if err != nil {
		log.Fatalf("Failed to initialize RabbitMQ client: %v", err)
	}
	defer client.Close()
	// INIT RabbitMQ Connection OUT
	clientOut, err := messaging.NewClient("amqp://user:password@localhost:5672/", "outQueue")
	if err != nil {
		log.Fatalf("Failed to initialize RabbitMQ client: %v", err)
	}
	defer clientOut.Close()
	msgs, err := clientOut.Consume()
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}
	forever := make(chan bool)

	// INIT GIN
	r := gin.Default()
	// Testing-Endpoint to validate the JWT and get the associated username
	r.GET("/validateJWT", middleware.TokenAuthMiddleware(), func(c *gin.Context) {
		username := c.MustGet("username").(string)
		c.JSON(http.StatusOK, gin.H{"username": username})
	})
	// Get all messages for user
	r.GET("/", middleware.TokenAuthMiddleware(), func(c *gin.Context) {
		username := c.MustGet("username").(string)
		c.JSON(http.StatusOK, gin.H{"allmessages": "messages for user " + username})
		// TODO get actual messages
	})
	// Publish Message
	r.POST("/message", middleware.TokenAuthMiddleware(), func(c *gin.Context) {
		var newMessage Message
		if err := c.BindJSON(&newMessage); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		userMessage := UserMessage{
			Username: c.MustGet("username").(string),
			Message:  newMessage,
		}
		if err := client.Publish(userMessage); err != nil {
			log.Fatalf("Failed to publish message: %v", err)
		}
	})

	// consume messages from out

	go func() {
		for d := range msgs {
			var userMessage UserMessage
			if err := json.Unmarshal(d.Body, &userMessage); err != nil {
				log.Printf("Error decoding JSON: %s", err)
				continue
			}
			log.Printf("Message Received: %+v", userMessage)
		}
	}()

	r.Run("0.0.0.0:8081")
	<-forever
}
