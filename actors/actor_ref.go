package actors

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"google.golang.org/protobuf/proto"
)

// Default values for ExponentialBackOff.
const (
	actorInitMaxRetries = 10 // TODO we can review this
)

// commandWrapper wraps the actual command sent to the actor and
// the context
type commandWrapper struct {
	CommandCtx context.Context
	Command    proto.Message
	ReplyChan  chan<- proto.Message
}

// ActorRef has a mailbox and can process messages one at a time for the underlying actor
type ActorRef struct {
	ID                string
	mailbox           chan *commandWrapper
	msgCount          int
	stop              chan bool
	lastUpdated       time.Time
	acceptingMessages bool
	mtx               sync.RWMutex
	actor             Actor
}

// IdleTime returns how long the actor has been idle as a time.Duration
func (ref *ActorRef) IdleTime() time.Duration {
	return time.Since(ref.lastUpdated)
}

// Send sends a message to the actors' mailbox to be processed and
// supplies an optional reply channel for responses to the sender
func (ref *ActorRef) Send(ctx context.Context, msg proto.Message) (success bool, replyChan <-chan proto.Message) {
	// get the observability span
	spanCtx, span := getSpanContext(ctx, "ActorRef.Send")
	defer span.End()
	// acquire a lock
	ref.mtx.Lock()
	defer ref.mtx.Unlock()
	// process
	if !ref.acceptingMessages {
		return false, nil
	}
	// set update time for activity
	ref.lastUpdated = time.Now()
	// create the reply chan
	replyTo := make(chan proto.Message, 1)
	// if successfully push to channel, return true, else false
	ref.mailbox <- &commandWrapper{
		CommandCtx: spanCtx,
		Command:    msg,
		ReplyChan:  replyTo,
	}
	// return the response
	return true, replyTo
}

// Stop the actor
func (ref *ActorRef) Stop() {
	// acquire a lock
	ref.mtx.Lock()
	// stop future messages
	ref.acceptingMessages = false
	// unlock
	ref.mtx.Unlock()
	// wait for no more messages
	for len(ref.mailbox) != 0 {
		// log.Printf("[mailbox] waiting for mailbox empty, len=%d\n", len(ref.mailbox))
		time.Sleep(time.Millisecond)
	}
	// begin shutdown
	ref.stop <- true
	log.Printf("[actor_ref] (%s) shutting down\n", ref.ID)
}

// run the actor loop
func (ref *ActorRef) run(bootCtx context.Context) {
	// run the actor initialization
	ref.selfInit(bootCtx)
	// run the process loop
	ref.process()
}

// process incoming messages until stop signal
func (ref *ActorRef) process() {
	// run the processing loop
	for {
		select {
		case <-ref.stop:
			return
		case received := <-ref.mailbox:
			// let the actor process the command
			err := ref.actor.Receive(received.CommandCtx, received.Command, received.ReplyChan)
			if err != nil {
				log.Printf("[actor_ref] error handling message, messageType=%s, err=%s\n", received.Command.ProtoReflect().Descriptor().FullName(), err.Error())
			}
			// increase the message counter
			// TODO add this prometheus
			ref.msgCount += 1
		}
	}
}

// selfInit runs the actor initialization.
// An exponential backoff strategy is applied for some number of tries in case of error
// during the actor initialization. When the tries limit is reached then an error message
// is logged and the actor is not started
func (ref *ActorRef) selfInit(ctx context.Context) {
	// get the observability span
	spanCtx, span := getSpanContext(ctx, "ActorRef.Init")
	defer span.End()
	// create the exponential backoff object
	expoBackoff := backoff.WithMaxRetries(backoff.NewExponentialBackOff(), actorInitMaxRetries)
	// start the actor initialization process
	err := backoff.Retry(func() error {
		return ref.actor.Init(spanCtx)
	}, expoBackoff)
	// handle backoff error
	if err != nil {
		log.Printf("[actor_ref] failed to initialize actor, total attempts=%d, err=%s", actorInitMaxRetries, err.Error())
		// attempt to pre-allocated free resource
		// FIXME check whether it is the best place
		ref.Stop()
	}
	// here the initialization has been successful
	log.Printf("[actor_ref] actor %s has been successfully initialized\n", ref.ID)
}
