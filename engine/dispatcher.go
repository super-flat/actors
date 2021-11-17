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
	// create actor data store
	actorCache := cache.New(time.Millisecond*-1, time.Millisecond*-1)
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
	go x.passivate()
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

func (x *ActorDispatcher) passivate() {
	// TODO: move to a configuration
	maxInactivity := time.Second * 10

	for {
		// TODO: make this configurable
		time.Sleep(time.Second * 5)
		// if there are items, start passivating
		if x.isReceiving && x.actors.ItemCount() > 0 {
			// fmt.Printf("(dispatcher) checking %d actors for inactivity\n", x.actors.ItemCount())
			stopping := []*Actor{}
			// loop over actors
			for actorID, actorIface := range x.actors.Items() {
				actor, ok := actorIface.Object.(*Actor)
				if !ok {
					fmt.Printf("(dispatcher) bad actor in state, id=%s\n", actorID)
					continue
				}
				idleTime := actor.IdleTime()
				if actor.IdleTime() > maxInactivity {
					fmt.Printf("(dispatcher) actor %s idle %v seconds\n", actor.ID, idleTime.Round(time.Second).Seconds())
					actor.Stop()
					stopping = append(stopping, actor)
				}

			}
			for _, actor := range stopping {
				for {
					if !actor.acceptingMessages {
						x.actors.Delete(actor.ID)
						// TODO: dump any remaining messages back into queue
						break
					}
				}
				fmt.Printf("(dispatcher) actor passivated, id=%s\n", actor.ID)
			}
		}

	}
}

type dispatcherMessage struct {
	msg     *actorsv1.Command
	replyTo chan *actorsv1.Response
}
