package queue

import (
	"context"
	"errors"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type rabbitmq struct {
	mu      sync.Mutex
	conn    *amqp.Connection
	connStr string
}

func NewRabbitMQ(connStr string) (Queue, error) {
	conn, err := amqp.Dial(connStr)
	if err != nil {
		return nil, err
	}

	return &rabbitmq{
		conn:    conn,
		connStr: connStr,
	}, nil
}

func (r *rabbitmq) dialLocked() error {
	conn, err := amqp.Dial(r.connStr)
	if err != nil {
		return err
	}
	r.conn = conn
	return nil
}

// channel returns an AMQP channel, reconnecting if the connection was dropped.
func (r *rabbitmq) channel() (*amqp.Channel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conn.IsClosed() {
		if err := r.dialLocked(); err != nil {
			return nil, err
		}
	}
	ch, err := r.conn.Channel()
	if err != nil {
		// IsClosed() returned false but connection is dead — reconnect once
		if err2 := r.dialLocked(); err2 != nil {
			return nil, err
		}
		return r.conn.Channel()
	}
	return ch, nil
}

func (r *rabbitmq) CreateQueue(ctx context.Context, name string, opts *QueueOptions) (string, error) {
	ch, err := r.channel()
	if err != nil {
		return "", err
	}
	defer ch.Close()

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
	ch, err := r.channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	_, err = ch.QueueDelete(name, false, false, false)
	return err
}

func (r *rabbitmq) Publish(ctx context.Context, exchange string, key string, derivery *Derivery) error {
	ch, err := r.channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	err = ch.Confirm(false)
	if err != nil {
		return err
	}

	headers := amqp.Table{}
	for k, v := range derivery.Headers {
		headers[k] = v
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
			Headers:       headers,
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

func (r *rabbitmq) Consume(ctx context.Context, queue string, prefetchCount int, requeue bool, handler func(derivery *Derivery, exit chan struct{}) error) error {
	ch, err := r.channel()
	if err != nil {
		return err
	}
	defer ch.Close()

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
		case err := <-errChan:
			return err
		case <-exitChan:
			return nil
		case msg, ok := <-msgs:
			if !ok {
				return nil
			}
			go func() {
				headers := make(map[string]interface{})
				for k, v := range msg.Headers {
					if s, ok := v.(string); ok {
						headers[k] = s
					}
				}
				derivery := &Derivery{
					Body:          msg.Body,
					CorrelationID: msg.CorrelationId,
					ReplyTo:       msg.ReplyTo,
					Headers:       headers,
				}

				if err := handler(derivery, exitChan); err != nil {
					errChan <- err
					msg.Nack(false, requeue)
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

func (r *rabbitmq) DeclareExchange(ctx context.Context, name, kind string, durable bool) error {
	ch, err := r.channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return ch.ExchangeDeclare(name, kind, durable, false, false, false, nil)
}

// CreateBoundQueue creates a server-named auto-delete queue and binds it to the
// given fanout exchange. Returns the generated queue name for the caller to consume.
func (r *rabbitmq) CreateBoundQueue(ctx context.Context, exchange string) (string, error) {
	ch, err := r.channel()
	if err != nil {
		return "", err
	}
	defer ch.Close()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	q, err := ch.QueueDeclare("", false, true, false, false, nil)
	if err != nil {
		return "", err
	}

	if err := ch.QueueBind(q.Name, "", exchange, false, nil); err != nil {
		return "", err
	}

	return q.Name, nil
}

func (r *rabbitmq) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.conn.Close()
}
