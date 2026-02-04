package queue

import (
	"context"
	"errors"

	amqp "github.com/rabbitmq/amqp091-go"
)

type rabbitmq struct {
	conn *amqp.Connection
}

func NewRabbitMQ(connStr string) (Queue, error) {
	conn, err := amqp.Dial(connStr)
	if err != nil {
		return nil, err
	}

	return &rabbitmq{
		conn: conn,
	}, nil
}

func (r *rabbitmq) CreateQueue(ctx context.Context, name string, opts *QueueOptions) (string, error) {
	ch, err := r.conn.Channel()
	if err != nil {
		return "", err
	}

	if opts == nil {
		opts = &QueueOptions{}
	}

	select {
	case <-ctx.Done():
		ch.Close()
		return "", ctx.Err()
	default:
	}

	q, err := ch.QueueDeclare(name, opts.Durable, opts.AutoDelete, opts.Exclusive, opts.NoWait, nil)
	if err != nil {
		return "", err
	}

	return q.Name, nil
}

func (r *rabbitmq) DeleteQueue(ctx context.Context, name string) error {
	ch, err := r.conn.Channel()
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		ch.Close()
		return ctx.Err()
	default:
	}

	_, err = ch.QueueDelete(name, false, false, false)
	return err
}

func (r *rabbitmq) Publish(ctx context.Context, exchange string, key string, derivery *Derivery) error {
	ch, err := r.conn.Channel()
	if err != nil {
		return err
	}

	err = ch.Confirm(false)
	if err != nil {
		return err
	}

	err = ch.PublishWithContext(
		ctx,
		exchange,
		key,
		false,
		false,
		amqp.Publishing{
			ContentType:   "application/json",
			CorrelationId: derivery.CorrelationID,
			ReplyTo:       derivery.ReplyTo,
			Body:          derivery.Body,
		},
	)
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case confirmed := <-ch.NotifyPublish(make(chan amqp.Confirmation)):
		if confirmed.Ack {
			return nil
		} else {
			return errors.New("failed to publish message to the queue")
		}
	}

}

func (r *rabbitmq) Consume(ctx context.Context, queue string, prefetchCount int, handler func(derivery *Derivery, exit chan struct{}) error) error {
	ch, err := r.conn.Channel()
	if err != nil {
		return err
	}

	err = ch.Qos(prefetchCount, 0, false)
	if err != nil {
		return err
	}

	msgs, err := ch.ConsumeWithContext(
		ctx,
		queue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	errChan := make(chan error, 1)
	exitChan := make(chan struct{}, 1)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errChan:
			return err
		case <-exitChan:
			return nil
		case msg, ok := <-msgs:
			if !ok {
				return nil
			}
			go func() {
				derivery := &Derivery{
					Body:          msg.Body,
					CorrelationID: msg.CorrelationId,
					ReplyTo:       msg.ReplyTo,
				}

				if err := handler(derivery, exitChan); err != nil {
					errChan <- err
					msg.Nack(false, true)
					return
				}

				if err = msg.Ack(false); err != nil {
					errChan <- err
					return
				}
			}()
		}
	}
}

func (r *rabbitmq) Close() {
	r.conn.Close()
}
