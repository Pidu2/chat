package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/Pidu2/chat/internal/messaging"
)

type Message struct {
	Message string `json:"message" binding:"required"`
}

type UserMessage struct {
	Username string  `json:"username" binding:"required"`
	Message  Message `json:"message" binding:"required"`
}

func main() {
	// INIT DB
	database, err := sql.Open("sqlite3", "./messages.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	statement, err := database.Prepare("CREATE TABLE IF NOT EXISTS message (id INTEGER PRIMARY KEY, username TEXT, message TEXT, timestamp TEXT)")
	if err != nil {
		log.Fatal(err)
	}
	statement.Exec()
	log.Println("Table created or already exists.")

	// INIT RabbitMQ Connection IN
	client, err := messaging.NewClient("amqp://user:password@localhost:5672/", "inQueue")
	if err != nil {
		log.Fatalf("Failed to initialize RabbitMQ client: %v", err)
	}
	defer client.Close()
	msgs, err := client.Consume()
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}
	forever := make(chan bool)

	// INIT RabbitMQ Connection OUT
	clientOut, err := messaging.NewClient("amqp://user:password@localhost:5672/", "outQueue")
	if err != nil {
		log.Fatalf("Failed to initialize RabbitMQ client: %v", err)
	}
	defer clientOut.Close()

	// Read from IN queue, write to DB, write to OUT queue
	go func() {
		for d := range msgs {
			var userMessage UserMessage
			if err := json.Unmarshal(d.Body, &userMessage); err != nil {
				log.Printf("Error decoding JSON: %s", err)
				continue
			}
			log.Printf("Message Received: %+v", userMessage)
			if insertMessage(database, userMessage.Username, userMessage.Message.Message) {
				if err := clientOut.Publish(userMessage); err != nil {
					log.Fatalf("Failed to publish message: %v", err)
				}
			}
		}
	}()

	<-forever
}

func insertMessage(database *sql.DB, username string, message string) bool {
	statement, err := database.Prepare("INSERT INTO message (username, message, timestamp) VALUES (?, ?, ?)")
	if err != nil {
		log.Fatal(err)
		return false
	}
	_, err = statement.Exec(username, message, time.Now().Format("2006-01-02 15:04:05"))
	if err != nil {
		log.Fatal(err)
		return false
	}
	log.Printf("Inserted data: %s %s", username, message)
	return true
}
