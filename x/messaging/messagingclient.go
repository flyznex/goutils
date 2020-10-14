package messaging

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

var (
	ErrNoConnection      = errors.New("Connection was not initialize")
	ErrOpenChannel       = errors.New("Can't open channel from connection")
	ErrConnStringIsEmpty = errors.New("Cannot initialize connection to broker, connectionString not set. Have you initialized?")
)

type MessageClient interface {
	Publish(msg []byte, exchangeName string, exchangeType string, routingKey string) error
	PublishOnQueue(msg []byte, queueName string) error
	PublicOnQueueWithContext(ctx context.Context, msg []byte, queueName string) error
	PublicOnQueueRoutingWithContext(ctx context.Context, msg []byte, queueName string, routingKey string) error
	Subscribe(exchangeName string, exchangeType string, consumerName string, queueName string, routingKey string, handlerFunc func(amqp.Delivery)) error
	SubscribeToQueue(queueName string, consumerName string, handlerFunc func(amqp.Delivery)) error
	Close()
}

type AmqpClient struct {
	conn *amqp.Connection
}

func New(conn string) (MessageClient, error) {
	if conn == "" {
		return &AmqpClient{conn: nil}, ErrConnStringIsEmpty
	}
	c, err := amqp.Dial(fmt.Sprintf("%s/", conn))
	if err != nil {
		return &AmqpClient{conn: nil}, err
	}
	return &AmqpClient{conn: c}, nil
}

func (c *AmqpClient) Publish(msg []byte, exchangeName string, exchangeType string, routingKey string) error {
	if c.conn == nil {
		return ErrNoConnection
	}
	ch, err := c.conn.Channel()
	if err != nil {
		return ErrOpenChannel
	}
	defer ch.Close()

	err = ch.ExchangeDeclare(
		exchangeName, //name of exchange
		exchangeType, //type of exchange
		true,         // durable
		false,        // delete when complete
		false,        // internal
		false,        // no wait
		nil,          //arguments
	)
	if err != nil {
		return err
	}

	err = ch.Publish( // Publishes a message onto the queue.
		exchangeName, // exchange
		routingKey,   // routing key      q.Name
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        msg, // Our JSON body as []byte
		})
	return err
}
func (c *AmqpClient) PublishOnQueue(msg []byte, queueName string) error {
	return c.PublicOnQueueWithContext(context.TODO(), msg, queueName)
}
func (c *AmqpClient) PublicOnQueueWithContext(ctx context.Context, msg []byte, queueName string) error {
	return c.PublicOnQueueRoutingWithContext(ctx, msg, queueName, queueName)
}

func (c *AmqpClient) PublicOnQueueRoutingWithContext(ctx context.Context, msg []byte, queueName string, routingKey string) error {
	if c.conn == nil {
		return ErrNoConnection
	}
	ch, err := c.conn.Channel()
	if err != nil {
		return ErrOpenChannel
	}

	defer ch.Close()

	_, err = ch.QueueDeclare( // Declare a queue that will be created if not exists with some args
		queueName, // our queue name
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return err
	}
	// Publishes a message onto the queue.
	err = ch.Publish(
		"",         // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		buildMessage(ctx, msg))
	return err
}

func buildMessage(ctx context.Context, body []byte) amqp.Publishing {
	publishing := amqp.Publishing{
		ContentType: "application/json",
		Body:        body, // Our JSON body as []byte
	}
	// tracing
	// if ctx != nil {
	// 	child := tracing.StartChildSpanFromContext(ctx, "messaging")
	// 	defer child.Finish()
	// 	var val = make(opentracing.TextMapCarrier)
	// 	err := tracing.AddTracingToTextMapCarrier(child, val)
	// 	if err != nil {
	// 		logrus.Errorf("Error injecting span context: %v", err.Error())
	// 	} else {
	// 		publishing.Headers = tracing.CarrierToMap(val)
	// 	}
	// }
	return publishing
}

func (c *AmqpClient) Subscribe(exchangeName string, exchangeType string, consumerName string, queueName string, routingKey string, handlerFunc func(amqp.Delivery)) error {
	if c.conn == nil {
		return ErrNoConnection
	}

	ch, err := c.conn.Channel()
	if err != nil {
		return ErrOpenChannel
	}

	err = ch.ExchangeDeclare(
		exchangeName, // name of the exchange
		exchangeType, // type
		true,         // durable
		false,        // delete when complete
		false,        // internal
		false,        // noWait
		nil,          // arguments
	)
	if err != nil {
		return err
	}

	logrus.Printf("declared Exchange, declaring Queue (%s)", "")
	queue, err := ch.QueueDeclare(
		queueName, // name of the queue
		false,     // durable
		false,     // delete when usused
		false,     // exclusive
		false,     // noWait
		nil,       // arguments
	)
	if err != nil {
		return err
	}

	logrus.Printf("declared Queue (%d messages, %d consumers), binding to Exchange (key '%s')",
		queue.Messages, queue.Consumers, exchangeName)

	err = ch.QueueBind(
		queue.Name,   // name of the queue
		routingKey,   // bindingKey
		exchangeName, // sourceExchange
		false,        // noWait
		nil,          // arguments
	)
	if err != nil {
		return fmt.Errorf("Queue Bind: %s", err)
	}

	msgs, err := ch.Consume(
		queue.Name,   // queue
		consumerName, // consumer
		true,         // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		return err
	}

	go consumeLoop(msgs, handlerFunc)
	return nil
}
func (c *AmqpClient) SubscribeToQueue(queueName string, consumerName string, handlerFunc func(amqp.Delivery)) error {
	if c.conn == nil {
		return ErrNoConnection
	}

	ch, err := c.conn.Channel()
	if err != nil {
		return ErrOpenChannel
	}

	logrus.Printf("Declaring Queue (%s)", queueName)
	queue, err := ch.QueueDeclare(
		queueName, // name of the queue
		false,     // durable
		false,     // delete when usused
		false,     // exclusive
		false,     // noWait
		nil,       // arguments
	)
	if err != nil {
		return err
	}

	msgs, err := ch.Consume(
		queue.Name,   // queue
		consumerName, // consumer
		true,         // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		return err
	}

	go consumeLoop(msgs, handlerFunc)
	return nil
}

func (c *AmqpClient) Close() {
	if c.conn != nil {
		logrus.Infoln("Closing connection to AMQP broker")
		c.conn.Close()
	}
}

func consumeLoop(deliveries <-chan amqp.Delivery, handlerFunc func(d amqp.Delivery)) {
	for d := range deliveries {
		// Invoke the handlerFunc func we passed as parameter.
		handlerFunc(d)
	}
}
