package queue_test

import (
	"context"
	"errors"
	"testing"

	"github.com/CSKU-Lab/queue"
)

const CONN_STR = "amqp://admin:password@localhost:5673"

func TestFailedConnection(t *testing.T) {
	_, err := queue.NewRabbitMQ("")
	if err == nil {
		t.Error("Cannot establish connection to rabbitmq need to be failed")
	}
}

/*
 * Happy Path
 */
func TestPublishAndConsumeFromQueue(t *testing.T) {
	q, err := queue.NewRabbitMQ(CONN_STR)
	if err != nil {
		t.Error(err)
	}

	ctx := context.Background()

	defer q.DeleteQueue(ctx, "test")

	qName, err := q.CreateQueue(ctx, "test", nil)
	if err != nil {
		t.Error(err)
	}

	err = q.Publish(ctx, "", qName, &queue.Derivery{
		Body: []byte("Hello World"),
	})
	if err != nil {
		t.Error(err)
	}

	err = q.Consume(ctx, qName, 1, func(derivery *queue.Derivery, exit chan struct{}) error {
		expected := "Hello World"
		actual := string(derivery.Body)

		if actual != expected {
			t.Errorf("Got %s, want %s", actual, expected)
		}

		exit <- struct{}{}

		return nil
	})
	if err != nil {
		t.Error(err)
	}
}

/*
 * Create Queue
 */
func TestCreateQueueWithOption(t *testing.T) {
	q, err := queue.NewRabbitMQ(CONN_STR)
	if err != nil {
		t.Error(err)
	}

	defer q.DeleteQueue(context.Background(), "create-queue-with-option")

	_, err = q.CreateQueue(context.Background(), "create-queue-with-option", &queue.QueueOptions{
		AutoDelete: true,
	})
	if err != nil {
		t.Error(err)
	}
}

func TestCreateQueueWhenContextCanceled(t *testing.T) {
	q, err := queue.NewRabbitMQ(CONN_STR)
	if err != nil {
		t.Error(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = q.CreateQueue(ctx, "test", &queue.QueueOptions{
		Durable: true,
	})
	if err == nil {
		t.Errorf("Context canceled. CreateQueue should return error")
	}
}

func TestCreateQueueOnClosedChannel(t *testing.T) {
	q, err := queue.NewRabbitMQ(CONN_STR)
	if err != nil {
		t.Error(err)
	}

	q.Close()

	_, err = q.CreateQueue(context.Background(), "test", &queue.QueueOptions{
		Durable: true,
	})
	if err == nil {
		t.Errorf("Channel closed. CreateQueue should return error")
	}
}

func TestCreateSameQueueWithDifferentOption(t *testing.T) {
	q, err := queue.NewRabbitMQ(CONN_STR)
	if err != nil {
		t.Error(err)
	}

	_, err = q.CreateQueue(context.Background(), "create-queue-with-option", &queue.QueueOptions{
		AutoDelete: true,
	})
	if err != nil {
		t.Error(err)
	}

	_, err = q.CreateQueue(context.Background(), "create-queue-with-option", &queue.QueueOptions{
		AutoDelete: false,
	})
	if err == nil {
		t.Error(err)
	}
}

/*
 * Delete Queue
 */
func TestDeleteQueueWhenContextCanceled(t *testing.T) {
	q, err := queue.NewRabbitMQ(CONN_STR)
	if err != nil {
		t.Error(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = q.DeleteQueue(ctx, "test")
	if err == nil {
		t.Errorf("Context canceled. CreateQueue should return error")
	}
}

func TestDeleteQueueOnClosedChannel(t *testing.T) {
	q, err := queue.NewRabbitMQ(CONN_STR)
	if err != nil {
		t.Error(err)
	}

	q.Close()

	err = q.DeleteQueue(context.Background(), "test")
	if err == nil {
		t.Errorf("Channel closed. CreateQueue should return error")
	}
}

/*
 * Publish
 */
func TestPublishWhenContextCanceled(t *testing.T) {
	q, err := queue.NewRabbitMQ(CONN_STR)
	if err != nil {
		t.Error(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = q.Publish(ctx, "", "some-queue", &queue.Derivery{
		Body: []byte("hello world"),
	})
	if err == nil {
		t.Errorf("Context canceled. Publish should return error")
	}
}

func TestPublishOnClosedChannel(t *testing.T) {
	q, err := queue.NewRabbitMQ(CONN_STR)
	if err != nil {
		t.Error(err)
	}

	q.Close()

	err = q.Publish(context.Background(), "", "close-channel", &queue.Derivery{
		Body: []byte("Hello World"),
	})
	if err == nil {
		t.Error("Channel closed. Pubish should return error")
	}
}

/*
 * Consume
 */
func TestConsumeWhenContextCanceled(t *testing.T) {
	q, err := queue.NewRabbitMQ(CONN_STR)
	if err != nil {
		t.Error(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = q.Consume(ctx, "test", 1, func(derivery *queue.Derivery, exit chan struct{}) error {
		return nil
	})
	if err == nil {
		t.Errorf("Context canceled. Consume should return error")
	}
}

func TestConsumeHandlerError(t *testing.T) {
	q, err := queue.NewRabbitMQ(CONN_STR)
	if err != nil {
		t.Error(err)
	}

	defer q.DeleteQueue(context.Background(), "test")

	ctx := context.Background()

	qName, err := q.CreateQueue(ctx, "test", nil)
	if err != nil {
		t.Error(err)
	}

	err = q.Publish(ctx, "", qName, &queue.Derivery{
		Body: []byte("Hello World"),
	})
	if err != nil {
		t.Error(err)
	}

	err = q.Consume(ctx, qName, 1, func(derivery *queue.Derivery, exit chan struct{}) error {
		return errors.New("some error happened")
	})
	if err == nil {
		t.Error("Handler returned error. Consume should return error")
	}
}

func TestConsumeOnClosedChannel(t *testing.T) {
	q, err := queue.NewRabbitMQ(CONN_STR)
	if err != nil {
		t.Error(err)
	}

	q.Close()

	err = q.Consume(context.Background(), "test", 1, func(derivery *queue.Derivery, exit chan struct{}) error {
		return nil
	})
	if err == nil {
		t.Errorf("Channel closed. Consume should return error")
	}
}

/*
 * Close
 */
func TestCloseConnection(t *testing.T) {
	q, err := queue.NewRabbitMQ(CONN_STR)
	if err != nil {
		t.Error(err)
	}

	if err := q.Close(); err != nil {
		t.Error(err)
	}
}
