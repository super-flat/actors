package engine

import (
	"context"

	actorsv1 "github.com/super-flat/actors/gen/actors/v1"
)

// Actor knows how to receive and process messages
type Actor interface {
	Receive(ctx context.Context, msg *ActorMessage) error
}

// ActorMessage wraps a message with a reply channel
type ActorMessage struct {
	Payload *actorsv1.Command
	ReplyTo chan<- *actorsv1.Response
}

// ActorFactory is a function that returns an actor, to be used as a factory
type ActorFactory func(actorID string) Actor
