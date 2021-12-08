package actors

import (
	"context"

	"google.golang.org/protobuf/proto"
)

// Actor knows how to receive and process messages
type Actor interface {
	// Receive accept messages and handle them
	Receive(ctx context.Context, command proto.Message, replyToChan chan<- proto.Message) error
	// Init initialize the actor. This can be used to set up
	// state refresh for persistence entity
	Init(ctx context.Context) error
}

// ActorFactory is a function that returns an actor, to be used as a factory
type ActorFactory func(actorID string) Actor
