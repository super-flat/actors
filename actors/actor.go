package actors

import (
	"context"

	"google.golang.org/protobuf/proto"
)

// Actor knows how to receive and process messages
type Actor interface {
	Receive(ctx context.Context, command proto.Message, replyToChan chan<- proto.Message) error
}

// ActorFactory is a function that returns an actor, to be used as a factory
type ActorFactory func(actorID string) Actor
