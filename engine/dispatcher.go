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

type actorRequest struct {
	actorID string
	replyTo chan<- *ActorMailbox
}

// ActorDispatcher directly manages actors and dispatches messages to them
type ActorDispatcher struct {
	isReceiving          bool
	actors               *cache.Cache
	actorRequests        chan *actorRequest
	maxActorInactivity   time.Duration
	passivationFrequence time.Duration
	actorFactory         ActorFactory
}

// NewActorDispatcher returns a new ActorDispatcher
func NewActorDispatcher(actorFactory ActorFactory) *ActorDispatcher {
	// number of messages this node can dispatch at the same time
	bufferSize := 100
	// create actor data store
	actorCache := cache.New(time.Millisecond*-1, time.Millisecond*-1)
	// create the dispatcher
	return &ActorDispatcher{
		isReceiving:          false,
		actors:               actorCache,
		actorRequests:        make(chan *actorRequest, bufferSize),
		maxActorInactivity:   time.Second * 5,
		passivationFrequence: time.Second * 5,
		actorFactory:         actorFactory,
	}
}

// Send a message to a specific actor
func (x *ActorDispatcher) Send(ctx context.Context, msg *actorsv1.Command) (*actorsv1.Response, error) {
	if !x.isReceiving {
		return nil, errors.New("not ready")
	}
	// set a unique message id
	msg.MessageId = uuid.NewString()
	for {
		// get the actor ref
		actor := x.getActor(msg.ActorId)
		// send the message, get the reply channel
		success, replyChan := actor.AddToMailbox(msg)
		if !success {
			continue
		}
		// try to get response up to a timeout
		select {
		case resp := <-replyChan:
			return resp, nil
		case <-time.After(time.Second * 5):
			return nil, errors.New("timeout")
		}
	}
}

// Start the ActorDispatcher
func (x *ActorDispatcher) Start() {
	if x.isReceiving {
		return
	}
	go x.actorLoop()
	go x.passivateLoop()
	x.isReceiving = true
}

// getActor gets or creates an actor in a thread-safe manner
func (x *ActorDispatcher) getActor(actorID string) *ActorMailbox {
	actorChan := make(chan *ActorMailbox)
	r := &actorRequest{actorID: actorID, replyTo: actorChan}
	x.actorRequests <- r
	actor := <-actorChan
	return actor
}

// actorLoop runs in a goroutine to create actors one at a time
func (x *ActorDispatcher) actorLoop() {
	for {
		req := <-x.actorRequests
		// get or create the actor from cache
		var actor *ActorMailbox
		value, exists := x.actors.Get(req.actorID)
		if exists {
			actor, _ = value.(*ActorMailbox)
		} else {
			actor = NewActorMailbox(req.actorID, x.actorFactory)
			x.actors.SetDefault(actor.ID, actor)
		}
		req.replyTo <- actor
	}
}

// AwaitTermination blocks unil the dispatcher is ready to shut down
func (x *ActorDispatcher) AwaitTermination() {
	for x.isReceiving {
		time.Sleep(time.Millisecond * 100)
	}
}

// passivateLoop runs in a goroutine and inactive actors
func (x *ActorDispatcher) passivateLoop() {
	for {
		// TODO: make this configurable
		time.Sleep(x.passivationFrequence)
		// if there are items, start passivating
		if x.isReceiving && x.actors.ItemCount() > 0 {
			// loop over actors
			for actorID, actorIface := range x.actors.Items() {
				actor, ok := actorIface.Object.(*ActorMailbox)
				if !ok {
					fmt.Printf("(dispatcher) bad actor in state, id=%s\n", actorID)
					continue
				}
				idleTime := actor.IdleTime()
				if actor.IdleTime() >= x.maxActorInactivity {
					fmt.Printf("(dispatcher) actor %s idle %v seconds\n", actor.ID, idleTime.Round(time.Second).Seconds())
					actor.Stop()
					x.actors.Delete(actor.ID)
					fmt.Printf("(dispatcher) actor passivated, id=%s\n", actor.ID)
				}
			}
		}

	}
}
