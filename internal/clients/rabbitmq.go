package clients

import (
	"fmt"
	"transfers-api/internal/config"
	"transfers-api/internal/logging"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQClient struct {
	conn  *amqp.Connection
	queue string
}

func NewRabbitMQClient(cfg config.RabbitMQ) *RabbitMQClient {
	conn, err := amqp.Dial(
		fmt.Sprintf(
			"amqp://%s:%s@%s:%d/",
			cfg.Username,
			cfg.Password,
			cfg.Hostname,
			cfg.Port,
		),
	)
	if err != nil {
		logging.Logger.Fatalf("failed to connect to RabbitMQ: %v", err)
	}
	return &RabbitMQClient{
		conn:  conn,
		queue: cfg.QueueName,
	}
}

func (c *RabbitMQClient) Publish(operation string, transferID string) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}
	defer ch.Close()

	_, err = ch.QueueDeclare(
		c.queue,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	body := fmt.Sprintf("%s:%s", operation, transferID)
	err = ch.Publish(
		"",      // exchange
		c.queue, // routing key (queue name)
		false,   // mandatory
		false,   // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}
	return nil
}

func (c *RabbitMQClient) Read() (string, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return "", fmt.Errorf("failed to open channel: %w", err)
	}
	defer ch.Close()

	msg, ok, err := ch.Get(
		c.queue,
		false, // autoAck
	)
	if err != nil {
		return "", fmt.Errorf("failed to get message: %w", err)
	}
	if !ok {
		return "", nil // no hay mensajes
	}

	if err := msg.Ack(false); err != nil {
		return "", fmt.Errorf("failed to ack message: %w", err)
	}

	return string(msg.Body), nil
}