package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/Pidu2/chat/internal/messaging"
	"github.com/Pidu2/chat/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Adjust this to fit your needs
	},
}

type Message struct {
	Message string `json:"message" binding:"required"`
}

type UserMessage struct {
	Username string  `json:"username" binding:"required"`
	Message  Message `json:"message" binding:"required"`
}

var clients = make(map[*websocket.Conn]bool) // connected clients
var broadcast = make(chan UserMessage)       // broadcast channel
var mutex = sync.Mutex{}                     // mutex to protect clients

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
	r.LoadHTMLGlob("templates/*")
	// Testing-Endpoint to validate the JWT and get the associated username
	r.GET("/validateJWT", middleware.TokenAuthMiddleware(), func(c *gin.Context) {
		username := c.MustGet("username").(string)
		c.JSON(http.StatusOK, gin.H{"username": username})
	})
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"result": "success",
		})
	})
	// Get all messages for user
	r.GET("/messages", middleware.TokenAuthMiddleware(), func(c *gin.Context) {
		username := c.MustGet("username").(string)
		c.HTML(http.StatusOK, "messages.html", gin.H{
			"UserID": username,
		})
	})
	// websocket endpoint
	r.GET("/ws", func(c *gin.Context) {
		wshandler(c.Writer, c.Request)
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
		c.JSON(http.StatusCreated, gin.H{"result": "success"})
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
			broadcast <- userMessage
		}
	}()

	go handleMessages()

	r.Run("0.0.0.0:8081")
	<-forever
}

func wshandler(w http.ResponseWriter, r *http.Request) {
	conn, err := wsupgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not open websocket connection", http.StatusBadRequest)
		return
	}
	defer conn.Close()

	// Register new client
	mutex.Lock()
	clients[conn] = true
	mutex.Unlock()

	// Ensure connection is removed on disconnect
	defer func() {
		mutex.Lock()
		delete(clients, conn)
		mutex.Unlock()
	}()

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if err := conn.WriteMessage(messageType, p); err != nil {
			return
		}
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		mutex.Lock()
		for conn := range clients {
			err := conn.WriteJSON(msg)
			if err != nil {
				conn.Close()
				delete(clients, conn)
			}
		}
		mutex.Unlock()
	}
}
