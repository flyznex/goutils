package messaging

import (
	"os"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

func TestMessageHandleLoop(t *testing.T) {
	var wg sync.WaitGroup
	var invocations = 0

	var handlerFunc = func(d amqp.Delivery) {
		wg.Add(1)
		logrus.Println("In handlerFunction")
		invocations = invocations + 1
		wg.Done()
	}

	var messageChan = make(chan amqp.Delivery, 1)
	go consumeLoop(messageChan, handlerFunc)
	d := amqp.Delivery{Body: []byte(""), ConsumerTag: ""}
	messageChan <- d
	messageChan <- d
	messageChan <- d
	wg.Wait()
	if want, got := 3, invocations; got != want {
		t.Errorf("Expected invocations: %d, but got %d", want, got)
	}
}

func TestPublishMessage(t *testing.T) {
	ms, err := New(os.Getenv("MH_RABBITMQ_URL_TEST"))
	if err != nil {
		t.Error(err)
		return
	}
	am := ms.(*AmqpClient)
	defer am.Close()
	err = am.Publish([]byte(""), "direct-exchange-test", amqp.ExchangeDirect, "direct-exchange-test")
	if err != nil {
		t.Error(err)
	}
	t.Run("Should consumer", testSubscribe(am))
	t.Run("Publish on queue", testPublishOnQueue(am))
	t.Run("Should consumer on queue", testSubscribeOnQueue(am))

}

func testPublishOnQueue(am *AmqpClient) func(*testing.T) {
	return func(t *testing.T) {
		err := am.PublishOnQueue([]byte("message"), "test-queue")
		if err != nil {
			t.Error(err)
		}
	}
}

func testSubscribeOnQueue(am *AmqpClient) func(*testing.T) {
	return func(t *testing.T) {
		var body = ""
		var handlerFunc = func(msg amqp.Delivery) {
			body = string(msg.Body)
			event := msg.RoutingKey
			logrus.Infof("Got RoutingKey: %s, a message: %v\n", event, body)
		}
		err := am.SubscribeToQueue("test-queue", "consumer-name-test", handlerFunc)
		if err != nil {
			t.Error(err)
			return
		}
		msg := "message"
		err = am.PublishOnQueue([]byte(msg), "test-queue")
		if err != nil {
			t.Error(err)
		}
		if got, want := body, msg; got != want {
			t.Errorf("Expected want message %q got %q", want, got)
		}
	}
}

func testSubscribe(am *AmqpClient) func(*testing.T) {
	return func(t *testing.T) {
		var body = ""
		var handlerFunc = func(msg amqp.Delivery) {
			body = string(msg.Body)
			event := msg.RoutingKey
			logrus.Infof("Got RoutingKey: %s, a message: %v\n", event, body)
		}
		err := am.Subscribe("test-exchange", amqp.ExchangeDirect, "consumer-name-test", "test-queue", "test-event", handlerFunc)
		if err != nil {
			t.Error(err)
			return
		}
		msg := "message"
		err = am.Publish([]byte(msg), "test-exchange", amqp.ExchangeDirect, "test-event")
		if err != nil {
			t.Error(err)
		}
		if got, want := body, msg; got != want {
			t.Errorf("Expected want message %q got %q", want, got)
		}
	}
}
