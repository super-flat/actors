package actors

import (
	"context"
	"time"
)

// Spawn is a utility function that help create a new actor and return the ActorRef,
// so you can send it messages
func Spawn(ctx context.Context, ID string, actorFactory ActorFactory) *ActorRef {
	// get the observability span
	spanCtx, span := getSpanContext(ctx, "Actor.Spawn")
	defer span.End()
	// set the actorRef size
	// TODO: make configurable
	mailboxSize := 10
	// create the inner actor
	actor := actorFactory(ID)
	// create the actor actorRef
	actorRef := &ActorRef{
		ID:                ID,
		mailbox:           make(chan *commandWrapper, mailboxSize),
		stop:              make(chan bool),
		lastUpdated:       time.Now(),
		acceptingMessages: true,
		actor:             actor,
	}
	// async initialize the actor and start processing messages
	go actorRef.run(spanCtx)
	// return the actorRef
	return actorRef
}
