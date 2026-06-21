package queue

import "context"

type Queue interface {
	Publish(ctx context.Context, exchange string, key string, derivery *Derivery) error
	Consume(ctx context.Context, queue string, prefetchCount int, requeue bool, handler func(derivery *Derivery, exit chan struct{}) error) error
	Close() error
	CreateQueue(ctx context.Context, name string, opts *QueueOptions) (string, error)
	DeleteQueue(ctx context.Context, name string) error
	DeclareExchange(ctx context.Context, name, kind string, durable bool) error
	CreateBoundQueue(ctx context.Context, exchange string) (string, error)
}

type Derivery struct {
	Body          []byte
	CorrelationID string
	ReplyTo       string
	Headers       map[string]interface{}
}

type QueueOptions struct {
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool
}
