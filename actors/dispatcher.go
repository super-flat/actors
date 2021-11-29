package actors

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

// Dispatcher directly manages actors and dispatches messages to them
type Dispatcher struct {
	isReceiving bool
	actors      *actorMap

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
		actors:               newActorMap(100),
		maxActorInactivity:   5 * time.Second,
		passivationFrequency: 5 * time.Second,
		askTimeout:           5 * time.Second,
		actorFactory:         actorFactory,
	}
	// set the custom options to override the default values
	for _, opt := range opts {
		opt(dispatcher)
	}
	// return the dispatcher
	return dispatcher
}

// Send a message to a specific actor
func (x *Dispatcher) Send(ctx context.Context, actorID string, msg proto.Message) (proto.Message, error) {
	// get the observability span
	if !x.isReceiving {
		return nil, errors.New("not ready to process messages")
	}
	for {
		// get the actor ref
		actor := x.getActor(ctx, actorID)
		// send the message, get the reply channel
		success, replyChan := actor.Send(ctx, msg)
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
	go x.passivateLoop()
	x.isReceiving = true
}

// getActor gets or creates an actor in a thread-safe manner
func (x *Dispatcher) getActor(ctx context.Context, actorID string) *Mailbox {
	// get the observability span
	spanCtx, span := getSpanContext(ctx, "Actor.Dispatcher.GetOrCreate")
	defer span.End()
	factory := func() *Mailbox {
		return NewMailbox(spanCtx, actorID, x.actorFactory)
	}
	actor := x.actors.GetOrCreate(actorID, factory)
	return actor
}

// passivateLoop runs in a goroutine and inactive actors
func (x *Dispatcher) passivateLoop() {
	for {
		time.Sleep(x.passivationFrequency)
		// if there are items, start passivating
		if x.isReceiving {
			// loop over actors
			for _, actor := range x.actors.GetAll() {
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

// Shutdown disables receiving new messages and shuts down all actors
func (x *Dispatcher) Shutdown() {
	log.Printf("(dispatcher) shutting down")
	x.isReceiving = false
	for {
		// check there are actors to shut down
		if x.actors.Len() == 0 {
			break
		}
		// get actors to delete
		actors := x.actors.GetAll()
		actorWg := &sync.WaitGroup{}
		actorWg.Add(len(actors))
		// create workers with channel to synchronize work
		isDone := false
		numWorkers := 10
		workerWg := &sync.WaitGroup{}
		workChan := make(chan (*Mailbox), numWorkers)
		workerFn := func() {
			workerWg.Add(1)
			for {
				if isDone {
					break
				}
				actor := <-workChan
				actor.Stop()
				x.actors.Delete(actor.ID)
				actorWg.Done()
			}
			workerWg.Done()
		}
		for i := 0; i < numWorkers; i++ {
			go workerFn()
		}
		// send actors to workers for shutdown
		for _, actor := range actors {
			workChan <- actor
		}
		// wait for all actors to be shut down
		actorWg.Wait()
		// instruct workers to shutdown
		isDone = true
		// wait for workers to shut down
		workerWg.Wait()
	}

	log.Printf("(dispatcher) finished shutting down")
}

// actorMap stores actors for an actor system and allows thread-safe
// get/set operations
type actorMap struct {
	actors map[string]*Mailbox
	mtx    sync.Mutex
}

// newActorMap instantiates a new actor map
func newActorMap(initialCapacity int) *actorMap {
	return &actorMap{
		actors: make(map[string]*Mailbox, initialCapacity),
		mtx:    sync.Mutex{},
	}
}

// Len returns the number of actors
func (x *actorMap) Len() int {
	return len(x.actors)
}

// Get retrieves an actor by ID
func (x *actorMap) Get(id string) (value *Mailbox, exists bool) {
	x.mtx.Lock()
	defer x.mtx.Unlock()
	value, exists = x.actors[id]
	return value, exists
}

// GetOrCreate retrieves or creates an actor by ID
func (x *actorMap) GetOrCreate(id string, factory func() *Mailbox) (value *Mailbox) {
	x.mtx.Lock()
	defer x.mtx.Unlock()
	actor, exists := x.actors[id]
	if !exists {
		actor = factory()
		x.actors[actor.ID] = actor
	}
	return actor
}

// Set sets an actor in the map
func (x *actorMap) Set(value *Mailbox) {
	x.mtx.Lock()
	defer x.mtx.Unlock()
	x.actors[value.ID] = value
}

// Delete removes an actor from the map
func (x *actorMap) Delete(id string) {
	x.mtx.Lock()
	defer x.mtx.Unlock()
	delete(x.actors, id)
}

// GetAll returns all actors as a slice
func (x *actorMap) GetAll() []*Mailbox {
	out := make([]*Mailbox, 0, len(x.actors))
	for _, actor := range x.actors {
		out = append(out, actor)
	}
	return out
}
