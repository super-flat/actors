package actors

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

type actorRequest struct {
	actorID string
	replyTo chan<- *Mailbox
}

// Dispatcher directly manages actors and dispatches messages to them
type Dispatcher struct {
	isReceiving   bool
	actors        *ActorMap
	actorRequests chan *actorRequest

	maxActorInactivity   time.Duration
	passivationFrequency time.Duration
	bufferSize           int

	askTimeout time.Duration

	actorFactory ActorFactory
}

// NewActorDispatcher returns a new Dispatcher
func NewActorDispatcher(actorFactory ActorFactory, opts ...DispatcherOpt) *Dispatcher {
	// create the dispatcher
	dispatcher := &Dispatcher{
		isReceiving:          false,
		actors:               NewActorMap(100),
		maxActorInactivity:   5 * time.Second,
		passivationFrequency: 5 * time.Second,
		bufferSize:           3000,
		askTimeout:           5 * time.Second,
		actorFactory:         actorFactory,
	}
	// set the custom options to override the default values
	for _, opt := range opts {
		opt(dispatcher)
	}
	// set the request channel
	dispatcher.actorRequests = make(chan *actorRequest, dispatcher.bufferSize)
	// return the dispatcher
	return dispatcher
}

// Send a message to a specific actor
func (x *Dispatcher) Send(ctx context.Context, actorID string, msg proto.Message) (proto.Message, error) {
	if !x.isReceiving {
		return nil, errors.New("not ready to process messages")
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
		case <-time.After(x.askTimeout):
			return nil, errors.New("command processing timeout")
		}
	}
}

// Start the Dispatcher
func (x *Dispatcher) Start() {
	if x.isReceiving {
		return
	}
	// go x.actorLoop()
	go x.passivateLoop()
	x.isReceiving = true
}

// getActor gets or creates an actor in a thread-safe manner
func (x *Dispatcher) getActor(actorID string) *Mailbox {
	// actorChan := make(chan *Mailbox)
	// r := &actorRequest{actorID: actorID, replyTo: actorChan}
	// x.actorRequests <- r
	// actor := <-actorChan
	// return actor
	factory := func() *Mailbox {
		return NewMailbox(actorID, x.actorFactory)
	}
	actor := x.actors.GetOrCreate(actorID, factory)
	return actor
}

// // actorLoop runs in a goroutine to create actors one at a time
// func (x *Dispatcher) actorLoop() {
// 	for {
// 		req := <-x.actorRequests
// 		// get or create the actor from cache
// 		actor, exists := x.actors.Get(req.actorID)
// 		if !exists {
// 			actor = NewMailbox(req.actorID, x.actorFactory)
// 			x.actors.Set(actor)
// 		}
// 		req.replyTo <- actor
// 	}
// }

// AwaitTermination blocks until the dispatcher is ready to shut down
func (x *Dispatcher) AwaitTermination() {
	awaitFrequency := time.Millisecond * 100
	for {
		if !x.isReceiving {
			return
		}
		time.Sleep(awaitFrequency)
	}

}

// passivateLoop runs in a goroutine and inactive actors
func (x *Dispatcher) passivateLoop() {
	for {
		time.Sleep(x.passivationFrequency)
		// if there are items, start passivating
		if x.isReceiving {
			// loop over actors
			for _, actor := range x.actors.List() {
				idleTime := actor.IdleTime()
				if idleTime >= x.maxActorInactivity {
					log.Printf("(dispatcher) actor %s idle %v seconds\n", actor.ID, idleTime.Round(time.Second).Seconds())
					// stop the actor
					actor.Stop()
					// remove actor from map
					x.actors.Delete(actor.ID)
					log.Printf("(dispatcher) actor passivated, id=%s\n", actor.ID)
				}
			}
		}

	}
}

type ActorMap struct {
	actors map[string]*Mailbox
	mtx    sync.Mutex
}

func NewActorMap(initialCapacity int) *ActorMap {
	return &ActorMap{
		actors: make(map[string]*Mailbox, initialCapacity),
		mtx:    sync.Mutex{},
	}
}

func (x *ActorMap) Get(id string) (value *Mailbox, exists bool) {
	x.mtx.Lock()
	defer x.mtx.Unlock()
	value, exists = x.actors[id]
	return value, exists
}

func (x *ActorMap) GetOrCreate(id string, factory func() *Mailbox) (value *Mailbox) {
	x.mtx.Lock()
	defer x.mtx.Unlock()
	actor, exists := x.actors[id]
	if !exists {
		actor = factory()
		x.actors[actor.ID] = actor
	}
	return actor
}

func (x *ActorMap) Set(value *Mailbox) {
	x.mtx.Lock()
	defer x.mtx.Unlock()
	x.actors[value.ID] = value
}
func (x *ActorMap) Delete(id string) {
	x.mtx.Lock()
	defer x.mtx.Unlock()
	delete(x.actors, id)
}

func (x *ActorMap) List() []*Mailbox {
	out := make([]*Mailbox, 0, len(x.actors))
	for _, actor := range x.actors {
		out = append(out, actor)
	}
	return out
}
