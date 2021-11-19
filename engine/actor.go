package engine

import (
	"context"

	actorsv1 "github.com/super-flat/actors/gen/actors/v1"
)

// Actor knows how to receive and process messages
type Actor interface {
	Receive(ctx context.Context, msg *ActorMessage) error
}

// ActorFactory returns an actor
type ActorFactory interface {
	CreateActor(actorID string) Actor
}

type ActorMaker func(actorID string) Actor

type ActorMessage struct {
	Payload *actorsv1.Command
	ReplyTo chan<- *actorsv1.Response
}
