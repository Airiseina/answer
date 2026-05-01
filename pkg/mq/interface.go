package mq

import "context"

type Mq interface {
	WriteMessage(ctx context.Context, message interface{}) error
	Close() error
}
