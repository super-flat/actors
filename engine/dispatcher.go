package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	cache "github.com/patrickmn/go-cache"
	actorsv1 "github.com/super-flat/actors/gen/actors/v1"
)

// ActorDispatcher directly manages actors and dispatches messages to them
type ActorDispatcher struct {
	msgQueue    chan *dispatcherMessage
	isReceiving bool
	actors      *cache.Cache
}

// NewActorDispatcher returns a new ActorDispatcher
func NewActorDispatcher() *ActorDispatcher {
	// number of messages this node can dispatch at the same time
	bufferSize := 100
	// create actor cache that shuts down actors on eviction
	actorCache := cache.New(time.Second*3, time.Second*7)
	actorCache.OnEvicted(evictActor)
	// create the dispatcher
	return &ActorDispatcher{
		msgQueue:    make(chan *dispatcherMessage, bufferSize),
		isReceiving: false,
		actors:      actorCache,
	}
}

// Send a message to a specific actor
func (x *ActorDispatcher) Send(ctx context.Context, msg *actorsv1.Command) (*actorsv1.Response, error) {
	if !x.isReceiving {
		return nil, errors.New("not ready")
	}
	// set a unique message id
	msg.MessageId = uuid.NewString()
	// create reply channel
	replyChan := make(chan *actorsv1.Response, 1)
	// wrap the message
	wrapped := &dispatcherMessage{msg: msg, replyTo: replyChan}
	// put message into queue
	x.msgQueue <- wrapped
	// try to read response form reply channel
	// TODO: implement a timeout here
	resp := <-replyChan
	return resp, nil
}

// Start the ActorDispatcher
func (x *ActorDispatcher) Start() {
	if x.isReceiving {
		return
	}
	go x.process()
	x.isReceiving = true
}

// process runs in the background listening for new messages, creates actors
// if needed, and adds messages to that actors mailbox
func (x *ActorDispatcher) process() {
	for {
		msg := <-x.msgQueue
		// get or create the actor from cache
		var actor *Actor
		value, exists := x.actors.Get(msg.msg.GetActorId())
		if exists {
			actor, _ = value.(*Actor)
		} else {
			actor = NewActor(msg.msg.GetActorId())
			fmt.Printf("(dispatcher) creating actor, id=%s\n", actor.ID)
		}
		actor.AddToMailbox(msg.msg, msg.replyTo)
		// re-write the actor into cache so it resets the expiry
		x.actors.SetDefault(actor.ID, actor)
	}
}

// AwaitTermination blocks unil the dispatcher is ready to shut down
func (x *ActorDispatcher) AwaitTermination() {
	for {
		if x.isReceiving {
			// fmt.Println("running...")
			time.Sleep(time.Second * 3)
		}
	}
}

// evictActor is used in go-cache to shut down actors when they are evicted
// from the cache
func evictActor(actorID string, actor interface{}) {
	typedActor, ok := actor.(*Actor)
	if ok {
		fmt.Printf("(dispatcher) passivating actor, id='%s'\n", typedActor.ID)
		typedActor.Stop()
	}
}

type dispatcherMessage struct {
	msg     *actorsv1.Command
	replyTo chan *actorsv1.Response
}
