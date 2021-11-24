package actors

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/patrickmn/go-cache"
	"google.golang.org/protobuf/proto"
)

type actorRequest struct {
	actorID string
	replyTo chan<- *Mailbox
}

// Dispatcher directly manages actors and dispatches messages to them
type Dispatcher struct {
	isReceiving          bool
	actors               *cache.Cache
	actorRequests        chan *actorRequest
	maxActorInactivity   time.Duration
	passivationFrequency time.Duration
	actorFactory         ActorFactory
}

// NewActorDispatcher returns a new Dispatcher
func NewActorDispatcher(actorFactory ActorFactory) *Dispatcher {
	// number of messages this node can dispatch at the same time
	bufferSize := 3000 // TODO make it configurable
	// create actor data store
	actorCache := cache.New(cache.NoExpiration, cache.NoExpiration)
	// create the dispatcher
	return &Dispatcher{
		isReceiving:          false,
		actors:               actorCache,
		actorRequests:        make(chan *actorRequest, bufferSize),
		maxActorInactivity:   time.Second * 5, // TODO make it configurable
		passivationFrequency: time.Second * 5, // TODO make it configurable
		actorFactory:         actorFactory,
	}
}

// Send a message to a specific actor
func (x *Dispatcher) Send(ctx context.Context, actorID string, msg proto.Message) (proto.Message, error) {
	if !x.isReceiving {
		return nil, errors.New("not ready")
	}
	for {
		// get the actor ref
		actor := x.getActor(actorID)
		// send the message, get the reply channel
		success, replyChan := actor.AddToMailbox(ctx, msg)
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

// Start the Dispatcher
func (x *Dispatcher) Start() {
	if x.isReceiving {
		return
	}
	go x.actorLoop()
	go x.passivateLoop()
	x.isReceiving = true
}

// getActor gets or creates an actor in a thread-safe manner
func (x *Dispatcher) getActor(actorID string) *Mailbox {
	actorChan := make(chan *Mailbox)
	r := &actorRequest{actorID: actorID, replyTo: actorChan}
	x.actorRequests <- r
	actor := <-actorChan
	return actor
}

// actorLoop runs in a goroutine to create actors one at a time
func (x *Dispatcher) actorLoop() {
	for {
		req := <-x.actorRequests
		// get or create the actor from cache
		var actor *Mailbox
		value, exists := x.actors.Get(req.actorID)
		if exists {
			actor, _ = value.(*Mailbox)
		} else {
			actor = NewMailbox(req.actorID, x.actorFactory)
			x.actors.SetDefault(actor.ID, actor)
		}
		req.replyTo <- actor
	}
}

// AwaitTermination blocks until the dispatcher is ready to shut down
func (x *Dispatcher) AwaitTermination() {
	for x.isReceiving {
		time.Sleep(time.Millisecond * 100)
	}
}

// passivateLoop runs in a goroutine and inactive actors
func (x *Dispatcher) passivateLoop() {
	for {
		// TODO: make this configurable
		time.Sleep(x.passivationFrequency)
		// if there are items, start passivating
		if x.isReceiving && x.actors.ItemCount() > 0 {
			// loop over actors
			for actorID, actorIface := range x.actors.Items() {
				actor, ok := actorIface.Object.(*Mailbox)
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
