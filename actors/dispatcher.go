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
func (d *Dispatcher) Send(ctx context.Context, actorID string, msg proto.Message) (proto.Message, error) {
	// get the observability span
	spanCtx, span := getSpanContext(ctx, "Dispatcher.Send")
	defer span.End()
	if !d.isReceiving {
		return nil, errors.New("not ready to process messages")
	}
	for {
		// get the actor ref
		actor := d.getActor(spanCtx, actorID)
		// send the message, get the reply channel
		success, replyChan := actor.Send(spanCtx, msg)
		if !success {
			continue
		}
		// try to get response up to a timeout
		select {
		case resp := <-replyChan:
			return resp, nil
		case <-time.After(d.askTimeout):
			return nil, errors.New("command processing timeout")
		}
	}
}

// Start the Dispatcher
func (d *Dispatcher) Start() {
	if d.isReceiving {
		return
	}
	go d.passivateLoop()
	d.isReceiving = true
}

// getActor gets or creates an actor in a thread-safe manner
func (d *Dispatcher) getActor(ctx context.Context, actorID string) *ActorRef {
	// get the observability span
	spanCtx, span := getSpanContext(ctx, "Actor.Dispatcher.GetOrCreate")
	defer span.End()
	factory := func() *ActorRef {
		return Spawn(spanCtx, actorID, d.actorFactory)
	}
	actor := d.actors.GetOrCreate(actorID, factory)
	return actor
}

// passivateLoop runs in a goroutine and inactive actors
func (d *Dispatcher) passivateLoop() {
	for {
		time.Sleep(d.passivationFrequency)
		// if there are items, start passivating
		if d.isReceiving {
			// loop over actors
			for _, actor := range d.actors.GetAll() {
				idleTime := actor.IdleTime()
				if idleTime >= d.maxActorInactivity {
					log.Printf("(dispatcher) actor %s idle %v seconds\n", actor.ID, idleTime.Round(time.Second).Seconds())
					// stop the actor
					actor.Stop()
					// remove actor from map
					d.actors.Delete(actor.ID)
					log.Printf("(dispatcher) actor passivated, id=%s\n", actor.ID)
				}
			}
		}
	}
}

// Shutdown disables receiving new messages and shuts down all actors
func (d *Dispatcher) Shutdown() {
	log.Printf("(dispatcher) shutting down")
	d.isReceiving = false
	for {
		// check there are actors to shut down
		if d.actors.Len() == 0 {
			break
		}
		// get actors to delete
		actors := d.actors.GetAll()
		actorWg := &sync.WaitGroup{}
		actorWg.Add(len(actors))
		// create workers with channel to synchronize work
		isDone := false
		numWorkers := 10
		workerWg := &sync.WaitGroup{}
		workChan := make(chan *ActorRef, numWorkers)
		workerFn := func() {
			workerWg.Add(1)
			for {
				if isDone {
					break
				}
				actor := <-workChan
				actor.Stop()
				d.actors.Delete(actor.ID)
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
	actors map[string]*ActorRef
	mtx    sync.Mutex
}

// newActorMap instantiates a new actor map
func newActorMap(initialCapacity int) *actorMap {
	return &actorMap{
		actors: make(map[string]*ActorRef, initialCapacity),
		mtx:    sync.Mutex{},
	}
}

// Len returns the number of actors
func (x *actorMap) Len() int {
	return len(x.actors)
}

// Get retrieves an actor by ID
func (x *actorMap) Get(id string) (value *ActorRef, exists bool) {
	x.mtx.Lock()
	defer x.mtx.Unlock()
	value, exists = x.actors[id]
	return value, exists
}

// GetOrCreate retrieves or creates an actor by ID
func (x *actorMap) GetOrCreate(id string, factory func() *ActorRef) (value *ActorRef) {
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
func (x *actorMap) Set(value *ActorRef) {
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
func (x *actorMap) GetAll() []*ActorRef {
	out := make([]*ActorRef, 0, len(x.actors))
	for _, actor := range x.actors {
		out = append(out, actor)
	}
	return out
}
