package messaging

import (
	"encoding/json"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Client struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
	Queue   amqp.Queue
}

// NewClient initializes and returns a Client object
func NewClient(url, queueName string) (*Client, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	q, err := ch.QueueDeclare(
		queueName, // name
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	return &Client{
		Conn:    conn,
		Channel: ch,
		Queue:   q,
	}, nil
}

// Publish sends a message to the queue
func (c *Client) Publish(data interface{}) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.Channel.Publish(
		"",           // exchange
		c.Queue.Name, // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

// Consume starts consuming messages from the queue
func (c *Client) Consume() (<-chan amqp.Delivery, error) {
	return c.Channel.Consume(
		c.Queue.Name, // queue
		"",           // consumer
		true,         // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
}

// Close cleanly closes the channel and connection
func (c *Client) Close() {
	c.Channel.Close()
	c.Conn.Close()
}
